package ec2activities

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
	return filepath.Join(filepath.Dir(filename), "..", "..", "terraform", "ec2")
}

type TerraformOutput struct {
	Value string `json:"value"`
}

type TerraformVars struct {
	Region       string
	InstanceType string
}

type TerraformApplyResult struct {
	InstanceID string
	VpcID      string
}

func EC2TerraformApply(ctx context.Context, vars TerraformVars) (*TerraformApplyResult, error) {
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
	if vars.InstanceType != "" {
		args = append(args, "-var", "instance_type="+vars.InstanceType)
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
		InstanceID: outputs["instance_id"].Value,
		VpcID:      outputs["vpc_id"].Value,
	}, nil
}
