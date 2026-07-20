package ec2activities

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func EC2WaitForInstance(ctx context.Context, instanceID string) error {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		out, err := client.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
			InstanceIds: []string{instanceID},
		})
		if err != nil {
			return fmt.Errorf("failed to describe instance: %w", err)
		}

		if len(out.InstanceStatuses) > 0 {
			status := out.InstanceStatuses[0]
			if status.InstanceState.Name == types.InstanceStateNameRunning &&
				status.InstanceStatus.Status == types.SummaryStatusOk &&
				status.SystemStatus.Status == types.SummaryStatusOk {
				return nil
			}
		}

		time.Sleep(10 * time.Second)
	}
}
