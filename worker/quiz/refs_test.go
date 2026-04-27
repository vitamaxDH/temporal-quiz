package quiz

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// FetchAllowlistSuite covers the sitemap-fetching activity. We swap in
// an httptest server so the test never hits the real docs site.
type FetchAllowlistSuite struct {
	suite.Suite
	server   *httptest.Server
	prevURL  string
	handler  http.HandlerFunc
	activity *QuizActivities
}

func (s *FetchAllowlistSuite) SetupTest() {
	s.handler = func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handler(w, r)
	}))
	s.prevURL = docsSitemapURL
	docsSitemapURL = s.server.URL + "/sitemap.xml"
	s.activity = &QuizActivities{HTTPClient: s.server.Client()}
}

func (s *FetchAllowlistSuite) TearDownTest() {
	docsSitemapURL = s.prevURL
	s.server.Close()
}

func TestFetchAllowlistSuite(t *testing.T) {
	suite.Run(t, new(FetchAllowlistSuite))
}

func (s *FetchAllowlistSuite) TestUrlset_Success() {
	s.handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://docs.temporal.io/develop/go/workflows</loc></url>
  <url><loc>https://docs.temporal.io/cli/</loc></url>
  <url><loc>https://example.com/not-temporal</loc></url>
</urlset>`))
	}

	urls, err := s.activity.FetchDocsURLAllowlist(s.T().Context())
	require.NoError(s.T(), err)
	assert.ElementsMatch(s.T(), []string{
		"https://docs.temporal.io/develop/go/workflows",
		"https://docs.temporal.io/cli",
	}, urls)
}

func (s *FetchAllowlistSuite) TestSitemapIndex_FollowsNested() {
	indexURL := s.server.URL + "/sitemap.xml"
	nestedURL := s.server.URL + "/sub-sitemap.xml"
	s.handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/sitemap.xml":
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>%s</loc></sitemap>
</sitemapindex>`, nestedURL)
		case "/sub-sitemap.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://docs.temporal.io/cloud/users</loc></url>
</urlset>`))
		default:
			http.NotFound(w, r)
		}
	}
	docsSitemapURL = indexURL

	urls, err := s.activity.FetchDocsURLAllowlist(s.T().Context())
	require.NoError(s.T(), err)
	assert.Equal(s.T(), []string{"https://docs.temporal.io/cloud/users"}, urls)
}

func (s *FetchAllowlistSuite) TestSitemap_HTTPError() {
	s.handler = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}
	_, err := s.activity.FetchDocsURLAllowlist(s.T().Context())
	require.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "status 500")
}

func TestNormalizeDocsURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://docs.temporal.io/develop/go/workflows/", "https://docs.temporal.io/develop/go/workflows"},
		{"https://docs.temporal.io/cli/#section", "https://docs.temporal.io/cli"},
		{"https://docs.temporal.io/cli/?utm_source=x", "https://docs.temporal.io/cli"},
		{"DOCS.temporal.io/develop", "https://docs.temporal.io/develop"},
		{"docs.temporal.io/develop", "https://docs.temporal.io/develop"},
		{"https://example.com/develop", ""},
		{"  ", ""},
		{"not-a-url", ""},
		{"https://docs.temporal.io", "https://docs.temporal.io"},
		{"https://docs.temporal.io/", "https://docs.temporal.io"},
	}
	for _, tc := range cases {
		got := NormalizeDocsURL(tc.in)
		assert.Equal(t, tc.want, got, "input=%q", tc.in)
	}
}

func TestValidateReferences(t *testing.T) {
	a := &QuizActivities{}
	allowlist := []string{
		"https://docs.temporal.io/develop/go/workflows",
		"https://docs.temporal.io/cli",
	}
	in := ValidateReferencesInput{
		Allowlist: allowlist,
		Questions: []QuizQuestion{
			{ID: "valid_1", Reference: "https://docs.temporal.io/develop/go/workflows"},
			{ID: "valid_trailing_slash", Reference: "https://docs.temporal.io/develop/go/workflows/"},
			{ID: "valid_fragment", Reference: "https://docs.temporal.io/cli/#start"},
			{ID: "valid_empty", Reference: ""},
			{ID: "invalid_typo", Reference: "https://docs.temporal.io/develop/go/workflowz"},
			{ID: "invalid_offsite", Reference: "https://example.com/develop"},
			{ID: "invalid_garbage", Reference: "not-a-url"},
		},
	}
	out, err := a.ValidateReferences(t.Context(), in)
	require.NoError(t, err)

	validIDs := make([]string, len(out.Valid))
	for i, q := range out.Valid {
		validIDs[i] = q.ID
	}
	invalidIDs := make([]string, len(out.Invalid))
	for i, q := range out.Invalid {
		invalidIDs[i] = q.ID
	}
	assert.ElementsMatch(t, []string{"valid_1", "valid_trailing_slash", "valid_fragment", "valid_empty"}, validIDs)
	assert.ElementsMatch(t, []string{"invalid_typo", "invalid_offsite", "invalid_garbage"}, invalidIDs)
}

func TestCandidateURLsForSource(t *testing.T) {
	allowlist := []string{
		"https://docs.temporal.io/develop/go/workflows",
		"https://docs.temporal.io/develop/go/workflows/schedules",
		"https://docs.temporal.io/develop/go/activities",
		"https://docs.temporal.io/cli",
	}
	t.Run("matches_full_prefix", func(t *testing.T) {
		got := candidateURLsForSource("develop_go_workflows.html", allowlist)
		assert.Equal(t, []string{
			"https://docs.temporal.io/develop/go/workflows",
			"https://docs.temporal.io/develop/go/workflows/schedules",
		}, got)
	})
	t.Run("falls_back_to_shorter_prefix", func(t *testing.T) {
		got := candidateURLsForSource("develop_go_unknown_subpage.html", allowlist)
		assert.Equal(t, []string{
			"https://docs.temporal.io/develop/go/activities",
			"https://docs.temporal.io/develop/go/workflows",
			"https://docs.temporal.io/develop/go/workflows/schedules",
		}, got)
	})
	t.Run("no_match_returns_nil", func(t *testing.T) {
		got := candidateURLsForSource("totally_unrelated.html", allowlist)
		assert.Nil(t, got)
	})
	t.Run("empty_source_doc_returns_nil", func(t *testing.T) {
		got := candidateURLsForSource("", allowlist)
		assert.Nil(t, got)
	})
}

// FixReferenceSuite covers the FixReference activity end-to-end, mocking
// the Claude HTTP endpoint to keep tests fast and offline.
type FixReferenceSuite struct {
	suite.Suite
	server    *httptest.Server
	prevURL   string
	respond   func(req []byte) string
	activity  *QuizActivities
	allowlist []string
}

func (s *FixReferenceSuite) SetupTest() {
	s.respond = func(req []byte) string { return `[{"reference":""}]` }
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var resp = map[string]any{
			"content": []map[string]any{{"type": "text", "text": s.respond(body)}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	s.prevURL = claudeEndpoint
	claudeEndpoint = s.server.URL
	s.activity = &QuizActivities{
		HTTPClient: s.server.Client(),
		APIKey:     "test",
		Model:      "test-model",
	}
	s.allowlist = []string{
		"https://docs.temporal.io/develop/go/workflows",
		"https://docs.temporal.io/develop/go/workflows/schedules",
		"https://docs.temporal.io/develop/go/activities",
	}
}

func (s *FixReferenceSuite) TearDownTest() {
	claudeEndpoint = s.prevURL
	s.server.Close()
}

func TestFixReferenceSuite(t *testing.T) {
	suite.Run(t, new(FixReferenceSuite))
}

func (s *FixReferenceSuite) TestSingleCandidate_NoClaudeCall() {
	allowlist := []string{"https://docs.temporal.io/develop/go/workflows"}
	// Make Claude fail loudly to assert it was never called.
	s.respond = func(req []byte) string {
		s.T().Fatal("Claude should not be called when there is exactly one candidate")
		return ""
	}
	out, err := s.activity.FixReference(s.T().Context(), FixReferenceInput{
		Question:  QuizQuestion{ID: "q1", SourceDoc: "develop_go_workflows.html"},
		Allowlist: allowlist,
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "https://docs.temporal.io/develop/go/workflows", out.FixedReference)
	assert.False(s.T(), out.Drop)
}

func (s *FixReferenceSuite) TestMultipleCandidates_ClaudePicksValid() {
	s.respond = func(req []byte) string {
		assert.Contains(s.T(), string(req), "develop/go/workflows/schedules")
		return `[{"reference":"https://docs.temporal.io/develop/go/workflows/schedules"}]`
	}
	out, err := s.activity.FixReference(s.T().Context(), FixReferenceInput{
		Question:  QuizQuestion{ID: "q1", SourceDoc: "develop_go_workflows.html"},
		Allowlist: s.allowlist,
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "https://docs.temporal.io/develop/go/workflows/schedules", out.FixedReference)
}

func (s *FixReferenceSuite) TestMultipleCandidates_ClaudePicksInvalid() {
	s.respond = func(req []byte) string {
		return `[{"reference":"https://docs.temporal.io/totally/different/path"}]`
	}
	out, err := s.activity.FixReference(s.T().Context(), FixReferenceInput{
		Question:  QuizQuestion{ID: "q1", SourceDoc: "develop_go_workflows.html"},
		Allowlist: s.allowlist,
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "", out.FixedReference, "off-allowlist Claude pick must be rejected")
}

func (s *FixReferenceSuite) TestNoCandidates_ReturnsEmpty() {
	s.respond = func(req []byte) string {
		s.T().Fatal("Claude should not be called when there are no candidates")
		return ""
	}
	out, err := s.activity.FixReference(s.T().Context(), FixReferenceInput{
		Question:  QuizQuestion{ID: "q1", SourceDoc: "totally_unrelated.html"},
		Allowlist: s.allowlist,
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "", out.FixedReference)
}

func (s *FixReferenceSuite) TestClaudeError_FallsBackToEmpty() {
	s.respond = func(req []byte) string {
		// Garbage that fails JSON-array extraction.
		return "no json here"
	}
	out, err := s.activity.FixReference(s.T().Context(), FixReferenceInput{
		Question:  QuizQuestion{ID: "q1", SourceDoc: "develop_go_workflows.html"},
		Allowlist: s.allowlist,
	})
	require.NoError(s.T(), err, "Claude failure must not bubble up; we keep the question with empty ref")
	assert.Equal(s.T(), "", out.FixedReference)
}
