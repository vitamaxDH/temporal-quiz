# Go Rewrite Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite the Python temporal-quiz web scraper as idiomatic Go, connecting to Temporal Cloud namespace `daehan-temporal-quiz.temporal-dev.tmprl-test.cloud:7233`.

**Architecture:** Clean separation into `cmd/` (entry points), `scraper/` (workflow + activities + domain logic), and `config/` (Temporal client factory). Activities do HTTP fetch + HTML parsing and HTML-to-Markdown bucketing. Workflow orchestrates BFS crawl with batched fan-out/fan-in, then sequential text processing.

**Tech Stack:** Go 1.24, Temporal Go SDK (`go.temporal.io/sdk`), `goquery` (HTML parsing), `html-to-markdown` (markdown conversion), `godotenv` (.env loading), `testify` (testing)

---

## Project Structure

```
temporal-quiz/
├── cmd/
│   ├── worker/main.go       # Worker process
│   └── starter/main.go      # Workflow trigger
├── scraper/
│   ├── workflow.go           # ScraperWorkflow
│   ├── workflow_test.go      # Workflow tests
│   ├── activities.go         # Activities struct + fetch/process
│   ├── activities_test.go    # Activity tests
│   └── buckets.go            # Bucket classification logic
│   └── buckets_test.go       # Bucket tests
├── config/
│   └── config.go             # Temporal client factory
├── go.mod
├── go.sum
├── Makefile
├── env.example
└── .gitignore
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Modify: `.gitignore`
- Create: directories `cmd/worker/`, `cmd/starter/`, `scraper/`, `config/`

**Step 1: Initialize Go module**

```bash
cd ~/project/temporal/temporal-quiz
go mod init temporal-quiz
```

**Step 2: Create directory structure**

```bash
mkdir -p cmd/worker cmd/starter scraper config
```

**Step 3: Update .gitignore for Go**

Append to existing `.gitignore`:

```gitignore
# Go
bin/
*.exe
```

**Step 4: Install dependencies**

```bash
go get go.temporal.io/sdk@latest
go get github.com/PuerkitoBio/goquery@latest
go get github.com/JohannesKaufmann/html-to-markdown/v2@latest
go get github.com/joho/godotenv@latest
go get github.com/stretchr/testify@latest
```

**Step 5: Commit**

```bash
git add go.mod go.sum .gitignore
git commit -m "chore: initialize Go module with dependencies"
```

---

### Task 2: Config Package (Temporal Client Factory)

**Files:**
- Create: `config/config.go`

Mirrors the Python `config.py` logic. Reads env vars, supports local (default) and Cloud (API key or mTLS).

**Step 1: Write config.go**

```go
// config/config.go
package config

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
)

const (
	LocalAddress    = "localhost:7233"
	DefaultNamespace = "default"
)

func init() {
	// Best-effort .env loading
	_ = godotenv.Load()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// NewTemporalClient creates a Temporal client based on environment variables.
//
// Local development (default): no env vars needed, connects to localhost:7233.
//
// Temporal Cloud: set TEMPORAL_ADDRESS and TEMPORAL_NAMESPACE,
// plus TEMPORAL_API_KEY or TEMPORAL_TLS_CERT_PATH + TEMPORAL_TLS_KEY_PATH.
func NewTemporalClient() (client.Client, error) {
	address := getEnv("TEMPORAL_ADDRESS", LocalAddress)
	namespace := getEnv("TEMPORAL_NAMESPACE", DefaultNamespace)
	apiKey := os.Getenv("TEMPORAL_API_KEY")
	certPath := os.Getenv("TEMPORAL_TLS_CERT_PATH")
	keyPath := os.Getenv("TEMPORAL_TLS_KEY_PATH")

	isCloud := address != LocalAddress

	opts := client.Options{
		HostPort:  address,
		Namespace: namespace,
	}

	if isCloud && apiKey != "" {
		opts.Credentials = client.NewAPIKeyStaticCredentials(apiKey)
		opts.ConnectionOptions = client.ConnectionOptions{
			TLS: &tls.Config{},
		}
	} else if isCloud && certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("loading TLS cert/key: %w", err)
		}
		opts.ConnectionOptions = client.ConnectionOptions{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}
	}

	return client.Dial(opts)
}
```

**Step 2: Verify it compiles**

```bash
cd ~/project/temporal/temporal-quiz && go build ./config/...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add config/
git commit -m "feat: add Temporal client factory with Cloud support"
```

---

### Task 3: Bucket Classification Logic + Tests

**Files:**
- Create: `scraper/buckets.go`
- Test: `scraper/buckets_test.go`

This is the pure domain logic, no Temporal dependency. Good TDD target.

**Step 1: Write the failing test**

```go
// scraper/buckets_test.go
package scraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBucketKey(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		// Language-specific SDKs (highest priority)
		{"develop_go_workers.html", "Develop_Go"},
		{"develop_java_activities.html", "Develop_Java"},
		{"develop_python_workflows.html", "Develop_Python"},
		{"develop_typescript_signals.html", "Develop_TypeScript"},

		// Other SDKs (before Develop_General)
		{"develop_dotnet_overview.html", "Develop_Other_SDKs"},
		{"develop_php_activities.html", "Develop_Other_SDKs"},
		{"develop_ruby_workers.html", "Develop_Other_SDKs"},

		// General develop (catch-all for develop_*)
		{"develop_index.html", "Develop_General"},

		// Features
		{"workflows_overview.html", "Features_Workflows"},
		{"workflow-timeouts.html", "Features_Workflows"},
		{"child-workflows.html", "Features_Workflows"},
		{"activities_overview.html", "Features_Activities"},
		{"activity-heartbeats.html", "Features_Activities"},
		{"workers_overview.html", "Features_Workers_and_Routing"},
		{"task-queue-routing.html", "Features_Workers_and_Routing"},
		{"namespaces.html", "Features_Workers_and_Routing"},
		{"dataconversion.html", "Features_Data_and_Security"},
		{"security.html", "Features_Data_and_Security"},
		{"nexus.html", "Features_Nexus"},
		{"sending-messages.html", "Features_Messaging_and_Visibility"},
		{"visibility.html", "Features_Messaging_and_Visibility"},
		{"schedule.html", "Features_Other"},
		{"retry-policies.html", "Features_Other"},

		// Ops, Cloud, AI
		{"self-hosted-guide.html", "Self_Hosted_and_Ops"},
		{"cloud-namespaces.html", "Temporal_Cloud"},
		{"ai-cookbook-intro.html", "AI_and_Cookbook"},
		{"cli-reference.html", "CLI_and_References"},
		{"tags-overview.html", "Tags"},

		// Evaluate and Concepts
		{"evaluate.html", "Evaluate_and_Concepts"},
		{"encyclopedia.html", "Evaluate_and_Concepts"},
		{"glossary.html", "Evaluate_and_Concepts"},

		// Fallback
		{"random-page.html", "Miscellaneous"},
		{"index.html", "Evaluate_and_Concepts"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := GetBucketKey(tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./scraper/ -run TestGetBucketKey -v
```

Expected: FAIL (function not defined).

**Step 3: Write implementation**

```go
// scraper/buckets.go
package scraper

import "strings"

// Buckets maps bucket names to their filename prefix patterns.
var Buckets = map[string][]string{
	"Evaluate_and_Concepts":        {"evaluate", "encyclopedia", "glossary", "index", "temporal.html", "temporal-service"},
	"Develop_Other_SDKs":           {"develop_dotnet", "develop_php", "develop_ruby"},
	"Develop_General":              {"develop"},
	"Features_Workflows":           {"workflows", "workflow-", "child-workflows", "sticky-execution", "cron-job", "parent-close-policy", "dynamic-handler"},
	"Features_Activities":          {"activities", "activity-", "local-activity"},
	"Features_Workers_and_Routing": {"workers", "worker-", "task-queue", "task-routing", "namespaces", "global-namespace"},
	"Features_Data_and_Security":   {"dataconversion", "payload", "codec", "failure-converter", "remote-data", "security", "key-management", "auth"},
	"Features_Nexus":               {"nexus"},
	"Features_Messaging_and_Visibility": {"sending-messages", "handling-messages", "queries", "signals", "visibility", "search-attribute", "list-filter", "dual-visibility"},
	"Features_Other":               {"schedule", "retry-policies", "patching"},
	"Self_Hosted_and_Ops":          {"self-hosted", "production", "ops", "troubleshooting", "best-practices"},
	"Temporal_Cloud":               {"cloud"},
	"AI_and_Cookbook":               {"ai-cookbook", "with-ai", "quickstarts"},
	"CLI_and_References":           {"cli", "tctl", "references", "api", "web-ui"},
	"Tags":                         {"tags"},
}

// bucketOrder defines the iteration order for Buckets (maps are unordered in Go).
// This must match the Python BUCKETS dict ordering.
var bucketOrder = []string{
	"Evaluate_and_Concepts",
	"Develop_Other_SDKs",
	"Develop_General",
	"Features_Workflows",
	"Features_Activities",
	"Features_Workers_and_Routing",
	"Features_Data_and_Security",
	"Features_Nexus",
	"Features_Messaging_and_Visibility",
	"Features_Other",
	"Self_Hosted_and_Ops",
	"Temporal_Cloud",
	"AI_and_Cookbook",
	"CLI_and_References",
	"Tags",
}

// GetBucketKey classifies an HTML filename into a documentation bucket.
func GetBucketKey(filename string) string {
	name := strings.ToLower(strings.TrimSuffix(filename, ".html"))

	// Language-specific SDKs take highest priority
	switch {
	case strings.HasPrefix(name, "develop_go"):
		return "Develop_Go"
	case strings.HasPrefix(name, "develop_java"):
		return "Develop_Java"
	case strings.HasPrefix(name, "develop_python"):
		return "Develop_Python"
	case strings.HasPrefix(name, "develop_typescript"):
		return "Develop_TypeScript"
	}

	// Check remaining buckets in order
	for _, bucketKey := range bucketOrder {
		prefixes := Buckets[bucketKey]

		// Skip Develop_General for other SDK prefixes
		if bucketKey == "Develop_General" {
			if strings.HasPrefix(name, "develop_dotnet") ||
				strings.HasPrefix(name, "develop_php") ||
				strings.HasPrefix(name, "develop_ruby") {
				continue
			}
		}

		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				return bucketKey
			}
		}
	}

	return "Miscellaneous"
}

// SortedBucketKeys returns all possible bucket keys in sorted order for output.
func SortedBucketKeys() []string {
	all := []string{
		"AI_and_Cookbook",
		"CLI_and_References",
		"Develop_General",
		"Develop_Go",
		"Develop_Java",
		"Develop_Other_SDKs",
		"Develop_Python",
		"Develop_TypeScript",
		"Evaluate_and_Concepts",
		"Features_Activities",
		"Features_Data_and_Security",
		"Features_Messaging_and_Visibility",
		"Features_Nexus",
		"Features_Other",
		"Features_Workers_and_Routing",
		"Features_Workflows",
		"Miscellaneous",
		"Self_Hosted_and_Ops",
		"Tags",
		"Temporal_Cloud",
	}
	return all
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./scraper/ -run TestGetBucketKey -v
```

Expected: PASS (all cases).

**Step 5: Commit**

```bash
git add scraper/buckets.go scraper/buckets_test.go
git commit -m "feat: add bucket classification logic with tests"
```

---

### Task 4: Activities - Fetch and Parse

**Files:**
- Create: `scraper/activities.go`
- Test: `scraper/activities_test.go`

**Step 1: Write the failing test for fetch_and_parse**

```go
// scraper/activities_test.go
package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type ActivitiesTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
	act *Activities
}

func (s *ActivitiesTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
	s.act = &Activities{
		OutputDir: s.T().TempDir(),
		Domain:    "127.0.0.1",
		Client:    &http.Client{},
	}
	s.env.RegisterActivity(s.act.FetchAndParse)
	s.env.RegisterActivity(s.act.ProcessHTMLToText)
}

func TestActivitiesTestSuite(t *testing.T) {
	suite.Run(t, new(ActivitiesTestSuite))
}

func (s *ActivitiesTestSuite) TestFetchAndParse_Success() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
			<a href="https://external.com/nope">External</a>
		</body></html>`)
	}))
	s.T().Cleanup(server.Close)

	s.act.Domain = "127.0.0.1"
	val, err := s.env.ExecuteActivity(s.act.FetchAndParse, server.URL)
	require.NoError(s.T(), err)

	var urls []string
	require.NoError(s.T(), val.Get(&urls))

	// Should find /page1 and /page2 (same domain), not external.com
	assert.Len(s.T(), urls, 2)

	// Should have saved HTML to disk
	files, _ := filepath.Glob(filepath.Join(s.act.OutputDir, "*.html"))
	assert.Len(s.T(), files, 1)
}

func (s *ActivitiesTestSuite) TestFetchAndParse_NonHTML() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"not":"html"}`)
	}))
	s.T().Cleanup(server.Close)

	val, err := s.env.ExecuteActivity(s.act.FetchAndParse, server.URL)
	require.NoError(s.T(), err)

	var urls []string
	require.NoError(s.T(), val.Get(&urls))
	assert.Empty(s.T(), urls)
}

func (s *ActivitiesTestSuite) TestFetchAndParse_404() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	s.T().Cleanup(server.Close)

	val, err := s.env.ExecuteActivity(s.act.FetchAndParse, server.URL)
	require.NoError(s.T(), err)

	var urls []string
	require.NoError(s.T(), val.Get(&urls))
	assert.Empty(s.T(), urls)
}

func (s *ActivitiesTestSuite) TestFetchAndParse_FiltersExtensions() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
			<a href="/good-page">Good</a>
			<a href="/image.png">PNG</a>
			<a href="/doc.pdf">PDF</a>
			<a href="/archive.zip">ZIP</a>
			<a href="/page#section">With Fragment</a>
		</body></html>`)
	}))
	s.T().Cleanup(server.Close)

	s.act.Domain = "127.0.0.1"
	val, err := s.env.ExecuteActivity(s.act.FetchAndParse, server.URL)
	require.NoError(s.T(), err)

	var urls []string
	require.NoError(s.T(), val.Get(&urls))

	// /good-page and /page (fragment stripped), not .png/.pdf/.zip
	assert.Len(s.T(), urls, 2)
	for _, u := range urls {
		assert.NotContains(s.T(), u, "#")
		assert.NotContains(s.T(), u, ".png")
		assert.NotContains(s.T(), u, ".pdf")
		assert.NotContains(s.T(), u, ".zip")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./scraper/ -run TestActivitiesTestSuite -v
```

Expected: FAIL (Activities struct not defined).

**Step 3: Write implementation**

```go
// scraper/activities.go
package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"go.temporal.io/sdk/activity"
)

const (
	StartURL  = "https://docs.temporal.io/"
	Domain    = "docs.temporal.io"
	OutputDir = "temporal_docs_html"
	BatchSize = 20
	TaskQueue = "scraper-task-queue"

	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Temporal-AI-Crawler"
)

var binaryExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".pdf", ".zip", ".tar", ".gz"}

// Activities holds dependencies for Temporal activities.
type Activities struct {
	OutputDir string
	Domain    string
	Client    *http.Client
}

// CleanFilename converts a URL to a safe filename.
func CleanFilename(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "unknown.html"
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return "index.html"
	}
	return strings.ReplaceAll(path, "/", "_") + ".html"
}

// FetchAndParse fetches a URL, saves the HTML, and returns discovered same-domain links.
func (a *Activities) FetchAndParse(ctx context.Context, targetURL string) ([]string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching", "url", targetURL)

	if err := os.MkdirAll(a.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		logger.Warn("Failed to create request", "url", targetURL, "error", err)
		return nil, nil
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := a.Client.Do(req)
	if err != nil {
		logger.Warn("Failed to fetch", "url", targetURL, "error", err)
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Non-200 status", "url", targetURL, "status", resp.StatusCode)
		return nil, nil
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		logger.Info("Skipping non-HTML", "url", targetURL, "contentType", contentType)
		return nil, nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logger.Warn("Failed to parse HTML", "url", targetURL, "error", err)
		return nil, nil
	}

	// Save raw HTML to disk
	html, err := doc.Html()
	if err == nil {
		filename := CleanFilename(targetURL)
		fpath := filepath.Join(a.OutputDir, filename)
		_ = os.WriteFile(fpath, []byte(html), 0o644)
	}

	// Extract and filter links
	baseURL, _ := url.Parse(targetURL)
	var newURLs []string

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		resolved, err := baseURL.Parse(href)
		if err != nil {
			return
		}

		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return
		}
		if resolved.Host != a.Domain {
			return
		}

		// Strip fragment
		resolved.Fragment = ""
		clean := resolved.String()

		// Exclude binary files
		lower := strings.ToLower(clean)
		for _, ext := range binaryExtensions {
			if strings.HasSuffix(lower, ext) {
				return
			}
		}

		newURLs = append(newURLs, clean)
	})

	return newURLs, nil
}

var multipleNewlines = regexp.MustCompile(`\n{3,}`)

// ProcessHTMLToText reads all saved HTML files, converts to Markdown, and groups into buckets.
func (a *Activities) ProcessHTMLToText(ctx context.Context) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting text processing and bucketing")

	outputTxtDir := strings.TrimSuffix(a.OutputDir, "_html") + "_txt"

	entries, err := filepath.Glob(filepath.Join(a.OutputDir, "*.html"))
	if err != nil {
		return "", fmt.Errorf("listing HTML files: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Sprintf("No HTML files found in '%s'.", a.OutputDir), nil
	}

	if err := os.MkdirAll(outputTxtDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output txt dir: %w", err)
	}

	bucketContents := make(map[string][]string)

	for _, fpath := range entries {
		htmlBytes, err := os.ReadFile(fpath)
		if err != nil {
			logger.Warn("Error reading file", "path", fpath, "error", err)
			continue
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(htmlBytes)))
		if err != nil {
			logger.Warn("Error parsing HTML", "path", fpath, "error", err)
			continue
		}

		// Remove unwanted elements
		doc.Find("script, style, nav, footer, aside").Remove()

		// Find main content
		content := doc.Find("main").First()
		if content.Length() == 0 {
			content = doc.Find("article").First()
		}
		if content.Length() == 0 {
			content = doc.Find("body").First()
		}

		contentHTML, err := content.Html()
		if err != nil {
			continue
		}

		// Convert to Markdown
		markdown, err := htmltomarkdown.ConvertString(contentHTML)
		if err != nil {
			logger.Warn("Error converting to markdown", "path", fpath, "error", err)
			continue
		}
		compacted := strings.TrimSpace(multipleNewlines.ReplaceAllString(markdown, "\n\n"))

		// Get title
		title := "Unknown Title"
		if titleEl := doc.Find("title").First(); titleEl.Length() > 0 {
			title = strings.TrimSpace(titleEl.Text())
		}

		filename := filepath.Base(fpath)
		bucketKey := GetBucketKey(filename)

		block := fmt.Sprintf("--- SOURCE: %s (%s) ---\n\n%s\n\n%s\n\n",
			title, filename, compacted, strings.Repeat("=", 80))
		bucketContents[bucketKey] = append(bucketContents[bucketKey], block)
	}

	// Write bucket files (sorted)
	for _, bucketKey := range SortedBucketKeys() {
		contents, ok := bucketContents[bucketKey]
		if !ok {
			continue
		}
		displayName := strings.ReplaceAll(bucketKey, "_", " ")
		outputPath := filepath.Join(outputTxtDir, fmt.Sprintf("temporal_docs_%s.txt", bucketKey))

		header := fmt.Sprintf("# Temporal Documentation: %s\n\nThis file contains context related to: %s\n\n%s\n\n",
			displayName, displayName, strings.Repeat("=", 80))

		data := header + strings.Join(contents, "")
		if err := os.WriteFile(outputPath, []byte(data), 0o644); err != nil {
			logger.Warn("Error writing bucket file", "path", outputPath, "error", err)
		}
	}

	return fmt.Sprintf("Success! Grouped %d HTML files into %d Markdown buckets.",
		len(entries), len(bucketContents)), nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./scraper/ -run TestActivitiesTestSuite -v
```

Expected: PASS (all 4 subtests).

**Step 5: Commit**

```bash
git add scraper/activities.go scraper/activities_test.go
git commit -m "feat: add fetch and process activities with tests"
```

---

### Task 5: Activity - ProcessHTMLToText Tests

**Files:**
- Modify: `scraper/activities_test.go`

**Step 1: Add process test to existing suite**

Append to `scraper/activities_test.go`:

```go
func (s *ActivitiesTestSuite) TestProcessHTMLToText_Success() {
	// Write sample HTML files to the activity's output dir
	html1 := `<html><head><title>Develop Go Workers</title></head>
		<body><main><h1>Go Workers</h1><p>Content about workers.</p></main>
		<nav>nav stuff</nav></body></html>`
	html2 := `<html><head><title>Cloud Overview</title></head>
		<body><main><h1>Cloud</h1><p>Cloud content.</p></main></body></html>`

	require.NoError(s.T(), os.WriteFile(
		filepath.Join(s.act.OutputDir, "develop_go_workers.html"),
		[]byte(html1), 0o644))
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(s.act.OutputDir, "cloud-overview.html"),
		[]byte(html2), 0o644))

	val, err := s.env.ExecuteActivity(s.act.ProcessHTMLToText)
	require.NoError(s.T(), err)

	var result string
	require.NoError(s.T(), val.Get(&result))
	assert.Contains(s.T(), result, "2 HTML files")
	assert.Contains(s.T(), result, "2 Markdown buckets")

	// Check output files were created
	txtDir := strings.TrimSuffix(s.act.OutputDir, "_html") + "_txt"
	files, _ := filepath.Glob(filepath.Join(txtDir, "*.txt"))
	assert.Len(s.T(), files, 2)
}

func (s *ActivitiesTestSuite) TestProcessHTMLToText_NoFiles() {
	val, err := s.env.ExecuteActivity(s.act.ProcessHTMLToText)
	require.NoError(s.T(), err)

	var result string
	require.NoError(s.T(), val.Get(&result))
	assert.Contains(s.T(), result, "No HTML files")
}
```

Note: Add `"os"` and `"strings"` to imports if not already present.

**Step 2: Run tests**

```bash
go test ./scraper/ -run TestActivitiesTestSuite -v
```

Expected: PASS (all 6 subtests).

**Step 3: Commit**

```bash
git add scraper/activities_test.go
git commit -m "test: add process_html_to_text activity tests"
```

---

### Task 6: Scraper Workflow + Tests

**Files:**
- Create: `scraper/workflow.go`
- Test: `scraper/workflow_test.go`

**Step 1: Write the failing workflow test**

```go
// scraper/workflow_test.go
package scraper

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type WorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *WorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *WorkflowTestSuite) TearDownTest() {
	s.env.AssertExpectations(s.T())
}

func TestWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowTestSuite))
}

func (s *WorkflowTestSuite) TestScraperWorkflow_SinglePage() {
	// First fetch returns no new links (single page, no outbound)
	s.env.OnActivity((*Activities).FetchAndParse, mock.Anything, "https://docs.temporal.io/").
		Return([]string{}, nil).Once()

	// Text processing runs after fetch phase
	s.env.OnActivity((*Activities).ProcessHTMLToText, mock.Anything).
		Return("Success! Grouped 1 HTML files into 1 Markdown buckets.", nil).Once()

	s.env.ExecuteWorkflow(ScraperWorkflow, "https://docs.temporal.io/")
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result int
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	require.Equal(s.T(), 1, result)
}

func (s *WorkflowTestSuite) TestScraperWorkflow_WithDiscoveredLinks() {
	// First fetch discovers 2 new pages
	s.env.OnActivity((*Activities).FetchAndParse, mock.Anything, "https://docs.temporal.io/").
		Return([]string{
			"https://docs.temporal.io/workflows",
			"https://docs.temporal.io/activities",
		}, nil).Once()

	// Second batch fetches the 2 discovered pages (no further links)
	s.env.OnActivity((*Activities).FetchAndParse, mock.Anything, "https://docs.temporal.io/workflows").
		Return([]string{}, nil).Once()
	s.env.OnActivity((*Activities).FetchAndParse, mock.Anything, "https://docs.temporal.io/activities").
		Return([]string{}, nil).Once()

	s.env.OnActivity((*Activities).ProcessHTMLToText, mock.Anything).
		Return("Success!", nil).Once()

	s.env.ExecuteWorkflow(ScraperWorkflow, "https://docs.temporal.io/")
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result int
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	require.Equal(s.T(), 3, result) // start + 2 discovered
}

func (s *WorkflowTestSuite) TestScraperWorkflow_DeduplicatesURLs() {
	// First fetch returns duplicate URLs
	s.env.OnActivity((*Activities).FetchAndParse, mock.Anything, "https://docs.temporal.io/").
		Return([]string{
			"https://docs.temporal.io/page1",
			"https://docs.temporal.io/page1", // duplicate
			"https://docs.temporal.io/page1", // duplicate
		}, nil).Once()

	// Only 1 unique page fetched (not 3)
	s.env.OnActivity((*Activities).FetchAndParse, mock.Anything, "https://docs.temporal.io/page1").
		Return([]string{}, nil).Once()

	s.env.OnActivity((*Activities).ProcessHTMLToText, mock.Anything).
		Return("Success!", nil).Once()

	s.env.ExecuteWorkflow(ScraperWorkflow, "https://docs.temporal.io/")
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result int
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	require.Equal(s.T(), 2, result) // start + 1 unique
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./scraper/ -run TestWorkflowTestSuite -v
```

Expected: FAIL (ScraperWorkflow not defined).

**Step 3: Write workflow implementation**

```go
// scraper/workflow.go
package scraper

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ScraperWorkflow crawls a website starting from startURL using BFS,
// then processes all downloaded HTML into bucketed Markdown.
func ScraperWorkflow(ctx workflow.Context, startURL string) (int, error) {
	logger := workflow.GetLogger(ctx)

	retryPolicy := &temporal.RetryPolicy{
		MaximumAttempts: 3,
		InitialInterval: 2 * time.Second,
	}

	fetchOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy:         retryPolicy,
	}

	processOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         retryPolicy,
	}

	visited := map[string]bool{startURL: true}
	queue := []string{startURL}

	for len(queue) > 0 {
		// Take a batch
		end := BatchSize
		if end > len(queue) {
			end = len(queue)
		}
		batch := queue[:end]
		queue = queue[end:]

		logger.Info("Processing batch", "batchSize", len(batch), "remaining", len(queue))

		// Fan-out: execute fetches in parallel
		futures := make([]workflow.Future, len(batch))
		fetchCtx := workflow.WithActivityOptions(ctx, fetchOpts)
		for i, u := range batch {
			futures[i] = workflow.ExecuteActivity(fetchCtx, (*Activities).FetchAndParse, u)
		}

		// Fan-in: collect results
		for _, f := range futures {
			var newLinks []string
			if err := f.Get(ctx, &newLinks); err != nil {
				logger.Warn("Fetch activity failed", "error", err)
				continue
			}
			for _, link := range newLinks {
				if !visited[link] {
					visited[link] = true
					queue = append(queue, link)
				}
			}
		}
	}

	logger.Info("HTML scraping complete", "totalPages", len(visited))
	logger.Info("Executing text processing activity")

	processCtx := workflow.WithActivityOptions(ctx, processOpts)
	var processResult string
	if err := workflow.ExecuteActivity(processCtx, (*Activities).ProcessHTMLToText).Get(ctx, &processResult); err != nil {
		return 0, err
	}

	logger.Info("Processing complete", "result", processResult)
	return len(visited), nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./scraper/ -run TestWorkflowTestSuite -v
```

Expected: PASS (all 3 subtests).

**Step 5: Commit**

```bash
git add scraper/workflow.go scraper/workflow_test.go
git commit -m "feat: add ScraperWorkflow with BFS crawl and tests"
```

---

### Task 7: Worker Entry Point

**Files:**
- Create: `cmd/worker/main.go`

**Step 1: Write worker main**

```go
// cmd/worker/main.go
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"temporal-quiz/config"
	"temporal-quiz/scraper"

	"go.temporal.io/sdk/worker"
)

func main() {
	fmt.Println("Connecting to Temporal server...")
	c, err := config.NewTemporalClient()
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()
	fmt.Printf("Connected to Temporal!\n")

	activities := &scraper.Activities{
		OutputDir: scraper.OutputDir,
		Domain:    scraper.Domain,
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	w := worker.New(c, scraper.TaskQueue, worker.Options{})
	w.RegisterWorkflow(scraper.ScraperWorkflow)
	w.RegisterActivity(activities)

	fmt.Printf("Starting worker on task queue '%s'...\n", scraper.TaskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}
```

**Step 2: Verify it compiles**

```bash
go build ./cmd/worker/
```

Expected: no errors.

**Step 3: Commit**

```bash
git add cmd/worker/
git commit -m "feat: add worker entry point"
```

---

### Task 8: Starter (Workflow Trigger) Entry Point

**Files:**
- Create: `cmd/starter/main.go`

**Step 1: Write starter main**

```go
// cmd/starter/main.go
package main

import (
	"context"
	"fmt"
	"log"

	"temporal-quiz/config"
	"temporal-quiz/scraper"

	"go.temporal.io/sdk/client"
)

func main() {
	fmt.Println("Connecting to Temporal server...")
	c, err := config.NewTemporalClient()
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	fmt.Printf("Triggering ScraperWorkflow for %s...\n", scraper.StartURL)

	we, err := c.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			ID:        "temporal-docs-scraper-workflow",
			TaskQueue: scraper.TaskQueue,
		},
		scraper.ScraperWorkflow,
		scraper.StartURL,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("Workflow started: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result int
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nWorkflow completed! Total pages crawled: %d\n", result)
	fmt.Printf("Downloaded to: ./%s/\n", scraper.OutputDir)
}
```

**Step 2: Verify it compiles**

```bash
go build ./cmd/starter/
```

Expected: no errors.

**Step 3: Commit**

```bash
git add cmd/starter/
git commit -m "feat: add workflow starter entry point"
```

---

### Task 9: Makefile Update

**Files:**
- Modify: `Makefile`

**Step 1: Rewrite Makefile for Go**

```makefile
.PHONY: build test lint worker scrape start-server stop-server clean wipe-all

help:
	@echo "Temporal Docs Scraper (Go) - Available Commands:"
	@echo "-------------------------------------------------"
	@echo "  make build        : Build worker and starter binaries"
	@echo "  make test         : Run all tests"
	@echo "  make lint         : Run go vet"
	@echo "  make worker       : Build and run the Temporal Worker"
	@echo "  make scrape       : Build and run the Scraper Workflow trigger"
	@echo "  make start-server : Start local Temporal Docker Cluster"
	@echo "  make stop-server  : Stop local Temporal Docker Cluster"
	@echo "  make clean        : Remove downloaded HTML files"
	@echo "  make wipe-all     : Remove ALL downloaded files and binaries"

build:
	@echo "Building binaries..."
	@go build -o bin/worker ./cmd/worker
	@go build -o bin/starter ./cmd/starter
	@echo "Built: bin/worker, bin/starter"

test:
	@go test ./... -v

lint:
	@go vet ./...

worker: build
	@echo "Starting Temporal Worker... (Press Ctrl+C to stop)"
	@./bin/worker

scrape: build
	@echo "Triggering Scraper Workflow..."
	@./bin/starter

start-server:
	@echo "Starting Temporal cluster via Docker Compose..."
	@cd temporal-docker-compose && docker-compose up -d
	@echo "Temporal Web UI available at: http://localhost:8081"

stop-server:
	@echo "Stopping Temporal cluster..."
	@cd temporal-docker-compose && docker-compose down

clean:
	@echo "Cleaning up raw HTML files..."
	@rm -rf temporal_docs_html

wipe-all: clean
	@echo "Wiping all processed txt files and binaries..."
	@rm -rf temporal_docs_txt bin/
```

**Step 2: Run tests via Makefile**

```bash
make test
```

Expected: All tests pass.

**Step 3: Run build via Makefile**

```bash
make build
```

Expected: `bin/worker` and `bin/starter` created.

**Step 4: Commit**

```bash
git add Makefile
git commit -m "chore: rewrite Makefile for Go build/test/run targets"
```

---

### Task 10: Final Cleanup and Verification

**Step 1: Run full test suite**

```bash
make test
```

Expected: All tests pass.

**Step 2: Run lint**

```bash
make lint
```

Expected: No issues.

**Step 3: Build both binaries**

```bash
make build
```

Expected: Compiles clean, creates `bin/worker` and `bin/starter`.

**Step 4: Remove old Python files**

```bash
rm -f run_worker.py run_scraper.py workflows.py config.py test_parse.py test_parse_ssl.py env.example requirements.txt
rm -rf .venv __pycache__
```

**Step 5: Final commit**

```bash
git add -A
git commit -m "chore: remove Python source files after Go rewrite"
```

**Step 6: Verify worker starts (local, will fail to connect but should compile/run)**

```bash
./bin/worker
```

Expected: "Connecting to Temporal server..." then connection error (no local Temporal running). This confirms the binary runs.

---

## Summary

| Task | What | Tests |
|------|------|-------|
| 1 | Project scaffolding | - |
| 2 | Config package (client factory) | Compiles |
| 3 | Bucket classification | 30+ table-driven cases |
| 4 | FetchAndParse activity | 4 test cases (success, non-HTML, 404, extension filtering) |
| 5 | ProcessHTMLToText activity | 2 test cases (success, no files) |
| 6 | ScraperWorkflow | 3 test cases (single page, discovery, dedup) |
| 7 | Worker entry point | Compiles |
| 8 | Starter entry point | Compiles |
| 9 | Makefile | build + test targets |
| 10 | Cleanup + verification | Full suite green |

**Total:** ~10 commits, ~500 LOC Go (vs 275 LOC Python)
