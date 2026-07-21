package s3lambdaactivities

import (
	"context"
	"fmt"
	"os/exec"
)

func S3LambdaTerraformDestroy(ctx context.Context, vars TerraformVars) error {
	dir := getTerraformDir()

	args := []string{"destroy", "-auto-approve"}
	if vars.Region != "" {
		args = append(args, "-var", "region="+vars.Region)
	}
	if vars.BucketName != "" {
		args = append(args, "-var", "bucket_name="+vars.BucketName)
	}
	if vars.LambdaFunctionName != "" {
		args = append(args, "-var", "lambda_function_name="+vars.LambdaFunctionName)
	}
	destroy := exec.CommandContext(ctx, "terraform", args...)
	destroy.Dir = dir
	if out, err := destroy.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform destroy failed: %s\n%s", err, out)
	}

	return nil
}
