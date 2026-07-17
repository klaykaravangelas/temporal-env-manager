package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// getTerraformDir returns the absolute path to the terraform/ directory.
func getTerraformDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "terraform")
}

// TerraformOutput represents a single terraform output value.
type TerraformOutput struct {
	Value string `json:"value"`
}

// TerraformApplyResult holds the outputs we care about.
type TerraformApplyResult struct {
	InstanceID string
	VpcID      string
}

func TerraformApply(ctx context.Context) (*TerraformApplyResult, error) {
	dir := getTerraformDir()

	// terraform init
	init := exec.CommandContext(ctx, "terraform", "init")
	init.Dir = dir
	if out, err := init.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform init failed: %s\n%s", err, out)
	}

	// terraform apply
	apply := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	apply.Dir = dir
	if out, err := apply.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform apply failed: %s\n%s", err, out)
	}

	// terraform output -json
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
		InstanceID: outputs["instance_id"].Value,
		VpcID:      outputs["vpc_id"].Value,
	}, nil
}
