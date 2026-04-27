package quiz

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// docsSitemapURL is the canonical sitemap to fetch when building the
// allowlist of valid docs.temporal.io URLs. It is package-level so tests
// can swap it out for an httptest server.
var docsSitemapURL = "https://docs.temporal.io/sitemap.xml"

// docsHost is the hostname every reference URL must match after
// normalization. Anything else is treated as out-of-scope.
const docsHost = "docs.temporal.io"

// sitemapURL covers both <url><loc>...</loc></url> entries (in a urlset)
// and <sitemap><loc>...</loc></sitemap> entries (in a sitemapindex), which
// share the same XML shape.
type sitemapURL struct {
	Loc string `xml:"loc"`
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Sitemaps []sitemapURL `xml:"sitemap"`
}

// FetchDocsURLAllowlist fetches docsSitemapURL and returns a sorted,
// deduplicated, normalized list of every URL on docs.temporal.io that
// appears in the sitemap. Sitemap indexes are followed one level deep.
func (a *QuizActivities) FetchDocsURLAllowlist(ctx context.Context) ([]string, error) {
	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	seen := make(map[string]struct{})
	if err := fetchSitemapInto(ctx, client, docsSitemapURL, seen, 0); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	sort.Strings(out)
	return out, nil
}

// fetchSitemapInto fetches one sitemap (urlset OR sitemapindex), recursing
// up to two levels for nested indexes. A nested sitemap that fails to
// fetch is logged and skipped rather than failing the whole allowlist.
func fetchSitemapInto(ctx context.Context, client *http.Client, sitemapURL string, seen map[string]struct{}, depth int) error {
	if depth > 2 {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return fmt.Errorf("build sitemap request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch sitemap %s: %w", sitemapURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sitemap %s returned status %d", sitemapURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read sitemap body: %w", err)
	}
	if idx, ok := parseSitemapIndex(data); ok {
		for _, sm := range idx.Sitemaps {
			if sm.Loc == "" {
				continue
			}
			if err := fetchSitemapInto(ctx, client, sm.Loc, seen, depth+1); err != nil {
				fmt.Printf("Warning: nested sitemap %s skipped: %v\n", sm.Loc, err)
				continue
			}
		}
		return nil
	}
	var us urlset
	if err := xml.Unmarshal(data, &us); err != nil {
		return fmt.Errorf("parse sitemap: %w", err)
	}
	for _, u := range us.URLs {
		if normalized := NormalizeDocsURL(u.Loc); normalized != "" {
			seen[normalized] = struct{}{}
		}
	}
	return nil
}

// parseSitemapIndex returns the index when the document is a sitemap
// index (root element <sitemapindex>) and false otherwise. We can't tell
// from a single Unmarshal call because Go silently ignores unknown
// elements, so we sniff the root via xml.Decoder.
func parseSitemapIndex(data []byte) (sitemapIndex, bool) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return sitemapIndex{}, false
		}
		if start, ok := tok.(xml.StartElement); ok {
			if start.Name.Local != "sitemapindex" {
				return sitemapIndex{}, false
			}
			var idx sitemapIndex
			if err := xml.Unmarshal(data, &idx); err != nil {
				return sitemapIndex{}, false
			}
			return idx, len(idx.Sitemaps) > 0
		}
	}
}

// NormalizeDocsURL canonicalizes a docs.temporal.io URL for set-membership
// comparisons: lowercases the host, strips the fragment and query string,
// drops the trailing slash, and forces https when no scheme is present.
// Returns the empty string for non-docs URLs or unparseable input.
func NormalizeDocsURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil || u == nil {
		return ""
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	u.Host = strings.ToLower(u.Host)
	if u.Host != docsHost {
		return ""
	}
	u.Fragment = ""
	u.RawFragment = ""
	u.RawQuery = ""
	s := u.String()
	if s == "https://"+docsHost {
		return s
	}
	return strings.TrimSuffix(s, "/")
}

// ValidateReferencesInput is the activity input for ValidateReferences.
// Allowlist is the set of canonical URLs to check against (typically the
// output of FetchDocsURLAllowlist).
type ValidateReferencesInput struct {
	Questions []QuizQuestion
	Allowlist []string
}

// ValidateReferencesOutput partitions the input questions by whether
// their Reference field appears in the allowlist after normalization.
// Empty references count as Valid (the question makes no claim).
type ValidateReferencesOutput struct {
	Valid   []QuizQuestion
	Invalid []QuizQuestion
}

// ValidateReferences splits questions into Valid and Invalid based on
// whether the Reference URL appears in the allowlist after normalization.
func (a *QuizActivities) ValidateReferences(ctx context.Context, in ValidateReferencesInput) (ValidateReferencesOutput, error) {
	set := make(map[string]struct{}, len(in.Allowlist))
	for _, u := range in.Allowlist {
		if n := NormalizeDocsURL(u); n != "" {
			set[n] = struct{}{}
		}
	}
	var out ValidateReferencesOutput
	for _, q := range in.Questions {
		ref := strings.TrimSpace(q.Reference)
		if ref == "" {
			out.Valid = append(out.Valid, q)
			continue
		}
		norm := NormalizeDocsURL(ref)
		if norm == "" {
			out.Invalid = append(out.Invalid, q)
			continue
		}
		if _, ok := set[norm]; ok {
			out.Valid = append(out.Valid, q)
		} else {
			out.Invalid = append(out.Invalid, q)
		}
	}
	return out, nil
}

// FixReferenceInput is the activity input for FixReference. The full
// allowlist is passed in; FixReference filters it down to a candidate
// shortlist before invoking Claude.
type FixReferenceInput struct {
	Question  QuizQuestion
	Allowlist []string
}

// FixReferenceOutput carries the activity result. FixedReference is
// either a URL drawn directly from the allowlist or the empty string
// (meaning "drop the reference field, keep the question"). Drop is
// reserved for future use; today we never drop questions for bad refs.
type FixReferenceOutput struct {
	FixedReference string
	Drop           bool
}

// FixReference picks a valid URL from the allowlist for a question whose
// existing Reference is broken. Strategy:
//   - Build a candidate set by matching path prefixes derived from the
//     question's source_doc (underscore -> slash conversion).
//   - 0 candidates: return empty (caller drops the reference field).
//   - 1 candidate: use it directly, no Claude call.
//   - 2+ candidates: ask Claude to pick the best one. Whatever Claude
//     returns must be in the candidate set verbatim or we fall back to
//     empty.
func (a *QuizActivities) FixReference(ctx context.Context, in FixReferenceInput) (FixReferenceOutput, error) {
	candidates := candidateURLsForSource(in.Question.SourceDoc, in.Allowlist)
	if len(candidates) == 0 {
		return FixReferenceOutput{}, nil
	}
	if len(candidates) == 1 {
		return FixReferenceOutput{FixedReference: candidates[0]}, nil
	}
	pick, err := a.askClaudePickReference(ctx, in.Question, candidates)
	if err != nil {
		fmt.Printf("Warning: FixReference Claude call failed for %s: %v\n", in.Question.ID, err)
		return FixReferenceOutput{}, nil
	}
	pickNorm := NormalizeDocsURL(pick)
	for _, c := range candidates {
		if c == pickNorm {
			return FixReferenceOutput{FixedReference: c}, nil
		}
	}
	return FixReferenceOutput{}, nil
}

// candidateURLsForSource filters the allowlist to URLs whose path shares
// a meaningful prefix with the source_doc. Underscores in the source_doc
// are converted to slashes. We try the full prefix first, then peel
// segments off the end until we get matches or exhaust the parts.
func candidateURLsForSource(sourceDoc string, allowlist []string) []string {
	if sourceDoc == "" {
		return nil
	}
	base := strings.TrimSuffix(strings.ToLower(sourceDoc), ".html")
	parts := strings.Split(base, "_")
	for i := len(parts); i > 0; i-- {
		prefix := "/" + strings.Join(parts[:i], "/")
		var matches []string
		for _, u := range allowlist {
			parsed, err := url.Parse(u)
			if err != nil || parsed == nil {
				continue
			}
			if strings.HasPrefix(strings.ToLower(parsed.Path), prefix) {
				matches = append(matches, u)
			}
		}
		if len(matches) > 0 {
			sort.Strings(matches)
			return matches
		}
	}
	return nil
}

// askClaudePickReference asks Claude to pick exactly one URL from the
// candidate list. The response is wrapped in a single-element JSON array
// so the existing callClaudeCachedRaw helper (which extracts JSON arrays)
// can be reused without a second extraction code path.
func (a *QuizActivities) askClaudePickReference(ctx context.Context, q QuizQuestion, candidates []string) (string, error) {
	const sys = `You are choosing the most appropriate documentation URL to attach as the "reference" field for a Temporal quiz question. You will be given the question, its correct answer, the source filename, and a list of CANDIDATE URLs.

Your job:
- Pick exactly ONE URL from the candidates that best supports the correct answer.
- The URL you return MUST be one of the candidates verbatim. Do NOT invent or alter URLs.
- If none of the candidates is plausibly the right page, return an empty string for "reference".

Return ONLY a JSON array containing a single object (no markdown fences, no extra text):
[{"reference":"<one URL from the candidate list, or empty string>"}]`
	payload := struct {
		Question    string   `json:"question"`
		Answer      string   `json:"answer"`
		Explanation string   `json:"explanation"`
		SourceDoc   string   `json:"source_doc"`
		Candidates  []string `json:"candidates"`
	}{
		Question:    q.Question,
		Answer:      q.Answer,
		Explanation: q.Explanation,
		SourceDoc:   q.SourceDoc,
		Candidates:  candidates,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal pick payload: %w", err)
	}
	user := "Pick the best reference URL for this question:\n\n" + string(body)
	cleaned, err := a.callClaudeCachedRaw(ctx, sys, user)
	if err != nil {
		return "", err
	}
	var picks []struct {
		Reference string `json:"reference"`
	}
	if err := json.Unmarshal([]byte(cleaned), &picks); err != nil {
		return "", fmt.Errorf("unmarshal pick: %w", err)
	}
	if len(picks) == 0 {
		return "", nil
	}
	return picks[0].Reference, nil
}
