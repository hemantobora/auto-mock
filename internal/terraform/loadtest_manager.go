// internal/terraform/loadtest_manager.go
package terraform

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	core "github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/models"
)

// LoadTestManager handles Terraform operations for the Locust load-testing stack
type LoadTestManager struct {
	ProjectName  string
	Region       string
	TerraformDir string
	WorkingDir   string
	Provider     core.Provider
	Profile      string
	BucketName   string
}

// NewLoadTestManager creates a new manager for the loadtest stack
func NewLoadTestManager(cleanProject, profile string, provider core.Provider) (*LoadTestManager, error) {
	exists, err := provider.ProjectExists(context.Background(), cleanProject)
	if err != nil {
		return nil, fmt.Errorf("project existence check failed: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("project %s does not exist (init first)", cleanProject)
	}

	exe, _ := os.Executable()
	root := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	terraformDir := filepath.Join(root, "terraform", "loadtest")
	if _, serr := os.Stat(terraformDir); os.IsNotExist(serr) {
		terraformDir = filepath.Join("terraform", "loadtest") // fallback
	}
	workingDir := filepath.Join(osTempDir(), fmt.Sprintf("automock-lt-%s-%d", cleanProject, os.Getpid()))

	return &LoadTestManager{
		ProjectName:  cleanProject,
		TerraformDir: terraformDir,
		WorkingDir:   workingDir,
		Provider:     provider,
		Profile:      profile,
		Region:       provider.GetRegion(),
		BucketName:   provider.GetStorageName(),
	}, nil
}

// Deploy creates Locust infrastructure via Terraform and saves deployment metadata
func (m *LoadTestManager) Deploy(opts *models.LoadTestDeploymentOptions) (*models.LoadTestDeploymentOutputs, error) {
	if m.BucketName == "" {
		return nil, fmt.Errorf("no Storage bucket found for project '%s'. Please run 'automock init' first", m.ProjectName)
	}
	// Prepare workspace, backend, init, tfvars, plan, apply
	if err := m.prepareWorkspace(); err != nil {
		return nil, err
	}
	defer m.cleanup()

	if err := m.createBackendConfigWithKey("terraform/loadtest/state/terraform.tfstate"); err != nil {
		return nil, err
	}
	if err := m.initTerraform(); err != nil {
		return nil, err
	}

	// Fill defaults from provider/environment
	if opts == nil {
		opts = &models.LoadTestDeploymentOptions{}
	}
	if opts.ProjectName == "" {
		opts.ProjectName = m.ProjectName
	}
	if opts.Region == "" {
		opts.Region = m.Region
	}
	if opts.BucketName == "" {
		opts.BucketName = m.BucketName
	}
	if opts.Provider == "" {
		opts.Provider = m.Provider.GetProviderType()
	}
	if opts.CPUUnits == 0 {
		opts.CPUUnits = 256
	}
	if opts.MemoryUnits == 0 {
		opts.MemoryUnits = 512
	}

	if err := m.createLoadTestVars(opts); err != nil {
		return nil, err
	}
	if err := m.planTerraform(); err != nil {
		return nil, err
	}
	if err := m.applyTerraform(); err != nil {
		return nil, err
	}

	out, err := m.getLoadTestOutputs()
	if err != nil {
		return nil, err
	}
	if err := m.Provider.SaveLoadTestDeploymentMetadata(out); err != nil {
		return nil, fmt.Errorf("save loadtest metadata: %w", err)
	}
	return out, nil
}

// Destroy tears down Locust infrastructure and removes metadata
func (m *LoadTestManager) Destroy() error {
	if err := m.prepareWorkspace(); err != nil {
		return err
	}
	defer m.cleanup()
	if err := m.createBackendConfigWithKey("terraform/loadtest/state/terraform.tfstate"); err != nil {
		return err
	}
	if err := m.initTerraform(); err != nil {
		return err
	}
	if err := m.destroyTerraform(); err != nil {
		return err
	}
	_ = m.Provider.DeleteLoadTestDeploymentMetadata()
	return nil
}

// ===================== internals (delegate to existing helpers) =====================

func (m *LoadTestManager) prepareWorkspace() error {
	if err := os.MkdirAll(m.WorkingDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return filepath.Walk(m.TerraformDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(m.TerraformDir, p)
		target := filepath.Join(m.WorkingDir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(p, target)
	})
}
func (m *LoadTestManager) cleanup() {
	if m.WorkingDir != "" && strings.Contains(m.WorkingDir, "automock-lt-") {
		_ = os.RemoveAll(m.WorkingDir)
	}
}

func (m *LoadTestManager) createBackendConfigWithKey(key string) error {
	if m.BucketName == "" {
		return fmt.Errorf("no S3 bucket configured")
	}
	backend := fmt.Sprintf(`terraform {
  backend "s3" {
    bucket  = "%s"
    key     = "%s"
    region  = "%s"
    encrypt = true
  }
}
`, m.BucketName, key, m.Region)
	return osWriteFile(filepath.Join(m.WorkingDir, "backend.tf"), []byte(backend), 0644)
}

func (m *LoadTestManager) terraformEnv() []string {
	env := os.Environ()
	if m.Profile != "" && m.Provider.GetProviderType() == "aws" {
		env = append(env, fmt.Sprintf("AWS_PROFILE=%s", m.Profile))
	}
	env = append(env, "TF_CLI_CONFIG_FILE=/dev/null")
	return env
}
func (m *LoadTestManager) runTerraform(args ...string) error {
	cmd := exec.Command("terraform", args...)
	cmd.Dir = m.WorkingDir
	cmd.Env = m.terraformEnv()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	go streamLines(stdout, false)
	go streamLines(stderr, true)
	return cmd.Wait()
}
func (m *LoadTestManager) initTerraform() error {
	fmt.Println("🔧 terraform init (loadtest)...")
	return m.runTerraform("init")
}
func (m *LoadTestManager) planTerraform() error {
	fmt.Println("📋 terraform plan (loadtest)...")
	return m.runTerraform("plan", "-out=tfplan")
}
func (m *LoadTestManager) applyTerraform() error {
	fmt.Println("🏗️ terraform apply (loadtest)...")
	return m.runTerraform("apply", "-auto-approve", "tfplan")
}
func (m *LoadTestManager) destroyTerraform() error {
	fmt.Println("💥 terraform destroy (loadtest)...")
	return m.runTerraform("destroy", "-auto-approve")
}

func (m *LoadTestManager) createLoadTestVars(opts *models.LoadTestDeploymentOptions) error {
	varsFile := filepath.Join(m.WorkingDir, "terraform.tfvars")
	return osWriteFile(varsFile, []byte(opts.CreateTerraformVars()), 0644)
}

func (m *LoadTestManager) getLoadTestOutputs() (*models.LoadTestDeploymentOutputs, error) {
	cmd := exec.Command("terraform", "output", "-json")
	cmd.Dir = m.WorkingDir
	cmd.Env = m.terraformEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform output: %w", err)
	}
	var raw map[string]struct {
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse outputs: %w", err)
	}

	getStr := func(k string) string {
		if v, ok := raw[k]; ok {
			if s, ok := v.Value.(string); ok {
				return s
			}
		}
		return ""
	}
	getInt := func(k string) int {
		if v, ok := raw[k]; ok {
			switch t := v.Value.(type) {
			case float64:
				return int(t)
			case int:
				return t
			}
		}
		return 0
	}

	return &models.LoadTestDeploymentOutputs{
		Project:            m.ProjectName,
		ClusterName:        getStr("cluster_name"),
		MasterServiceName:  getStr("master_service_name"),
		WorkerServiceName:  getStr("worker_service_name"),
		WorkerDesiredCount: getInt("worker_desired_count"),
		ALBDNSName:         getStr("alb_dns_name"),
		CloudMapMasterFQDN: getStr("cloud_map_master_fqdn"),
		Region:             m.Region,
	}, nil
}

// Small seams to avoid importing os directly in this file and keep parity with manager.go patterns
func osTempDir() string { return os.TempDir() }
func osWriteFile(name string, data []byte, perm uint32) error {
	return os.WriteFile(name, data, os.FileMode(perm))
}

// copyFile utility
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func streamLines(r io.Reader, isErr bool) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if isErr {
			fmt.Fprintf(os.Stderr, "%s\n", sc.Text())
		} else {
			fmt.Println(sc.Text())
		}
	}
}

// ScaleWorkers updates worker_desired_count in terraform.tfvars and reapplies
func (m *LoadTestManager) ScaleWorkers(desired int) error {
	tfvars := filepath.Join(m.WorkingDir, "terraform.tfvars")
	data, err := os.ReadFile(tfvars)
	if err != nil {
		return fmt.Errorf("read tfvars: %w", err)
	}
	re := regexp.MustCompile(`(?m)^worker_desired_count\s*=\s*\d+`)
	updated := re.ReplaceAllString(string(data), fmt.Sprintf("worker_desired_count = %d", desired))
	if updated == string(data) {
		updated += fmt.Sprintf("\nworker_desired_count = %d\n", desired)
	}
	if err := os.WriteFile(tfvars, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write tfvars: %w", err)
	}
	if err := m.planTerraform(); err != nil {
		return err
	}
	if err := m.applyTerraform(); err != nil {
		return err
	}
	if out, err := m.getLoadTestOutputs(); err == nil {
		_ = m.Provider.SaveLoadTestDeploymentMetadata(out)
	}
	return nil
}

func parseWorkerCount(s string) (int, error) { return strconv.Atoi(strings.TrimSpace(s)) }
