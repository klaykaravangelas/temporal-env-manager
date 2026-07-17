package workflow

import (
	"time"

	"github.com/klaykaravangelas/temporal-env-manager/activities"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type EnvironmentConfig struct {
	TTL time.Duration
}

func EnvironmentWorkflow(ctx workflow.Context, cfg EnvironmentConfig) error {
	// Internal state
	currentStep := "initializing"
	instanceID := ""

	// Query handler — returns the current step and instance ID
	err := workflow.SetQueryHandler(ctx, "status", func() (map[string]string, error) {
		return map[string]string{
			"step":       currentStep,
			"instanceId": instanceID,
		}, nil
	})
	if err != nil {
		return err
	}

	// Signal channels
	extendCh := workflow.GetSignalChannel(ctx, "extend-ttl")
	teardownCh := workflow.GetSignalChannel(ctx, "teardown")

	// Activity options
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
	var applyResult activities.TerraformApplyResult
	err = workflow.ExecuteActivity(terraformCtx, activities.TerraformApply).Get(ctx, &applyResult)
	if err != nil {
		return err
	}
	instanceID = applyResult.InstanceID

	// Step 2: Wait for instance to be healthy
	currentStep = "waiting-for-instance"
	waitCtx := workflow.WithActivityOptions(ctx, waitOpts)
	err = workflow.ExecuteActivity(waitCtx, activities.WaitForInstance, applyResult.InstanceID).Get(ctx, nil)
	if err != nil {
		_ = workflow.ExecuteActivity(terraformCtx, activities.TerraformDestroy).Get(ctx, nil)
		return err
	}

	// Step 3: Run setup commands
	currentStep = "running-setup"
	ssmCtx := workflow.WithActivityOptions(ctx, ssmOpts)
	err = workflow.ExecuteActivity(ssmCtx, activities.RunSetupCommands, applyResult.InstanceID).Get(ctx, nil)
	if err != nil {
		_ = workflow.ExecuteActivity(terraformCtx, activities.TerraformDestroy).Get(ctx, nil)
		return err
	}

	// Step 4: Sleep for TTL, but listen for signals
	currentStep = "sleeping"
	ttlRemaining := cfg.TTL
	timerDone := false

	for !timerDone {
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		timer := workflow.NewTimer(timerCtx, ttlRemaining)

		// Wait for either: timer expires, extend signal, or teardown signal
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

	// Step 5: Terraform Destroy
	currentStep = "destroying"
	cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
	cleanupCtx = workflow.WithActivityOptions(cleanupCtx, terraformOpts)
	err = workflow.ExecuteActivity(cleanupCtx, activities.TerraformDestroy).Get(cleanupCtx, nil)
	if err != nil {
		return err
	}

	currentStep = "completed"
	return nil
}
