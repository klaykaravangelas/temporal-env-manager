package ec2activities

import (
	"context"
	"fmt"
	"os/exec"
)

func EC2TerraformDestroy(ctx context.Context) error {
	dir := getTerraformDir()

	destroy := exec.CommandContext(ctx, "terraform", "destroy", "-auto-approve")
	destroy.Dir = dir
	if out, err := destroy.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform destroy failed: %s\n%s", err, out)
	}

	return nil
}
