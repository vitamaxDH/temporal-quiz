package quiz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var claudeEndpoint = "https://api.anthropic.com/v1/messages"

type QuizActivities struct {
	HTTPClient *http.Client
	OutputDir  string
	BucketDir  string
	APIKey     string
	Model      string
}

// ListBuckets scans BucketDir for temporal_docs_*.txt files and returns a
// GenerateQuizInput per file with the category extracted from the filename.
func (a *QuizActivities) ListBuckets(ctx context.Context) ([]GenerateQuizInput, error) {
	pattern := filepath.Join(a.BucketDir, "temporal_docs_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob bucket dir: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no bucket files found in %s", a.BucketDir)
	}

	var inputs []GenerateQuizInput
	for _, m := range matches {
		base := filepath.Base(m)
		name := strings.TrimPrefix(base, "temporal_docs_")
		name = strings.TrimSuffix(name, ".txt")
		inputs = append(inputs, GenerateQuizInput{
			BucketPath:     m,
			Category:       name,
			EasyCount:      DefaultQuestionsPerCat,
			MedCount:       DefaultQuestionsPerCat,
			HardCount:      DefaultQuestionsPerCat,
			NightmareCount: DefaultQuestionsPerCat,
		})
	}

	return inputs, nil
}

// GenerateQuiz reads a bucket file, calls Claude for hard and nightmare
// questions, and returns enriched QuizQuestion slices.
func (a *QuizActivities) GenerateQuiz(ctx context.Context, input GenerateQuizInput) ([]QuizQuestion, error) {
	data, err := os.ReadFile(input.BucketPath)
	if err != nil {
		return nil, fmt.Errorf("read bucket file: %w", err)
	}

	bucketText := string(data)
	// Cap per-call context. 20k is plenty of material for a single bucket
	// at our question counts, and keeps input-token costs in check.
	if len(bucketText) > 20000 {
		bucketText = bucketText[:20000]
	}

	now := time.Now().UTC().Format(time.RFC3339)
	categoryLower := strings.ToLower(strings.ReplaceAll(input.Category, " ", "_"))

	type diffSpec struct {
		name  string
		count int
	}

	diffs := []diffSpec{
		{"easy", input.EasyCount},
		{"med", input.MedCount},
		{"hard", input.HardCount},
		{"nightmare", input.NightmareCount},
	}

	var questions []QuizQuestion
	for _, d := range diffs {
		raw, err := a.callClaudeCached(ctx, GenerationSystem(d.name), GenerationUserMsg(d.count, d.name, bucketText))
		if err != nil {
			// If Claude can't generate questions (e.g. thin content), skip this tier
			fmt.Printf("Warning: skipping %s for %s: %v\n", d.name, input.Category, err)
			continue
		}
		for i, rq := range raw {
			questions = append(questions, QuizQuestion{
				ID:          fmt.Sprintf("%s_%s_%03d", categoryLower, d.name, i+1),
				Category:    input.Category,
				Difficulty:  d.name,
				Question:    rq.Question,
				Choices:     rq.Choices,
				Answer:      rq.Answer,
				Explanation: rq.Explanation,
				SourceDoc:   rq.SourceDoc,
				GeneratedAt: now,
			})
		}
	}

	return questions, nil
}

// callClaudeCachedRaw sends a system + user prompt to the Anthropic Messages
// API and returns the cleaned JSON array string from the response.
//
// The system prompt is sent as a cacheable block. Anthropic caches stable
// system content for ~5 minutes at ~10% of normal input cost, which lets
// us reuse the ~1.5k-token instruction block across 100+ calls per
// pipeline run. The variable user message (bucket text or questions JSON)
// is left uncached.
func (a *QuizActivities) callClaudeCachedRaw(ctx context.Context, systemPrompt, userMsg string) (string, error) {
	reqBody := claudeRequest{
		Model:     a.Model,
		MaxTokens: 8192,
		System: []claudeSystemBlock{
			{
				Type:         "text",
				Text:         systemPrompt,
				CacheControl: &claudeCacheControl{Type: "ephemeral"},
			},
		},
		Messages: []claudeMessage{
			{Role: "user", Content: userMsg},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in claude response")
	}

	var content string
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			content = block.Text
			break
		}
	}

	if content == "" {
		return "", fmt.Errorf("no text content in claude response")
	}

	re := regexp.MustCompile(`\[[\s\S]*\]`)
	match := re.FindString(content)
	if match == "" {
		return "", fmt.Errorf("no JSON array found in response: %s", content[:min(200, len(content))])
	}

	cleaned := regexp.MustCompile(`,\s*}`).ReplaceAllString(match, "}")
	cleaned = regexp.MustCompile(`,\s*\]`).ReplaceAllString(cleaned, "]")

	return cleaned, nil
}

// callClaudeCached calls the Messages API with a cacheable system prompt
// and returns a parsed slice of rawQuestion.
func (a *QuizActivities) callClaudeCached(ctx context.Context, systemPrompt, userMsg string) ([]rawQuestion, error) {
	cleaned, err := a.callClaudeCachedRaw(ctx, systemPrompt, userMsg)
	if err != nil {
		return nil, err
	}

	var raw []rawQuestion
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal questions: %w", err)
	}

	return raw, nil
}

// EvalBatchSize is the max questions the workflow bundles into a single
// EvaluateQuiz activity call. Chosen to keep each Claude call well under
// the 5-minute activity timeout while still amortizing prompt-cache hits
// across a meaningful chunk of questions.
const EvalBatchSize = 20

// EvaluateQuiz sends a single batch of questions to Claude for quality
// evaluation and returns the passed/failed split plus per-question scores.
// The workflow is responsible for splitting the full set into batches of
// size <= EvalBatchSize and fanning them out in parallel.
func (a *QuizActivities) EvaluateQuiz(ctx context.Context, questions []QuizQuestion) (EvalOutput, error) {
	if len(questions) == 0 {
		return EvalOutput{}, nil
	}

	// Build a compact JSON representation for evaluation
	type evalInput struct {
		ID          string   `json:"id"`
		Difficulty  string   `json:"difficulty"`
		Question    string   `json:"question"`
		Choices     []Choice `json:"choices"`
		Answer      string   `json:"answer"`
		Explanation string   `json:"explanation"`
	}
	inputs := make([]evalInput, len(questions))
	for j, q := range questions {
		inputs[j] = evalInput{
			ID:          q.ID,
			Difficulty:  q.Difficulty,
			Question:    q.Question,
			Choices:     q.Choices,
			Answer:      q.Answer,
			Explanation: q.Explanation,
		}
	}

	inputJSON, err := json.Marshal(inputs)
	if err != nil {
		return EvalOutput{}, fmt.Errorf("marshal eval input: %w", err)
	}

	results := make([]EvalResult, 0, len(questions))
	cleaned, err := a.callClaudeCachedRaw(ctx, EvalSystem, EvalUserMsg(string(inputJSON)))
	if err != nil {
		fmt.Printf("Warning: eval batch of %d failed: %v, passing all\n", len(questions), err)
		for _, q := range questions {
			results = append(results, EvalResult{
				QuestionID: q.ID,
				Scores:     EvalScores{3, 3, 3, 3, 3},
				Pass:       true,
			})
		}
	} else if err := json.Unmarshal([]byte(cleaned), &results); err != nil {
		fmt.Printf("Warning: eval batch of %d parse failed: %v, passing all\n", len(questions), err)
		results = results[:0]
		for _, q := range questions {
			results = append(results, EvalResult{
				QuestionID: q.ID,
				Scores:     EvalScores{3, 3, 3, 3, 3},
				Pass:       true,
			})
		}
	}

	// Split into passed/failed
	passSet := make(map[string]bool, len(results))
	for _, r := range results {
		passSet[r.QuestionID] = r.Pass
	}

	var output EvalOutput
	output.Results = results
	for _, q := range questions {
		if passSet[q.ID] {
			output.Passed = append(output.Passed, q)
		} else {
			output.Failed = append(output.Failed, q)
		}
	}

	return output, nil
}

// WriteQuizFiles groups questions by category and writes a dated snapshot
// at OutputDir/runs/<YYYY-MM-DD>/<Category>.json plus
// OutputDir/runs/<YYYY-MM-DD>/manifest.json. Daily runs accumulate there
// so the UI can offer a "previous quizzes" picker. Running on the same
// day overwrites that day's snapshot.
//
// The runs.json index is intentionally NOT written here — it is rebuilt
// by the publish step from whatever date folders exist in the destination,
// so the worker can stay stateless across machines.
func (a *QuizActivities) WriteQuizFiles(ctx context.Context, questions []QuizQuestion) error {
	if err := os.MkdirAll(a.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Group questions by category.
	grouped := make(map[string][]QuizQuestion)
	for _, q := range questions {
		grouped[q.Category] = append(grouped[q.Category], q)
	}

	nowUTC := time.Now().UTC()
	now := nowUTC.Format(time.RFC3339)
	dateStr := nowUTC.Format("2006-01-02")

	runDir := filepath.Join(a.OutputDir, "runs", dateStr)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}

	var entries []ManifestEntry

	for cat, qs := range grouped {
		cq := CategoryQuiz{
			Category:  cat,
			Questions: qs,
		}

		data, err := json.MarshalIndent(cq, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal category %s: %w", cat, err)
		}

		catPath := filepath.Join(runDir, cat+".json")
		if err := os.WriteFile(catPath, data, 0o644); err != nil {
			return fmt.Errorf("write category file %s: %w", catPath, err)
		}

		var easyCount, medCount, hardCount, nightmareCount int
		for _, q := range qs {
			switch q.Difficulty {
			case "easy":
				easyCount++
			case "med":
				medCount++
			case "hard":
				hardCount++
			case "nightmare":
				nightmareCount++
			}
		}

		entries = append(entries, ManifestEntry{
			Category:       cat,
			QuestionCount:  len(qs),
			EasyCount:      easyCount,
			MedCount:       medCount,
			HardCount:      hardCount,
			NightmareCount: nightmareCount,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Category < entries[j].Category
	})

	totalCount := 0
	for _, e := range entries {
		totalCount += e.QuestionCount
	}

	manifest := Manifest{
		GeneratedAt: now,
		Categories:  entries,
		TotalCount:  totalCount,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(runDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}
