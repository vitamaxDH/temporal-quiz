package scraper

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// buildTestTarball creates an in-memory gzipped tarball from a map of path->content.
func buildTestTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

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
	s.env.RegisterActivity(s.act.FetchDocsRepo)
	s.env.RegisterActivity(s.act.ReadLocalDocs)
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

	// Domain must include the port to match the test server's host.
	parsed, _ := url.Parse(server.URL)
	s.act.Domain = parsed.Host
	val, err := s.env.ExecuteActivity(s.act.FetchAndParse, server.URL)
	require.NoError(s.T(), err)

	var urls []string
	require.NoError(s.T(), val.Get(&urls))
	assert.Len(s.T(), urls, 2)

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

	parsed, _ := url.Parse(server.URL)
	s.act.Domain = parsed.Host
	val, err := s.env.ExecuteActivity(s.act.FetchAndParse, server.URL)
	require.NoError(s.T(), err)

	var urls []string
	require.NoError(s.T(), val.Get(&urls))
	assert.Len(s.T(), urls, 2)
	for _, u := range urls {
		assert.NotContains(s.T(), u, "#")
		assert.NotContains(s.T(), u, ".png")
		assert.NotContains(s.T(), u, ".pdf")
		assert.NotContains(s.T(), u, ".zip")
	}
}

func (s *ActivitiesTestSuite) TestProcessHTMLToText_Success() {
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

func (s *ActivitiesTestSuite) TestFetchDocsRepo_Success() {
	// Build a minimal tarball with a docs/ subdirectory.
	tarballData := buildTestTarball(s.T(), map[string]string{
		"temporalio-documentation-abc1234/docs/develop/go/basics.mdx": "---\ntitle: Go Basics\n---\n\n# Go SDK Basics\n\nThis is about Go.",
		"temporalio-documentation-abc1234/docs/cloud/namespaces.mdx":  "---\ntitle: Cloud NS\n---\n\n# Cloud Namespaces\n\nCloud stuff.",
		"temporalio-documentation-abc1234/README.md":                  "# Documentation repo",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(s.T(), r.URL.Path, "/repos/")
		assert.Contains(s.T(), r.URL.Path, "/tarball/")
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(tarballData)
	}))
	s.T().Cleanup(server.Close)

	// Override the GitHub API base for testing.
	origBase := githubAPIBase
	setGitHubAPIBase(server.URL)
	s.T().Cleanup(func() { setGitHubAPIBase(origBase) })

	val, err := s.env.ExecuteActivity(s.act.FetchDocsRepo)
	require.NoError(s.T(), err)

	var docsPath string
	require.NoError(s.T(), val.Get(&docsPath))
	s.T().Cleanup(func() { os.RemoveAll(filepath.Dir(docsPath)) })

	// Verify extracted files exist under docs/
	goFile := filepath.Join(docsPath, "develop", "go", "basics.mdx")
	assert.FileExists(s.T(), goFile)

	cloudFile := filepath.Join(docsPath, "cloud", "namespaces.mdx")
	assert.FileExists(s.T(), cloudFile)

	// README should NOT be in docs/
	readmeFile := filepath.Join(docsPath, "README.md")
	assert.NoFileExists(s.T(), readmeFile)
}

func (s *ActivitiesTestSuite) TestReadLocalDocs_Success() {
	docsDir := filepath.Join(s.T().TempDir(), "docs")
	require.NoError(s.T(), os.MkdirAll(filepath.Join(docsDir, "develop", "go"), 0o755))
	require.NoError(s.T(), os.MkdirAll(filepath.Join(docsDir, "cloud"), 0o755))

	goContent := "---\ntitle: Go Basics\n---\n\n# Go SDK Basics\n\nThe Go SDK allows you to write Temporal workflows and activities in Go. It provides a strongly typed API for defining workflows, activities, and workers. You can use it to build reliable distributed systems that handle failures gracefully."
	cloudContent := "---\ntitle: Cloud NS\n---\n\n# Cloud Namespaces\n\nTemporal Cloud namespaces provide isolated environments for your workflows. Each namespace has its own workflow history, task queues, and search attributes. You can create multiple namespaces for different teams or environments."
	require.NoError(s.T(), os.WriteFile(filepath.Join(docsDir, "develop", "go", "basics.mdx"), []byte(goContent), 0o644))
	require.NoError(s.T(), os.WriteFile(filepath.Join(docsDir, "cloud", "namespaces.mdx"), []byte(cloudContent), 0o644))

	// Change to temp dir so temporal_docs_txt is written there.
	origDir, _ := os.Getwd()
	tmpOut := s.T().TempDir()
	os.Chdir(tmpOut)
	s.T().Cleanup(func() { os.Chdir(origDir) })

	val, err := s.env.ExecuteActivity(s.act.ReadLocalDocs, docsDir)
	require.NoError(s.T(), err)

	var result string
	require.NoError(s.T(), val.Get(&result))
	assert.Contains(s.T(), result, "2 files")
	assert.Contains(s.T(), result, "2 buckets")

	// Verify bucket files.
	devFile := filepath.Join(tmpOut, "temporal_docs_txt", "temporal_docs_Develop.txt")
	assert.FileExists(s.T(), devFile)
	devContent, _ := os.ReadFile(devFile)
	assert.Contains(s.T(), string(devContent), "Go SDK Basics")

	cloudFile := filepath.Join(tmpOut, "temporal_docs_txt", "temporal_docs_Temporal_Cloud.txt")
	assert.FileExists(s.T(), cloudFile)
	cloudData, _ := os.ReadFile(cloudFile)
	assert.Contains(s.T(), string(cloudData), "Cloud Namespaces")
}

func (s *ActivitiesTestSuite) TestReadLocalDocs_StripsFrontmatter() {
	docsDir := s.T().TempDir()
	evalContent := "---\ntitle: Evaluate\nsidebar_position: 1\ntags:\n  - getting-started\n---\n\n# Evaluate Temporal\n\nTemporal is a durable execution platform that allows you to build reliable applications. It provides workflow orchestration, activity execution, and automatic retries. This page helps you evaluate whether Temporal is right for your use case."
	require.NoError(s.T(), os.WriteFile(filepath.Join(docsDir, "evaluate.mdx"), []byte(evalContent), 0o644))

	origDir, _ := os.Getwd()
	tmpOut := s.T().TempDir()
	os.Chdir(tmpOut)
	s.T().Cleanup(func() { os.Chdir(origDir) })

	val, err := s.env.ExecuteActivity(s.act.ReadLocalDocs, docsDir)
	require.NoError(s.T(), err)

	var result string
	require.NoError(s.T(), val.Get(&result))
	assert.Contains(s.T(), result, "1 files")

	evalFile := filepath.Join(tmpOut, "temporal_docs_txt", "temporal_docs_Evaluate_and_Concepts.txt")
	assert.FileExists(s.T(), evalFile)
	content, _ := os.ReadFile(evalFile)
	assert.Contains(s.T(), string(content), "Evaluate Temporal")
	assert.NotContains(s.T(), string(content), "sidebar_position")
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "with frontmatter",
			input: "---\ntitle: Foo\ntags: [a, b]\n---\n\nContent here",
			want:  "Content here",
		},
		{
			name:  "no frontmatter",
			input: "Just content\nwith lines",
			want:  "Just content\nwith lines",
		},
		{
			name:  "empty frontmatter",
			input: "---\n---\n\nContent after empty",
			want:  "Content after empty",
		},
		{
			name:  "no closing marker",
			input: "---\ntitle: Broken\nNo closing",
			want:  "---\ntitle: Broken\nNo closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFrontmatter(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
