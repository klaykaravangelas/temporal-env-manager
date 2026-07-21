package ec2activities

import (
	"context"
	"fmt"
	"os/exec"
)

func EC2TerraformDestroy(ctx context.Context, vars TerraformVars) error {
	dir := getTerraformDir()

	args := []string{"destroy", "-auto-approve"}
	if vars.Region != "" {
		args = append(args, "-var", "region="+vars.Region)
	}
	if vars.InstanceType != "" {
		args = append(args, "-var", "instance_type="+vars.InstanceType)
	}
	destroy := exec.CommandContext(ctx, "terraform", args...)
	destroy.Dir = dir
	if out, err := destroy.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform destroy failed: %s\n%s", err, out)
	}

	return nil
}
