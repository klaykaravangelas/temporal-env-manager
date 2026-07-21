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

type TerraformVars struct {
	Region             string
	BucketName         string
	LambdaFunctionName string
}

type TerraformApplyResult struct {
	BucketName         string
	LambdaFunctionName string
	LambdaArn          string
}

func S3LambdaTerraformApply(ctx context.Context, vars TerraformVars) (*TerraformApplyResult, error) {
	dir := getTerraformDir()

	init := exec.CommandContext(ctx, "terraform", "init")
	init.Dir = dir
	if out, err := init.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform init failed: %s\n%s", err, out)
	}

	args := []string{"apply", "-auto-approve"}
	if vars.Region != "" {
		args = append(args, "-var", "region="+vars.Region)
	}
	if vars.BucketName != "" {
		args = append(args, "-var", "bucket_name="+vars.BucketName)
	}
	if vars.LambdaFunctionName != "" {
		args = append(args, "-var", "lambda_function_name="+vars.LambdaFunctionName)
	}
	apply := exec.CommandContext(ctx, "terraform", args...)
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
		BucketName:         outputs["bucket_name"].Value,
		LambdaFunctionName: outputs["lambda_function_name"].Value,
		LambdaArn:          outputs["lambda_arn"].Value,
	}, nil
}
