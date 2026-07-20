package s3lambdaactivities

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

func getTerraformDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "terraform", "s3lambda")
}

type TerraformOutput struct {
	Value string `json:"value"`
}

type TerraformApplyResult struct {
	BucketName string
	LambdaArn  string
}

func S3LambdaTerraformApply(ctx context.Context) (*TerraformApplyResult, error) {
	dir := getTerraformDir()

	init := exec.CommandContext(ctx, "terraform", "init")
	init.Dir = dir
	if out, err := init.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform init failed: %s\n%s", err, out)
	}

	apply := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	apply.Dir = dir
	if out, err := apply.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform apply failed: %s\n%s", err, out)
	}

	output := exec.CommandContext(ctx, "terraform", "output", "-json")
	output.Dir = dir
	out, err := output.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform output failed: %s", err)
	}

	var outputs map[string]TerraformOutput
	if err := json.Unmarshal(out, &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse terraform output: %s", err)
	}

	return &TerraformApplyResult{
		BucketName: outputs["bucket_name"].Value,
		LambdaArn:  outputs["lambda_arn"].Value,
	}, nil
}
