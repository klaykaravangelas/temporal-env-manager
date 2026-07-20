package ec2workflow

import (
	"fmt"
	"testing"
	"time"

	ec2activities "github.com/klaykaravangelas/temporal-env-manager/activities/ec2"

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

func ttlPtr(d time.Duration) *time.Duration {
	return &d
}

func (s *WorkflowTestSuite) Test_HappyPath() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(ec2activities.EC2TerraformApply, mock.Anything).Return(
		&ec2activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(ec2activities.EC2WaitForInstance, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2RunSetupCommands, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2TerraformDestroy, mock.Anything).Return(nil)

	env.ExecuteWorkflow(EC2EnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_Compensation_OnWaitForInstanceFailure() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(ec2activities.EC2TerraformApply, mock.Anything).Return(
		&ec2activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(ec2activities.EC2WaitForInstance, mock.Anything, "i-abc123").Return(
		fmt.Errorf("instance never became healthy"),
	)
	env.OnActivity(ec2activities.EC2TerraformDestroy, mock.Anything).Return(nil)

	env.ExecuteWorkflow(EC2EnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_ExtendSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(ec2activities.EC2TerraformApply, mock.Anything).Return(
		&ec2activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(ec2activities.EC2WaitForInstance, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2RunSetupCommands, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2TerraformDestroy, mock.Anything).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("extend-ttl", 10*time.Minute)
	}, 1*time.Minute)

	env.ExecuteWorkflow(EC2EnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_TeardownSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(ec2activities.EC2TerraformApply, mock.Anything).Return(
		&ec2activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(ec2activities.EC2WaitForInstance, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2RunSetupCommands, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2TerraformDestroy, mock.Anything).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("teardown", nil)
	}, 1*time.Minute)

	env.ExecuteWorkflow(EC2EnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_NoTTL_WaitsForTeardownSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(ec2activities.EC2TerraformApply, mock.Anything).Return(
		&ec2activities.TerraformApplyResult{InstanceID: "i-abc123", VpcID: "vpc-xyz"}, nil,
	)
	env.OnActivity(ec2activities.EC2WaitForInstance, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2RunSetupCommands, mock.Anything, "i-abc123").Return(nil)
	env.OnActivity(ec2activities.EC2TerraformDestroy, mock.Anything).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("teardown", nil)
	}, 1*time.Hour)

	env.ExecuteWorkflow(EC2EnvironmentWorkflow, EnvironmentConfig{TTL: nil})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}
