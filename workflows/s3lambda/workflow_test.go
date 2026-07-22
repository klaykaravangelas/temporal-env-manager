package s3lambdaworkflow

import (
	"fmt"
	"testing"
	"time"

	s3lambdaactivities "github.com/klaykaravangelas/temporal-env-manager/activities/s3lambda"

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

	env.OnActivity(s3lambdaactivities.S3LambdaTerraformApply, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(
		&s3lambdaactivities.TerraformApplyResult{BucketName: "my-bucket", LambdaArn: "arn:aws:lambda:us-east-1:123:function:my-fn"}, nil,
	)
	env.OnActivity(s3lambdaactivities.S3LambdaTerraformDestroy, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(nil)

	env.ExecuteWorkflow(S3LambdaEnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_Compensation_OnApplyFailure() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(s3lambdaactivities.S3LambdaTerraformApply, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(
		nil, fmt.Errorf("apply failed"),
	)

	env.ExecuteWorkflow(S3LambdaEnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_ExtendSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(s3lambdaactivities.S3LambdaTerraformApply, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(
		&s3lambdaactivities.TerraformApplyResult{BucketName: "my-bucket", LambdaArn: "arn:aws:lambda:us-east-1:123:function:my-fn"}, nil,
	)
	env.OnActivity(s3lambdaactivities.S3LambdaTerraformDestroy, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("extend-ttl", 10*time.Minute)
	}, 1*time.Minute)

	env.ExecuteWorkflow(S3LambdaEnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_TeardownSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(s3lambdaactivities.S3LambdaTerraformApply, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(
		&s3lambdaactivities.TerraformApplyResult{BucketName: "my-bucket", LambdaArn: "arn:aws:lambda:us-east-1:123:function:my-fn"}, nil,
	)
	env.OnActivity(s3lambdaactivities.S3LambdaTerraformDestroy, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("teardown", nil)
	}, 1*time.Minute)

	env.ExecuteWorkflow(S3LambdaEnvironmentWorkflow, EnvironmentConfig{TTL: ttlPtr(5 * time.Minute)})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) Test_NoTTL_WaitsForTeardownSignal() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(s3lambdaactivities.S3LambdaTerraformApply, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(
		&s3lambdaactivities.TerraformApplyResult{BucketName: "my-bucket", LambdaArn: "arn:aws:lambda:us-east-1:123:function:my-fn"}, nil,
	)
	env.OnActivity(s3lambdaactivities.S3LambdaTerraformDestroy, mock.Anything, s3lambdaactivities.TerraformVars{}).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("teardown", nil)
	}, 1*time.Hour)

	env.ExecuteWorkflow(S3LambdaEnvironmentWorkflow, EnvironmentConfig{TTL: nil})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}
