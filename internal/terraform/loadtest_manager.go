// internal/terraform/loadtest_manager.go
package terraform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	// Capture the exact tfvars used so future operations (e.g., scale) can reuse them
	tfvarsPath := filepath.Join(m.WorkingDir, "terraform.tfvars")
	tfvarsBytes, _ := os.ReadFile(tfvarsPath)

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
	if len(tfvarsBytes) > 0 {
		if out.Extras == nil {
			out.Extras = map[string]string{}
		}
		out.Extras["tfvars"] = string(tfvarsBytes)
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
	// Provide required variables to avoid interactive prompts during destroy
	if err := m.createLoadTestVars(&models.LoadTestDeploymentOptions{
		ProjectName:        m.ProjectName,
		Region:             m.Region,
		BucketName:         m.BucketName,
		Provider:           m.Provider.GetProviderType(),
		CPUUnits:           256,
		MemoryUnits:        512,
		WorkerDesiredCount: 0,
	}); err != nil {
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
	l := models.NewLoader(os.Stdout, "Preparing workspace")
	l.Start()
	defer l.Stop()
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
	l := models.NewLoader(os.Stdout, "Initializing Terraform")
	l.Start()
	defer l.Stop()
	cmd := exec.Command("terraform", "init")
	cmd.Dir = m.WorkingDir
	cmd.Env = m.terraformEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Ensure loader line cleared, then print error output
		l.StopWithMessage("")
		return fmt.Errorf("terraform init failed: %w\n%s", err, string(out))
	}
	return nil
}
func (m *LoadTestManager) planTerraform() error {
	fmt.Println("📋 terraform plan (loadtest)...")
	l := models.NewLoader(os.Stdout, "Planning changes")
	l.Start()
	defer l.Stop()
	cmd := exec.Command("terraform", "plan", "-out=tfplan")
	cmd.Dir = m.WorkingDir
	cmd.Env = m.terraformEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		l.StopWithMessage("")
		return fmt.Errorf("terraform plan failed: %w\n%s", err, string(out))
	}
	return nil
}
func (m *LoadTestManager) applyTerraform() error {
	fmt.Println("🏗️ terraform apply (loadtest)...")
	// Stream output for apply so users see progress details
	return m.runTerraform("apply", "-auto-approve", "tfplan")
}
func (m *LoadTestManager) destroyTerraform() error {
	fmt.Println("💥 terraform destroy (loadtest)...")
	// Stream output for destroy as well
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
	// Always use an ephemeral workspace for scaling; restore tfvars from metadata to avoid drift
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

	tfvars := filepath.Join(m.WorkingDir, "terraform.tfvars")
	if _, statErr := os.Stat(tfvars); os.IsNotExist(statErr) {
		// Restore exact variables from saved deployment metadata (preferred)
		md, mdErr := m.Provider.GetLoadTestDeploymentMetadata()
		if mdErr != nil || md == nil || md.Details == nil || md.Details.Extras == nil {
			return fmt.Errorf("cannot scale safely: missing saved tfvars in deployment metadata; run 'automock deploy --project %s' once to capture variables, then retry scaling", m.ProjectName)
		}
		tfv, ok := md.Details.Extras["tfvars"]
		if !ok || strings.TrimSpace(tfv) == "" {
			return fmt.Errorf("cannot scale safely: no tfvars found in metadata; run 'automock deploy --project %s' to refresh metadata, then retry", m.ProjectName)
		}
		if err := os.WriteFile(tfvars, []byte(tfv), 0644); err != nil {
			return fmt.Errorf("restore tfvars: %w", err)
		}
	}

	data, err := os.ReadFile(tfvars)
	if err != nil {
		return fmt.Errorf("read tfvars: %w", err)
	}
	// Remove any existing worker_desired_count lines (with optional leading whitespace)
	reLine := regexp.MustCompile(`(?m)^\s*worker_desired_count\s*=.*$`)
	cleaned := reLine.ReplaceAllString(string(data), "")
	// Normalize multiple blank lines possibly introduced by removals
	var outLines []string
	for _, ln := range strings.Split(cleaned, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		outLines = append(outLines, ln)
	}
	outLines = append(outLines, fmt.Sprintf("worker_desired_count = %d", desired))
	updated := strings.Join(outLines, "\n") + "\n"
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
		// Persist the updated tfvars alongside outputs to keep future scales drift-free
		if b, rerr := os.ReadFile(tfvars); rerr == nil {
			if out.Extras == nil {
				out.Extras = map[string]string{}
			}
			out.Extras["tfvars"] = string(b)
		}
		_ = m.Provider.SaveLoadTestDeploymentMetadata(out)
	}
	return nil
}
