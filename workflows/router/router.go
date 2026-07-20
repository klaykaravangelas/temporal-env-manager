package router

import (
	"fmt"
	"time"

	ec2workflow "github.com/klaykaravangelas/temporal-env-manager/workflows/ec2"
	s3lambdaworkflow "github.com/klaykaravangelas/temporal-env-manager/workflows/s3lambda"
	enumspb "go.temporal.io/api/enums/v1"

	"go.temporal.io/sdk/workflow"
)

type EnvironmentRequest struct {
	Type string
	TTL  *time.Duration // nil means no TTL
}

func RouterWorkflow(ctx workflow.Context, req EnvironmentRequest) (string, error) {
	childID := fmt.Sprintf("%s-%d", req.Type, workflow.Now(ctx).UnixMilli())

	childOpts := workflow.ChildWorkflowOptions{
		WorkflowID:        childID,
		ParentClosePolicy: enumspb.PARENT_CLOSE_POLICY_ABANDON,
	}
	childCtx := workflow.WithChildOptions(ctx, childOpts)

	switch req.Type {
	case "ec2":
		cfg := ec2workflow.EnvironmentConfig{TTL: req.TTL}
		err := workflow.ExecuteChildWorkflow(childCtx, ec2workflow.EC2EnvironmentWorkflow, cfg).GetChildWorkflowExecution().Get(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed to start ec2 workflow: %w", err)
		}
	case "s3lambda":
		cfg := s3lambdaworkflow.EnvironmentConfig{TTL: req.TTL}
		err := workflow.ExecuteChildWorkflow(childCtx, s3lambdaworkflow.S3LambdaEnvironmentWorkflow, cfg).GetChildWorkflowExecution().Get(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed to start s3lambda workflow: %w", err)
		}
	default:
		return "", fmt.Errorf("unknown environment type: %s", req.Type)
	}

	return childID, nil
}
