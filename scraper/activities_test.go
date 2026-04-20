package scraper

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
		Client: &http.Client{},
	}
	s.env.RegisterActivity(s.act.FetchDocsRepo)
	s.env.RegisterActivity(s.act.ReadLocalDocs)
}

func TestActivitiesTestSuite(t *testing.T) {
	suite.Run(t, new(ActivitiesTestSuite))
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

	cloudFile := filepath.Join(tmpOut, "temporal_docs_txt", "temporal_docs_Operations_TemporalCloud.txt")
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

	evalFile := filepath.Join(tmpOut, "temporal_docs_txt", "temporal_docs_General_Concepts.txt")
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
