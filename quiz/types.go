package quiz

import "slices"

const (
	TaskQueue              = "scraper-task-queue"
	DefaultModel           = "claude-sonnet-4-6"
	DefaultQuestionsPerCat = 10
	DefaultOutputDir       = "web/quizzes"
	BucketDir              = "temporal_docs_txt"

	// Multiplier applied to per-difficulty counts for PriorityCategories.
	// Bigger pool -> more questions survive the eval filter -> priority
	// categories keep showing up in daily runs.
	PriorityCountMultiplier = 2
)

// PriorityCategories are the core Temporal topics that MUST have coverage
// in every daily run. Drawn from the structure of
// https://github.com/temporalio/documentation plus the features every
// Temporal user hits early: workflow primitives, execution state,
// messaging, scheduling, retries, and security.
//
// QuizGeneratorWorkflow generates more questions per priority bucket
// (PriorityCountMultiplier) and falls back to pre-eval output if the
// eval filter zeros a priority category, so coverage is guaranteed
// even when eval is strict.
var PriorityCategories = []string{
	"Features_Workflows",
	"Features_Activities",
	"Features_Workers",
	"Features_TaskQueues",
	"Features_Signals_Queries_Updates",
	"Features_Schedules",
	"Features_Timers_Retries",
	"Features_Nexus",
	"Features_History",
	"Features_WorkerVersioning",
	"Features_DataConversion",
	"Features_Security",
	// Cloud-first operator features that matter on day one of running
	// Temporal in production. Split out of Operations_TemporalCloud so
	// they get their own coverage each run instead of being averaged into
	// a single "cloud" bucket.
	"Features_Export",
	"Features_Observability",
	"Features_HighAvailability",
}

// IsPriorityCategory reports whether the given category is in PriorityCategories.
func IsPriorityCategory(cat string) bool {
	return slices.Contains(PriorityCategories, cat)
}

type Choice struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

type QuizQuestion struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Difficulty  string   `json:"difficulty"`
	Question    string   `json:"question"`
	Choices     []Choice `json:"choices"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	SourceDoc   string   `json:"source_doc"`
	GeneratedAt string   `json:"generated_at"`
}

type CategoryQuiz struct {
	Category  string         `json:"category"`
	Questions []QuizQuestion `json:"questions"`
}

type ManifestEntry struct {
	Category       string `json:"category"`
	QuestionCount  int    `json:"question_count"`
	EasyCount      int    `json:"easy_count"`
	MedCount       int    `json:"med_count"`
	HardCount      int    `json:"hard_count"`
	NightmareCount int    `json:"nightmare_count"`
}

type Manifest struct {
	GeneratedAt string          `json:"generated_at"`
	Categories  []ManifestEntry `json:"categories"`
	TotalCount  int             `json:"total_count"`
}

// QuizGenParams are the user-facing parameters for quiz generation.
// Safe to pass as workflow args (no secrets).
type QuizGenParams struct {
	EasyCount      int      // easy questions per category (default 3, max 30)
	MedCount       int      // med questions per category (default 4, max 30)
	HardCount      int      // hard questions per category (default 4, max 30)
	NightmareCount int      // nightmare questions per category (default 2, max 30)
	Categories     []string // filter to specific categories (empty = all)
	SkipEval       bool     // skip AI quality evaluation
}

type GenerateQuizInput struct {
	BucketPath     string
	Category       string
	EasyCount      int
	MedCount       int
	HardCount      int
	NightmareCount int
}

// CategoryPipelineInput drives the per-category child workflow that runs
// GenerateQuiz followed immediately by EvaluateQuiz batches for just that
// category, so evaluation for finished categories overlaps with still-running
// generation elsewhere.
type CategoryPipelineInput struct {
	Bucket   GenerateQuizInput
	SkipEval bool
}

// CategoryPipelineResult is what each CategoryPipelineWorkflow child returns.
// PreEval is the raw generated set, kept so the parent can recover priority
// categories that lose everything to the eval filter. Passed is the post-eval
// filtered set (or the raw set when SkipEval is true).
type CategoryPipelineResult struct {
	Category    string
	PreEval     []QuizQuestion
	Passed      []QuizQuestion
	PassedCount int
	FailedCount int
}

type claudeRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    []claudeSystemBlock `json:"system,omitempty"`
	Messages  []claudeMessage     `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeSystemBlock holds one entry of the system prompt array. When
// CacheControl is set to {Type: "ephemeral"}, Anthropic caches the block
// (and everything before it) for ~5 minutes at ~10% of the normal input
// rate. We use a single cacheable block for the stable generation /
// evaluation instructions; the variable user message is left uncached.
type claudeSystemBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text"`
	CacheControl *claudeCacheControl `json:"cache_control,omitempty"`
}

type claudeCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type rawQuestion struct {
	Question    string   `json:"question"`
	Choices     []Choice `json:"choices"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	SourceDoc   string   `json:"source_doc"`
}

type EvalScores struct {
	Clarity       int `json:"clarity"`
	Accuracy      int `json:"accuracy"`
	DifficultyFit int `json:"difficulty_fit"`
	Explanation   int `json:"explanation"`
	Relevance     int `json:"relevance"`
}

type EvalResult struct {
	QuestionID string     `json:"question_id"`
	Scores     EvalScores `json:"scores"`
	Pass       bool       `json:"pass"`
	Feedback   string     `json:"feedback"`
}

type EvalOutput struct {
	Passed  []QuizQuestion `json:"passed"`
	Failed  []QuizQuestion `json:"failed"`
	Results []EvalResult   `json:"results"`
}
