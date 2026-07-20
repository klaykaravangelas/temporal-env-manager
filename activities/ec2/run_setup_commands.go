package ec2activities

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func EC2RunSetupCommands(ctx context.Context, instanceID string) error {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ssm.NewFromConfig(cfg)

	sendOut, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		InstanceIds:  []string{instanceID},
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]string{
			"commands": {"echo 'Hello from Temporal! Environment setup complete.'"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	commandID := *sendOut.Command.CommandId

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		invOut, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
			CommandId:  &commandID,
			InstanceId: &instanceID,
		})
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		switch invOut.Status {
		case types.CommandInvocationStatusSuccess:
			return nil
		case types.CommandInvocationStatusFailed,
			types.CommandInvocationStatusCancelled,
			types.CommandInvocationStatusTimedOut:
			return fmt.Errorf("command failed with status: %s", invOut.Status)
		}

		time.Sleep(2 * time.Second)
	}
}
