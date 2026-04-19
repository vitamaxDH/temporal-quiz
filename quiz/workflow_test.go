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

	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		[]QuizQuestion{{ID: "q1", Category: "Features_Workflows", Difficulty: "hard"}}, nil,
	).Once()
	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		[]QuizQuestion{{ID: "q2", Category: "Develop_Go", Difficulty: "hard"}}, nil,
	).Once()

	s.env.OnActivity("WriteQuizFiles", mock.Anything, mock.Anything).Return(nil).Once()

	s.env.ExecuteWorkflow(QuizGeneratorWorkflow, QuizGenParams{EasyCount: 3, MedCount: 4, HardCount: 4, NightmareCount: 2})
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
	s.env.OnActivity("GenerateQuiz", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("rate limited"),
	).Once()

	s.env.ExecuteWorkflow(QuizGeneratorWorkflow, QuizGenParams{EasyCount: 3, MedCount: 4, HardCount: 4, NightmareCount: 2})
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var result int
	require.NoError(s.T(), s.env.GetWorkflowResult(&result))
	require.Equal(s.T(), 0, result)
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
