package scraper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type WorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *WorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterActivity(&Activities{})
}

func (s *WorkflowTestSuite) TearDownTest() {
	s.env.AssertExpectations(s.T())
}

func TestWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowTestSuite))
}

func (s *WorkflowTestSuite) TestScraperWorkflow_Success() {
	s.env.OnActivity("FetchDocsRepo", mock.Anything).
		Return("/tmp/docs", nil).Once()

	s.env.OnActivity("ReadLocalDocs", mock.Anything, "/tmp/docs").
		Return("Success! Read 100 files into 15 buckets.", nil).Once()

	s.env.ExecuteWorkflow(ScraperWorkflow, "")
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())
}

func (s *WorkflowTestSuite) TestScraperWorkflow_FetchFails() {
	s.env.OnActivity("FetchDocsRepo", mock.Anything).
		Return("", fmt.Errorf("github returned status 500")).Times(3)

	s.env.ExecuteWorkflow(ScraperWorkflow, "")
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.Error(s.T(), s.env.GetWorkflowError())
}

func (s *WorkflowTestSuite) TestScraperWorkflow_ReadFails() {
	s.env.OnActivity("FetchDocsRepo", mock.Anything).
		Return("/tmp/docs", nil).Once()

	s.env.OnActivity("ReadLocalDocs", mock.Anything, "/tmp/docs").
		Return("", fmt.Errorf("walking docs dir: no such directory")).Times(3)

	s.env.ExecuteWorkflow(ScraperWorkflow, "")
	require.True(s.T(), s.env.IsWorkflowCompleted())
	require.Error(s.T(), s.env.GetWorkflowError())
}
