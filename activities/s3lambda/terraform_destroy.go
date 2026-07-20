package s3lambdaactivities

import (
	"context"
	"fmt"
	"os/exec"
)

func S3LambdaTerraformDestroy(ctx context.Context) error {
	dir := getTerraformDir()

	destroy := exec.CommandContext(ctx, "terraform", "destroy", "-auto-approve")
	destroy.Dir = dir
	if out, err := destroy.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform destroy failed: %s\n%s", err, out)
	}

	return nil
}
