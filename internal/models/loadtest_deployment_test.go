package models

import "testing"

func TestLoadTestDeploymentOptions_TerraformVars(t *testing.T) {
	opts := &LoadTestDeploymentOptions{ProjectName: "demo", Region: "us-east-1", BucketName: "bucket123", Provider: "aws", CPUUnits: 256, MemoryUnits: 512, WorkerDesiredCount: 3}
	vars := opts.CreateTerraformVars()
	checks := []string{"project_name         = \"demo\"", "aws_region           = \"us-east-1\"", "existing_bucket_name = \"bucket123\"", "cpu_units            = 256", "memory_units         = 512", "worker_desired_count = 3"}
	for _, c := range checks {
		if !containsLine(vars, c) {
			t.Fatalf("expected tfvars to contain line: %s\nGot:\n%s", c, vars)
		}
	}
}

func containsLine(s, line string) bool {
	for _, l := range splitLines(s) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
