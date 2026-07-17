package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/klaykaravangelas/temporal-env-manager/activities"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type WorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestWorkflowSuite(t *testing.T) {
	suite.Run(t, new(WorkflowTestSuite))
}

func (s *WorkflowTestSuite) Test_HappyPath() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(activities.TerraformApply, mock.Anything).Return(
		&activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(activities.WaitForInstance, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(activities.RunSetupCommands, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(activities.TerraformDestroy, mock.Anything).Return(nil)

	env.ExecuteWorkflow(EnvironmentWorkflow, EnvironmentConfig{TTL: 5 * time.Minute})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_Compensation_OnWaitForInstanceFailure() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(activities.TerraformApply, mock.Anything).Return(
		&activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(activities.WaitForInstance, mock.Anything, "i-abc123").Return(
		fmt.Errorf("instance never became healthy"),
	)
	env.OnActivity(activities.TerraformDestroy, mock.Anything).Return(nil)

	env.ExecuteWorkflow(EnvironmentWorkflow, EnvironmentConfig{TTL: 5 * time.Minute})

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_ExtendSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(activities.TerraformApply, mock.Anything).Return(
		&activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(activities.WaitForInstance, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(activities.RunSetupCommands, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(activities.TerraformDestroy, mock.Anything).Return(nil)

	// Send extend signal 1 minute into the sleep
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("extend-ttl", 10*time.Minute)
	}, 1*time.Minute)

	env.ExecuteWorkflow(EnvironmentWorkflow, EnvironmentConfig{TTL: 5 * time.Minute})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_TeardownSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(activities.TerraformApply, mock.Anything).Return(
		&activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(activities.WaitForInstance, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(activities.RunSetupCommands, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(activities.TerraformDestroy, mock.Anything).Return(nil)

	// Send teardown signal 1 minute into the sleep
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("teardown", nil)
	}, 1*time.Minute)

	env.ExecuteWorkflow(EnvironmentWorkflow, EnvironmentConfig{TTL: 5 * time.Minute})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}
