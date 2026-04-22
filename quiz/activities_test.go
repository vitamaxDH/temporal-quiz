package quiz

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readGzippedJSON decodes a gzipped category file — the on-disk format
// WriteQuizFiles produces for per-category payloads.
func readGzippedJSON(t *testing.T, path string, out interface{}) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer gz.Close()
	data, err := io.ReadAll(gz)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, out))
}

func TestListBuckets_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Write 2 sample bucket files.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "temporal_docs_workflows.txt"), []byte("workflow docs"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "temporal_docs_activities.txt"), []byte("activity docs"), 0o644))

	a := &QuizActivities{BucketDir: tmpDir}
	inputs, err := a.ListBuckets(t.Context())
	require.NoError(t, err)

	assert.Len(t, inputs, 2)

	categories := map[string]bool{}
	for _, inp := range inputs {
		categories[inp.Category] = true
		assert.Equal(t, DefaultQuestionsPerCat, inp.EasyCount)
		assert.Equal(t, DefaultQuestionsPerCat, inp.MedCount)
		assert.Equal(t, DefaultQuestionsPerCat, inp.HardCount)
		assert.Equal(t, DefaultQuestionsPerCat, inp.NightmareCount)
	}
	assert.True(t, categories["workflows"])
	assert.True(t, categories["activities"])
}

func TestListBuckets_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	a := &QuizActivities{BucketDir: tmpDir}
	_, err := a.ListBuckets(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no bucket files found")
}

func TestGenerateQuiz_Success(t *testing.T) {
	// Build a mock OpenAI response with a single rawQuestion.
	mockQuestion := rawQuestion{
		Question: "What happens when a workflow times out?",
		Choices: []Choice{
			{Key: "A", Text: "It retries"},
			{Key: "B", Text: "It fails"},
			{Key: "C", Text: "It pauses"},
			{Key: "D", Text: "It continues"},
		},
		Answer:      "B",
		Explanation: "Workflows fail on timeout.",
		SourceDoc:   "timeouts.html",
	}
	mockQuestionsJSON, err := json.Marshal([]rawQuestion{mockQuestion})
	require.NoError(t, err)

	mockResp := claudeResponse{
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{
			{Type: "text", Text: string(mockQuestionsJSON)},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	origEndpoint := claudeEndpoint
	claudeEndpoint = server.URL
	defer func() { claudeEndpoint = origEndpoint }()

	tmpDir := t.TempDir()
	bucketPath := filepath.Join(tmpDir, "temporal_docs_features_workflows.txt")
	require.NoError(t, os.WriteFile(bucketPath, []byte("some workflow documentation text"), 0o644))

	a := &QuizActivities{
		HTTPClient: server.Client(),
		BucketDir:  tmpDir,
		APIKey:     "test-api-key",
		Model:      "claude-sonnet-4-6",
	}

	input := GenerateQuizInput{
		BucketPath:     bucketPath,
		Category:       "Features Workflows",
		EasyCount:      1,
		MedCount:       1,
		HardCount:      1,
		NightmareCount: 1,
	}

	questions, err := a.GenerateQuiz(t.Context(), input)
	require.NoError(t, err)
	assert.Len(t, questions, 4) // 1 easy + 1 med + 1 hard + 1 nightmare

	// Verify easy question.
	easy := questions[0]
	assert.Equal(t, "features_workflows_easy_001", easy.ID)
	assert.Equal(t, "Features Workflows", easy.Category)
	assert.Equal(t, "easy", easy.Difficulty)
	assert.Equal(t, mockQuestion.Question, easy.Question)
	assert.Equal(t, mockQuestion.Answer, easy.Answer)
	assert.NotEmpty(t, easy.GeneratedAt)

	// Verify med question.
	med := questions[1]
	assert.Equal(t, "features_workflows_med_001", med.ID)
	assert.Equal(t, "Features Workflows", med.Category)
	assert.Equal(t, "med", med.Difficulty)

	// Verify hard question.
	hard := questions[2]
	assert.Equal(t, "features_workflows_hard_001", hard.ID)
	assert.Equal(t, "Features Workflows", hard.Category)
	assert.Equal(t, "hard", hard.Difficulty)

	// Verify nightmare question.
	nightmare := questions[3]
	assert.Equal(t, "features_workflows_nightmare_001", nightmare.ID)
	assert.Equal(t, "Features Workflows", nightmare.Category)
	assert.Equal(t, "nightmare", nightmare.Difficulty)
}

func TestWriteQuizFiles_Success(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	a := &QuizActivities{OutputDir: outputDir}

	questions := []QuizQuestion{
		{
			ID:         "develop_easy_001",
			Category:   "develop",
			Difficulty: "easy",
			Question:   "Q1",
			Choices:    []Choice{{Key: "A", Text: "A1"}},
			Answer:     "A",
		},
		{
			ID:         "develop_med_001",
			Category:   "develop",
			Difficulty: "med",
			Question:   "Q2",
			Choices:    []Choice{{Key: "A", Text: "A2"}},
			Answer:     "A",
		},
		{
			ID:         "develop_hard_001",
			Category:   "develop",
			Difficulty: "hard",
			Question:   "Q3",
			Choices:    []Choice{{Key: "A", Text: "A3"}},
			Answer:     "A",
		},
		{
			ID:         "develop_nightmare_001",
			Category:   "develop",
			Difficulty: "nightmare",
			Question:   "Q4",
			Choices:    []Choice{{Key: "A", Text: "A4"}},
			Answer:     "A",
		},
		{
			ID:         "cli_hard_001",
			Category:   "cli",
			Difficulty: "hard",
			Question:   "Q5",
			Choices:    []Choice{{Key: "A", Text: "A5"}},
			Answer:     "A",
		},
	}

	err := a.WriteQuizFiles(t.Context(), questions)
	require.NoError(t, err)

	// WriteQuizFiles writes only the dated snapshot. We discover today's
	// run dir dynamically (the test runs in real time).
	runsDir := filepath.Join(outputDir, "runs")
	runDirs, err := os.ReadDir(runsDir)
	require.NoError(t, err)
	require.Len(t, runDirs, 1, "expected exactly one dated run directory")
	require.True(t, runDirs[0].IsDir(), "expected run entry to be a directory")

	runDir := filepath.Join(runsDir, runDirs[0].Name())

	// Verify manifest.json under the run dir.
	manifestData, err := os.ReadFile(filepath.Join(runDir, "manifest.json"))
	require.NoError(t, err)

	var manifest Manifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	assert.Equal(t, 5, manifest.TotalCount)
	assert.Len(t, manifest.Categories, 2)

	// Categories should be sorted alphabetically.
	assert.Equal(t, "cli", manifest.Categories[0].Category)
	assert.Equal(t, "develop", manifest.Categories[1].Category)

	// Verify difficulty counts.
	assert.Equal(t, 0, manifest.Categories[0].EasyCount)
	assert.Equal(t, 0, manifest.Categories[0].MedCount)
	assert.Equal(t, 1, manifest.Categories[0].HardCount)
	assert.Equal(t, 0, manifest.Categories[0].NightmareCount)
	assert.Equal(t, 1, manifest.Categories[1].EasyCount)
	assert.Equal(t, 1, manifest.Categories[1].MedCount)
	assert.Equal(t, 1, manifest.Categories[1].HardCount)
	assert.Equal(t, 1, manifest.Categories[1].NightmareCount)

	// Verify per-category JSON files under the run dir. They're written
	// as .json.gz; readGzippedJSON decompresses and unmarshals.
	var devQuiz CategoryQuiz
	readGzippedJSON(t, filepath.Join(runDir, "develop.json.gz"), &devQuiz)
	assert.Equal(t, "develop", devQuiz.Category)
	assert.Len(t, devQuiz.Questions, 4)

	var cliQuiz CategoryQuiz
	readGzippedJSON(t, filepath.Join(runDir, "cli.json.gz"), &cliQuiz)
	assert.Equal(t, "cli", cliQuiz.Category)
	assert.Len(t, cliQuiz.Questions, 1)

	// No flat-file mirror should exist at the top of the output dir.
	_, err = os.Stat(filepath.Join(outputDir, "manifest.json"))
	assert.True(t, os.IsNotExist(err), "manifest.json should NOT exist at outputDir root")
	_, err = os.Stat(filepath.Join(outputDir, "develop.json.gz"))
	assert.True(t, os.IsNotExist(err), "develop.json.gz should NOT exist at outputDir root")
}

func TestEvaluateQuiz_Success(t *testing.T) {
	// Build a mock eval response: q1 passes, q2 fails
	evalResults := []EvalResult{
		{QuestionID: "q1", Scores: EvalScores{5, 5, 4, 4, 5}, Pass: true, Feedback: ""},
		{QuestionID: "q2", Scores: EvalScores{2, 5, 3, 4, 5}, Pass: false, Feedback: "Ambiguous wording"},
	}
	evalJSON, err := json.Marshal(evalResults)
	require.NoError(t, err)

	mockResp := claudeResponse{
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{
			{Type: "text", Text: string(evalJSON)},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	origEndpoint := claudeEndpoint
	claudeEndpoint = server.URL
	defer func() { claudeEndpoint = origEndpoint }()

	a := &QuizActivities{
		HTTPClient: server.Client(),
		APIKey:     "test-key",
		Model:      "claude-sonnet-4-6",
	}

	questions := []QuizQuestion{
		{ID: "q1", Category: "test", Difficulty: "easy", Question: "Good question"},
		{ID: "q2", Category: "test", Difficulty: "hard", Question: "Ambiguous question"},
	}

	output, err := a.EvaluateQuiz(t.Context(), questions)
	require.NoError(t, err)

	assert.Len(t, output.Passed, 1)
	assert.Len(t, output.Failed, 1)
	assert.Equal(t, "q1", output.Passed[0].ID)
	assert.Equal(t, "q2", output.Failed[0].ID)
	assert.Len(t, output.Results, 2)
	assert.True(t, output.Results[0].Pass)
	assert.False(t, output.Results[1].Pass)
	assert.Equal(t, "Ambiguous wording", output.Results[1].Feedback)
}
