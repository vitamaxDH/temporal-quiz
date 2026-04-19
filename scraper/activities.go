package scraper

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"log/slog"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
)

// getLogger returns a Temporal activity logger if available, or a fallback slog-based logger.
func getLogger(ctx context.Context) log.Logger {
	if activity.IsActivity(ctx) {
		return activity.GetLogger(ctx)
	}
	return &slogAdapter{slog.Default()}
}

// slogAdapter wraps slog.Logger to satisfy the Temporal log.Logger interface.
type slogAdapter struct{ l *slog.Logger }

func (s *slogAdapter) Debug(msg string, keyvals ...interface{}) { s.l.Debug(msg, keyvals...) }
func (s *slogAdapter) Info(msg string, keyvals ...interface{})  { s.l.Info(msg, keyvals...) }
func (s *slogAdapter) Warn(msg string, keyvals ...interface{})  { s.l.Warn(msg, keyvals...) }
func (s *slogAdapter) Error(msg string, keyvals ...interface{}) { s.l.Error(msg, keyvals...) }

const (
	StartURL  = "https://docs.temporal.io/"
	Domain    = "docs.temporal.io"
	OutputDir = "temporal_docs_html"
	BatchSize = 20
	TaskQueue = "scraper-task-queue"

	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Temporal-AI-Crawler"
)

var binaryExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".pdf", ".zip", ".tar", ".gz"}

type Activities struct {
	OutputDir string
	Domain    string
	Client    *http.Client
}

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

	html, err := doc.Html()
	if err == nil {
		filename := CleanFilename(targetURL)
		fpath := filepath.Join(a.OutputDir, filename)
		_ = os.WriteFile(fpath, []byte(html), 0o644)
	}

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

		resolved.Fragment = ""
		clean := resolved.String()

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

const (
	defaultDocsRepo = "temporalio/documentation"
	docsSubdir      = "docs"
)

var githubAPIBase = "https://api.github.com"

// setGitHubAPIBase overrides the GitHub API base URL (for testing).
func setGitHubAPIBase(base string) { githubAPIBase = base }

// FetchDocsRepo downloads a GitHub repo as a tarball and extracts the docs
// subdirectory to a temp directory. Returns the path to the extracted docs.
func (a *Activities) FetchDocsRepo(ctx context.Context) (string, error) {
	logger := getLogger(ctx)
	repo := defaultDocsRepo
	logger.Info("Fetching docs repo", "repo", repo)

	tarURL := fmt.Sprintf("%s/repos/%s/tarball/main", githubAPIBase, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tarURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch tarball: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("github returned status %d: %s", resp.StatusCode, string(body))
	}

	tmpDir, err := os.MkdirTemp("", "temporal-docs-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	docsDir := ""

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("read tar: %w", err)
		}

		// GitHub tarballs have a top-level dir like "temporalio-documentation-abc1234/"
		// We want files under that prefix + "/docs/"
		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		relPath := parts[1] // strip the top-level dir

		if !strings.HasPrefix(relPath, docsSubdir+"/") && relPath != docsSubdir {
			continue
		}

		target := filepath.Join(tmpDir, relPath)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("mkdir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("mkdir parent: %w", err)
			}
			f, err := os.Create(target)
			if err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("write file: %w", err)
			}
			f.Close()
		}
	}

	docsDir = filepath.Join(tmpDir, docsSubdir)
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("docs directory not found in repo archive")
	}

	logger.Info("Docs repo extracted", "path", docsDir)
	return docsDir, nil
}

// ReadLocalDocs walks a local documentation directory, reads MDX/MD files,
// strips frontmatter, and writes bucketed text files to temporal_docs_txt/.
func (a *Activities) ReadLocalDocs(ctx context.Context, docsDir string) (string, error) {
	logger := getLogger(ctx)
	logger.Info("Reading local docs", "dir", docsDir)

	outputTxtDir := "temporal_docs_txt"
	if err := os.MkdirAll(outputTxtDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	bucketContents := make(map[string][]string)
	count := 0

	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mdx" && ext != ".md" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("Error reading file", "path", path, "error", err)
			return nil
		}

		content := stripFrontmatter(string(data))
		trimmed := strings.TrimSpace(content)
		if trimmed == "" || len(trimmed) < 200 {
			return nil
		}

		// Convert path to a "filename" for bucketing:
		// develop/go/workers/basics.mdx -> develop_go_workers_basics.html
		relPath, _ := filepath.Rel(docsDir, path)
		relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath))
		fakeFilename := strings.ReplaceAll(relPath, string(filepath.Separator), "_") + ".html"

		bucketKey := GetBucketKey(fakeFilename)
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

	return fmt.Sprintf("Success! Read %d files into %d buckets.", count, len(bucketContents)), nil
}

// stripFrontmatter removes YAML frontmatter (between --- markers) from MDX content.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	end := strings.Index(content[3:], "\n---")
	if end == -1 {
		return content
	}
	// Skip past the closing "---" and any trailing newline
	return strings.TrimSpace(content[end+7:])
}

var multipleNewlines = regexp.MustCompile(`\n{3,}`)

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

		doc.Find("script, style, nav, footer, aside").Remove()

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

		markdown, err := htmltomarkdown.ConvertString(contentHTML)
		if err != nil {
			logger.Warn("Error converting to markdown", "path", fpath, "error", err)
			continue
		}
		compacted := strings.TrimSpace(multipleNewlines.ReplaceAllString(markdown, "\n\n"))

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
