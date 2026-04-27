package quiz

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type QuizWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

// scraperWorkflowStub is a placeholder so the test environment can resolve
// the "ScraperWorkflow" type for child-workflow mocking.
func scraperWorkflowStub(ctx workflow.Context, startURL string) (int, error) {
	return 0, nil
}

func (s *QuizWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterActivity(&QuizActivities{})
	s.env.RegisterWorkflowWithOptions(scraperWorkflowStub, workflow.RegisterOptions{Name: "ScraperWorkflow"})
	s.env.RegisterWorkflowWithOptions(QuizGeneratorWorkflow, workflow.RegisterOptions{Name: "QuizGeneratorWorkflow"})
	s.env.RegisterWorkflowWithOptions(CategoryPipelineWorkflow, workflow.RegisterOptions{Name: "CategoryPipelineWorkflow"})
}

func (s *QuizWorkflowTestSuite) TearDownTest() {
	s.env.AssertExpectations(s.T())
}

func TestQuizWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(QuizWorkflowTestSuite))
}

func (s *QuizWorkflowTestSuite) TestQuizGeneratorWorkflow_Success() {
	buckets := []GenerateQuizInput{
		{BucketPath: "/tmp/a.txt", Category: "Features_Workflows"},
		{BucketPath: "/tmp/b.txt", Category: "Develop_Go"},
	}

	s.env.OnActivity("ListBuckets", mock.Anything).Return(buckets, nil).Once()

	// Each category child workflow runs its own GenerateQuiz + EvaluateQuiz
	// batches; SkipEval is true here so no eval activity is exercised.
	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		[]QuizQuestion{{ID: "q1", Category: "Features_Workflows", Difficulty: "hard"}}, nil,
	).Once()
	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		[]QuizQuestion{{ID: "q2", Category: "Develop_Go", Difficulty: "hard"}}, nil,
	).Once()

	// Reference validation runs after all generation; mock empty allowlist
	// so the validate/fix loop is a no-op for these synthetic questions.
	s.env.OnActivity("FetchDocsURLAllowlist", mock.Anything).Return([]string{}, nil).Once()

	s.env.OnActivity("WriteQuizFiles", mock.Anything, mock.Anything).Return(nil).Once()

	s.env.ExecuteWorkflow(QuizGeneratorWorkflow, QuizGenParams{
		EasyCount: 3, MedCount: 4, HardCount: 4, NightmareCount: 2,
		SkipEval: true,
	})
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result int
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	require.Equal(s.T(), 2, result)
}

func (s *QuizWorkflowTestSuite) TestQuizGeneratorWorkflow_PartialFailure() {
	buckets := []GenerateQuizInput{
		{BucketPath: "/tmp/a.txt", Category: "Features_Workflows"},
	}

	s.env.OnActivity("ListBuckets", mock.Anything).Return(buckets, nil).Once()
	// GenerateQuiz fails inside the child workflow → child workflow surfaces
	// the error → parent logs and moves on with zero questions.
	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("rate limited"),
	).Times(2) // child retry policy = 2 attempts

	s.env.ExecuteWorkflow(QuizGeneratorWorkflow, QuizGenParams{
		EasyCount: 3, MedCount: 4, HardCount: 4, NightmareCount: 2,
		SkipEval: true,
	})
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result int
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	require.Equal(s.T(), 0, result)
}

func (s *QuizWorkflowTestSuite) TestQuizGeneratorWorkflow_WithEval() {
	buckets := []GenerateQuizInput{
		{BucketPath: "/tmp/a.txt", Category: "Features_Workflows"},
	}

	s.env.OnActivity("ListBuckets", mock.Anything).Return(buckets, nil).Once()
	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		[]QuizQuestion{
			{ID: "q1", Category: "Features_Workflows", Difficulty: "hard"},
			{ID: "q2", Category: "Features_Workflows", Difficulty: "easy"},
		}, nil,
	).Once()
	// EvaluateQuiz fans out batches inside the child workflow (one batch here
	// since only 2 questions < EvalBatchSize).
	s.env.OnActivity("EvaluateQuiz", mock.Anything, mock.Anything).Return(
		EvalOutput{
			Passed:  []QuizQuestion{{ID: "q1", Category: "Features_Workflows", Difficulty: "hard"}},
			Failed:  []QuizQuestion{{ID: "q2", Category: "Features_Workflows", Difficulty: "easy"}},
			Results: []EvalResult{{QuestionID: "q1", Pass: true}, {QuestionID: "q2", Pass: false}},
		}, nil,
	).Once()
	s.env.OnActivity("FetchDocsURLAllowlist", mock.Anything).Return([]string{}, nil).Once()
	s.env.OnActivity("WriteQuizFiles", mock.Anything, mock.Anything).Return(nil).Once()

	s.env.ExecuteWorkflow(QuizGeneratorWorkflow, QuizGenParams{
		EasyCount: 3, MedCount: 4, HardCount: 4, NightmareCount: 2,
	})
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result int
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	// q1 passes, q2 fails — priority-category recovery only fires when the
	// category loses ALL its questions, so only q1 survives.
	require.Equal(s.T(), 1, result)
}

func (s *QuizWorkflowTestSuite) TestQuizGeneratorWorkflow_FixesBrokenReferences() {
	buckets := []GenerateQuizInput{
		{BucketPath: "/tmp/a.txt", Category: "Features_Workflows"},
	}
	s.env.OnActivity("ListBuckets", mock.Anything).Return(buckets, nil).Once()

	// Two questions: one with a valid ref, one with a broken ref.
	q1 := QuizQuestion{ID: "q1", Category: "Features_Workflows", Difficulty: "hard",
		SourceDoc: "develop_go_workflows.html",
		Reference: "https://docs.temporal.io/develop/go/workflows"}
	q2 := QuizQuestion{ID: "q2", Category: "Features_Workflows", Difficulty: "hard",
		SourceDoc: "develop_go_workflows.html",
		Reference: "https://docs.temporal.io/develop/go/workflowz"}
	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		[]QuizQuestion{q1, q2}, nil,
	).Once()

	allowlist := []string{"https://docs.temporal.io/develop/go/workflows"}
	s.env.OnActivity("FetchDocsURLAllowlist", mock.Anything).Return(allowlist, nil).Once()
	s.env.OnActivity("ValidateReferences", mock.Anything, mock.Anything).Return(
		ValidateReferencesOutput{
			Valid:   []QuizQuestion{q1},
			Invalid: []QuizQuestion{q2},
		}, nil,
	).Once()
	// Fixer picks the only valid candidate for q2.
	s.env.OnActivity("FixReference", mock.Anything, mock.Anything).Return(
		FixReferenceOutput{FixedReference: "https://docs.temporal.io/develop/go/workflows"}, nil,
	).Once()

	var written []QuizQuestion
	s.env.OnActivity("WriteQuizFiles", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		written = args.Get(1).([]QuizQuestion)
	}).Return(nil).Once()

	s.env.ExecuteWorkflow(QuizGeneratorWorkflow, QuizGenParams{
		EasyCount: 3, MedCount: 4, HardCount: 4, NightmareCount: 2,
		SkipEval: true,
	})
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	require.Len(s.T(), written, 2)
	for _, q := range written {
		require.Equal(s.T(), "https://docs.temporal.io/develop/go/workflows", q.Reference,
			"both questions should end up with the only valid URL")
	}
}

func (s *QuizWorkflowTestSuite) TestDailyPipelineWorkflow_Success() {
	s.env.OnWorkflow("ScraperWorkflow", mock.Anything, mock.Anything).Return(0, nil).Once()
	s.env.OnWorkflow("QuizGeneratorWorkflow", mock.Anything, mock.Anything).Return(100, nil).Once()

	s.env.ExecuteWorkflow(DailyPipelineWorkflow, QuizGenParams{EasyCount: 3, MedCount: 4, HardCount: 4, NightmareCount: 2})
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result string
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	require.Contains(s.T(), result, "100 questions")
}
