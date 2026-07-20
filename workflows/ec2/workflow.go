package ec2workflow

import (
	"time"

	ec2activities "github.com/klaykaravangelas/temporal-env-manager/activities/ec2"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type EnvironmentConfig struct {
	TTL *time.Duration
}

func EC2EnvironmentWorkflow(ctx workflow.Context, cfg EnvironmentConfig) error {
	currentStep := "initializing"
	instanceID := ""

	err := workflow.SetQueryHandler(ctx, "status", func() (map[string]string, error) {
		return map[string]string{
			"step":       currentStep,
			"instanceId": instanceID,
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

	waitOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}

	ssmOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}

	// Step 1: Terraform Apply
	currentStep = "applying"
	terraformCtx := workflow.WithActivityOptions(ctx, terraformOpts)
	var applyResult ec2activities.TerraformApplyResult
	err = workflow.ExecuteActivity(terraformCtx, ec2activities.EC2TerraformApply).Get(ctx, &applyResult)
	if err != nil {
		return err
	}
	instanceID = applyResult.InstanceID

	// Step 2: Wait for instance to be healthy
	currentStep = "waiting-for-instance"
	waitCtx := workflow.WithActivityOptions(ctx, waitOpts)
	err = workflow.ExecuteActivity(waitCtx, ec2activities.EC2WaitForInstance, applyResult.InstanceID).Get(ctx, nil)
	if err != nil {
		_ = workflow.ExecuteActivity(terraformCtx, ec2activities.EC2TerraformDestroy).Get(ctx, nil)
		return err
	}

	// Step 3: Run setup commands
	currentStep = "running-setup"
	ssmCtx := workflow.WithActivityOptions(ctx, ssmOpts)
	err = workflow.ExecuteActivity(ssmCtx, ec2activities.EC2RunSetupCommands, applyResult.InstanceID).Get(ctx, nil)
	if err != nil {
		_ = workflow.ExecuteActivity(terraformCtx, ec2activities.EC2TerraformDestroy).Get(ctx, nil)
		return err
	}

	// Step 4: Sleep for TTL or wait for teardown signal
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

	// Step 5: Terraform Destroy
	currentStep = "destroying"
	cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
	cleanupCtx = workflow.WithActivityOptions(cleanupCtx, terraformOpts)
	err = workflow.ExecuteActivity(cleanupCtx, ec2activities.EC2TerraformDestroy).Get(cleanupCtx, nil)
	if err != nil {
		return err
	}

	currentStep = "completed"
	return nil
}
