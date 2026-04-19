package quiz

const (
	TaskQueue              = "scraper-task-queue"
	DefaultModel           = "claude-sonnet-4-6"
	DefaultQuestionsPerCat = 10
	DefaultOutputDir       = "web/quizzes"
	BucketDir              = "temporal_docs_txt"
)

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

type claudeRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	Messages    []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
