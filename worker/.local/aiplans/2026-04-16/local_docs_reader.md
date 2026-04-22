# Read Local Documentation Instead of Scraping

**Goal:** Replace the HTTP scraper with a local file reader that reads MDX/MD files from `~/project/temporal/documentation/docs/` and produces the same bucket text files. The quiz generator doesn't change at all since it only reads from `temporal_docs_txt/`.

**Success criteria:**
- New `ReadLocalDocs` activity reads local MDX files and writes bucket text files
- No HTTP fetching needed for the happy path
- `FetchAndParse` is kept for backward compatibility but the default path uses local docs
- Same output format in `temporal_docs_txt/` so quiz generation works unchanged
- All tests pass

---

## Current Flow vs New Flow

**Current:** ScraperWorkflow -> FetchAndParse (HTTP) -> ProcessHTMLToText (HTML->MD) -> bucket files
**New:** ScraperWorkflow -> ReadLocalDocs (local MDX) -> bucket files

The key insight: local docs are already MDX (markdown), so we skip both HTTP fetch AND HTML-to-markdown conversion. Just read, strip frontmatter, bucket, and write.

---

## Phase 1: Add `ReadLocalDocs` Activity

### 1.1 New activity in `scraper/activities.go`

```go
const LocalDocsDir = "local_docs_dir" // env var or param

// ReadLocalDocs walks a local documentation directory, reads MDX/MD files,
// strips frontmatter, and writes bucketed text files to temporal_docs_txt/.
func (a *Activities) ReadLocalDocs(ctx context.Context, docsDir string) (string, error) {
    logger := activity.GetLogger(ctx)
    logger.Info("Reading local docs", "dir", docsDir)

    outputTxtDir := "temporal_docs_txt"
    if err := os.MkdirAll(outputTxtDir, 0o755); err != nil {
        return "", fmt.Errorf("creating output dir: %w", err)
    }

    bucketContents := make(map[string][]string)
    count := 0

    err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        if info.IsDir() { return nil }

        ext := strings.ToLower(filepath.Ext(path))
        if ext != ".mdx" && ext != ".md" { return nil }

        data, err := os.ReadFile(path)
        if err != nil {
            logger.Warn("Error reading file", "path", path, "error", err)
            return nil
        }

        content := stripFrontmatter(string(data))
        if strings.TrimSpace(content) == "" { return nil }

        // Convert path to a "filename" for bucketing:
        // docs/develop/go/workers/basics.mdx -> develop_go_workers_basics.html
        relPath, _ := filepath.Rel(docsDir, path)
        relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath))
        fakeFilename := strings.ReplaceAll(relPath, string(filepath.Separator), "_") + ".html"

        bucketKey := GetBucketKey(fakeFilename)

        // Use the relative path as the title
        title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

        block := fmt.Sprintf("--- SOURCE: %s (%s) ---\n\n%s\n\n%s\n\n",
            title, fakeFilename, content, strings.Repeat("=", 80))
        bucketContents[bucketKey] = append(bucketContents[bucketKey], block)
        count++

        return nil
    })
    if err != nil {
        return "", fmt.Errorf("walking docs dir: %w", err)
    }

    for _, bucketKey := range SortedBucketKeys() {
        contents, ok := bucketContents[bucketKey]
        if !ok { continue }
        displayName := strings.ReplaceAll(bucketKey, "_", " ")
        outputPath := filepath.Join(outputTxtDir, fmt.Sprintf("temporal_docs_%s.txt", bucketKey))

        header := fmt.Sprintf("# Temporal Documentation: %s\n\nThis file contains context related to: %s\n\n%s\n\n",
            displayName, displayName, strings.Repeat("=", 80))

        data := header + strings.Join(contents, "")
        if err := os.WriteFile(outputPath, []byte(data), 0o644); err != nil {
            logger.Warn("Error writing bucket file", "path", outputPath, "error", err)
        }
    }

    return fmt.Sprintf("Success! Read %d files into %d buckets.", count, len(bucketContents)), nil
}
```

### 1.2 Add `stripFrontmatter` helper

MDX files start with YAML frontmatter between `---` markers. Strip it:

```go
func stripFrontmatter(content string) string {
    if !strings.HasPrefix(content, "---") {
        return content
    }
    end := strings.Index(content[3:], "---")
    if end == -1 {
        return content
    }
    return strings.TrimSpace(content[end+6:])
}
```

---

## Phase 2: Update Workflow to Support Local Mode

### 2.1 Update `ScraperWorkflow` in `scraper/workflow.go`

The workflow currently takes `startURL string`. Change to accept a more flexible param:

```go
type ScraperParams struct {
    StartURL string // HTTP scrape mode (empty = skip)
    LocalDir string // Local docs mode (empty = skip)
}
```

If `LocalDir` is set, call `ReadLocalDocs` directly and skip the HTTP crawl + ProcessHTMLToText.
If `StartURL` is set (or both empty for backward compat), use the existing HTTP path.

```go
func ScraperWorkflow(ctx workflow.Context, params ScraperParams) (int, error) {
    // backward compat: if params is zero-value, default to HTTP mode
    if params.LocalDir == "" && params.StartURL == "" {
        params.StartURL = StartURL
    }

    if params.LocalDir != "" {
        // Local mode: just read files
        processCtx := workflow.WithActivityOptions(ctx, processOpts)
        var result string
        err := workflow.ExecuteActivity(processCtx, "ReadLocalDocs", params.LocalDir).Get(ctx, &result)
        if err != nil { return 0, err }
        logger.Info("Local docs processing complete", "result", result)
        return 0, nil // page count not meaningful for local
    }

    // HTTP mode: existing crawl logic...
}
```

**Wait** - this changes the workflow signature from `(string)` to `(ScraperParams)`. That breaks:
- `DailyPipelineWorkflow` which calls ScraperWorkflow as a child
- `cmd/starter/main.go` which starts ScraperWorkflow
- Workflow tests

Better approach: keep the existing signature and add a **separate workflow** for local mode. Or simpler: just add a new CLI command.

### 2.2 Simpler approach: Add `cmd/localgen/main.go`

Instead of modifying the workflow, add a standalone CLI tool that:
1. Calls `ReadLocalDocs` directly (no Temporal workflow needed for local reading)
2. Then starts `QuizGeneratorWorkflow` via Temporal

```go
// cmd/localgen/main.go
func main() {
    docsDir := flag.String("docs", "", "local docs directory path")
    easy := flag.Int("easy", 3, "easy questions per category")
    med := flag.Int("med", 4, "med questions per category")
    hard := flag.Int("hard", 4, "hard questions per category")
    nightmare := flag.Int("nightmare", 2, "nightmare questions per category")
    flag.Parse()

    if *docsDir == "" {
        log.Fatal("-docs flag is required")
    }

    // Step 1: Read local docs (no Temporal needed)
    a := &scraper.Activities{OutputDir: scraper.OutputDir}
    result, err := a.ReadLocalDocs(context.Background(), *docsDir)
    // ...

    // Step 2: Start quiz generation workflow
    c, err := config.NewTemporalClient()
    // ... same as quizgen/main.go
}
```

This is the cleanest approach because:
- No workflow signature changes
- No breaking existing tests
- Reading local files doesn't need Temporal's retry/timeout features
- Quiz generation still benefits from Temporal's fan-out

### 2.3 Add Makefile target

```makefile
localgen:
    go run cmd/localgen/main.go -docs ~/project/temporal/documentation/docs
```

---

## Phase 3: Update Bucket Mapping for Local Paths

The current `GetBucketKey` works on filenames like `develop_go_workers.html`. When we convert local paths like `develop/go/workers/basics.mdx` to `develop_go_workers_basics.html`, the prefix matching still works:

| Local path | Fake filename | Bucket key |
|------------|---------------|------------|
| `develop/go/workers/basics.mdx` | `develop_go_workers_basics.html` | `Develop` |
| `develop/python/activities/basics.mdx` | `develop_python_activities_basics.html` | `Develop` |
| `encyclopedia/activities/basics.mdx` | `encyclopedia_activities_basics.html` | `Evaluate_and_Concepts` |
| `cloud/namespaces/index.mdx` | `cloud_namespaces_index.html` | `Temporal_Cloud` |
| `cli/cmd/temporal-activity.mdx` | `cli_cmd_temporal-activity.html` | `CLI_and_References` |
| `best-practices/large-scale.mdx` | `best-practices_large-scale.html` | `Self_Hosted_and_Ops` (via "best-practices" prefix) |

Wait, `best-practices` is not in the Buckets map. Let me check... The Self_Hosted_and_Ops bucket has `"best-practices"` prefix. Good.

Some paths that need attention:
- `encyclopedia/**` -> All goes to `Evaluate_and_Concepts` (same as current behavior)
- `production-deployment/**` -> Needs `"production"` prefix. Current bucket has `"production"` in Self_Hosted_and_Ops. Path `production-deployment/foo.mdx` -> `production-deployment_foo.html` -> prefix match on `"production"`. But wait, there's a dash. `strings.HasPrefix("production-deployment_foo", "production")` is `true`. So it works.
- `getting-started.mdx` -> `getting-started.html` -> No bucket match -> "Miscellaneous". That's fine.
- `with-ai.mdx` -> `with-ai.html` -> Matches "with-ai" in AI_and_Cookbook. Good.
- `quickstarts.mdx` -> `quickstarts.html` -> Matches "quickstarts" in AI_and_Cookbook. Good.
- `security.mdx` -> `security.html` -> Matches "security" in Features_Data_and_Security. Good.

No bucket changes needed. The existing prefix matching handles local paths correctly.

---

## Phase 4: Tests

### 4.1 Add `TestReadLocalDocs` in `scraper/activities_test.go`

```go
func TestReadLocalDocs_Success(t *testing.T) {
    // Create temp dir with sample MDX files
    tmpDir := t.TempDir()
    docsDir := filepath.Join(tmpDir, "docs")
    os.MkdirAll(filepath.Join(docsDir, "develop", "go"), 0o755)
    os.MkdirAll(filepath.Join(docsDir, "cloud"), 0o755)

    os.WriteFile(filepath.Join(docsDir, "develop", "go", "basics.mdx"),
        []byte("---\ntitle: Go Basics\n---\n\n# Go SDK Basics\n\nThis is about Go."), 0o644)
    os.WriteFile(filepath.Join(docsDir, "cloud", "namespaces.mdx"),
        []byte("---\ntitle: Cloud NS\n---\n\n# Cloud Namespaces\n\nCloud stuff."), 0o644)

    a := &Activities{OutputDir: filepath.Join(tmpDir, "html")}
    result, err := a.ReadLocalDocs(context.Background(), docsDir)
    require.NoError(t, err)
    assert.Contains(t, result, "2 files")

    // Verify bucket files were created
    devFile := filepath.Join("temporal_docs_txt", "temporal_docs_Develop.txt")
    // ... assert file exists and contains expected content
}
```

### 4.2 Add `TestStripFrontmatter`

```go
func TestStripFrontmatter(t *testing.T) {
    tests := []struct{
        name, input, want string
    }{
        {"with frontmatter", "---\ntitle: Foo\n---\n\nContent here", "Content here"},
        {"no frontmatter", "Just content", "Just content"},
        {"empty frontmatter", "---\n---\n\nContent", "Content"},
    }
    // ...
}
```

---

## Phase 5: Verify

- `go build ./...`
- `go test ./...`
- Run: `go run cmd/localgen/main.go -docs ~/project/temporal/documentation/docs`
- Verify `temporal_docs_txt/` is populated with expected buckets

---

## Summary of files changed

| File | Changes |
|------|---------|
| `scraper/activities.go` | Add `ReadLocalDocs` activity, `stripFrontmatter` helper |
| `cmd/localgen/main.go` | New CLI for local docs -> quiz generation |
| `scraper/activities_test.go` | Add TestReadLocalDocs, TestStripFrontmatter |
| `Makefile` | Add `localgen` target |

**Files NOT changed:**
- `scraper/workflow.go` - kept as-is (HTTP scraping still works if needed)
- `scraper/buckets.go` - no changes needed, prefix matching handles local paths
- `quiz/*` - completely unchanged, reads from the same bucket files
