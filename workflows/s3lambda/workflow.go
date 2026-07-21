package s3lambdaworkflow

import (
	"time"

	s3lambdaactivities "github.com/klaykaravangelas/temporal-env-manager/activities/s3lambda"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type EnvironmentConfig struct {
	TTL  *time.Duration
	Vars s3lambdaactivities.TerraformVars
}

func S3LambdaEnvironmentWorkflow(ctx workflow.Context, cfg EnvironmentConfig) error {
	currentStep := "initializing"
	bucketName := ""
	lambdaFunctionName := ""
	lambdaArn := ""

	err := workflow.SetQueryHandler(ctx, "status", func() (map[string]string, error) {
		return map[string]string{
			"step":               currentStep,
			"bucketName":         bucketName,
			"lambdaFunctionName": lambdaFunctionName,
			"lambdaArn":          lambdaArn,
		}, nil
	})
	if err != nil {
		return err
	}

	extendCh := workflow.GetSignalChannel(ctx, "extend-ttl")
	teardownCh := workflow.GetSignalChannel(ctx, "teardown")

	terraformOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	terraformCtx := workflow.WithActivityOptions(ctx, terraformOpts)

	// Step 1: Terraform Apply
	currentStep = "applying"
	var applyResult s3lambdaactivities.TerraformApplyResult
	err = workflow.ExecuteActivity(terraformCtx, s3lambdaactivities.S3LambdaTerraformApply, cfg.Vars).Get(ctx, &applyResult)
	if err != nil {
		return err
	}
	bucketName = applyResult.BucketName
	lambdaFunctionName = applyResult.LambdaFunctionName
	lambdaArn = applyResult.LambdaArn

	// Step 2: Sleep for TTL or wait for teardown signal
	currentStep = "sleeping"

	if cfg.TTL != nil {
		ttlRemaining := *cfg.TTL
		timerDone := false

		for !timerDone {
			timerCtx, cancelTimer := workflow.WithCancel(ctx)
			timer := workflow.NewTimer(timerCtx, ttlRemaining)

			sel := workflow.NewSelector(ctx)

			sel.AddFuture(timer, func(f workflow.Future) {
				timerDone = true
			})

			sel.AddReceive(extendCh, func(ch workflow.ReceiveChannel, more bool) {
				var extension time.Duration
				ch.Receive(ctx, &extension)
				ttlRemaining += extension
				cancelTimer()
			})

			sel.AddReceive(teardownCh, func(ch workflow.ReceiveChannel, more bool) {
				ch.Receive(ctx, nil)
				timerDone = true
				cancelTimer()
			})

			sel.Select(ctx)
		}
	} else {
		teardownCh.Receive(ctx, nil)
	}

	// Step 3: Terraform Destroy
	currentStep = "destroying"
	cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
	cleanupCtx = workflow.WithActivityOptions(cleanupCtx, terraformOpts)
	err = workflow.ExecuteActivity(cleanupCtx, s3lambdaactivities.S3LambdaTerraformDestroy, cfg.Vars).Get(cleanupCtx, nil)
	if err != nil {
		return err
	}

	currentStep = "completed"
	return nil
}
