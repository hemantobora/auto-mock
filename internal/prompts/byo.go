package prompts

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// PromptBYONetworking prompts for BYO VPC and subnet configuration
// Returns error if user confirms BYO but provides invalid input
func PromptBYONetworking(opts *models.LoadTestDeploymentOptions) error {
	var useBYO bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Bring your own networking (VPC & subnets)?",
		Default: false,
	}, &useBYO)

	if !useBYO {
		return nil
	}

	opts.UseExistingVPC = true
	opts.UseExistingSubnets = true

	var vpcID string
	_ = survey.AskOne(&survey.Input{
		Message: "Network ID (e.g., AWS VPC ID vpc-xxxx):",
	}, &vpcID)
	vpcID = strings.TrimSpace(vpcID)
	if vpcID == "" {
		return fmt.Errorf("VPC ID is required when using BYO networking")
	}
	opts.VpcID = vpcID

	var subnetsCSV string
	_ = survey.AskOne(&survey.Input{
		Message: "Public subnet IDs (comma-separated):",
		Help:    "e.g., AWS: subnet-aaaa,subnet-bbbb",
	}, &subnetsCSV)
	subnetsCSV = strings.TrimSpace(subnetsCSV)
	if subnetsCSV == "" {
		return fmt.Errorf("at least one subnet ID is required when using BYO networking")
	}

	parts := strings.Split(subnetsCSV, ",")
	var subs []string
	for _, p := range parts {
		pp := strings.TrimSpace(p)
		if pp != "" {
			subs = append(subs, pp)
		}
	}
	if len(subs) == 0 {
		return fmt.Errorf("no valid subnet IDs provided")
	}
	opts.PublicSubnetIDs = subs

	return nil
}

// PromptBYOIGW prompts for using an existing Internet Gateway when BYO VPC is enabled
func PromptBYOIGW(opts *models.LoadTestDeploymentOptions) error {
	if !opts.UseExistingVPC {
		// Only relevant when user brings their own VPC
		opts.UseExistingIGW = false
		opts.InternetGatewayID = ""
		return nil
	}
	opts.UseExistingIGW = true
	var igwID string
	_ = survey.AskOne(&survey.Input{Message: "Internet Gateway ID (e.g., igw-xxxx):"}, &igwID)
	igwID = strings.TrimSpace(igwID)
	// IGW ID is optional for our module when BYO VPC is true, but capture if provided
	opts.InternetGatewayID = igwID
	return nil
}

// PromptExtraEnvironment prompts for environment variables via .env file and/or manual key=value entries
func PromptExtraEnvironment(opts *models.LoadTestDeploymentOptions) error {
	// Initialize map if nil
	if opts.ExtraEnvironment == nil {
		opts.ExtraEnvironment = map[string]string{}
	}

	// Ask for .env file path (optional)
	var useEnvFile bool
	_ = survey.AskOne(&survey.Confirm{Message: "Load environment variables from a .env file?", Default: false}, &useEnvFile)
	if useEnvFile {
		var path string
		_ = survey.AskOne(&survey.Input{Message: "Path to .env file:"}, &path)
		path = strings.TrimSpace(path)
		if path != "" {
			if err := loadEnvFileInto(path, opts.ExtraEnvironment); err != nil {
				return fmt.Errorf("read .env: %w", err)
			}
		}
	}

	// Allow manual additions
	var addManual bool
	_ = survey.AskOne(&survey.Confirm{Message: "Add environment variables manually?", Default: len(opts.ExtraEnvironment) == 0}, &addManual)
	if addManual {
		for {
			var kv string
			_ = survey.AskOne(&survey.Input{Message: "Enter KEY=VALUE (blank to finish):"}, &kv)
			kv = strings.TrimSpace(kv)
			if kv == "" {
				break
			}
			k, v, ok := splitOnce(kv)
			if !ok || k == "" {
				fmt.Println("Skipping invalid entry; expected KEY=VALUE")
				continue
			}
			opts.ExtraEnvironment[k] = v
		}
	}
	return nil
}

func splitOnce(s string) (string, string, bool) {
	idx := strings.IndexByte(s, '=')
	if idx <= 0 {
		return "", "", false
	}
	k := strings.TrimSpace(s[:idx])
	v := strings.TrimSpace(s[idx+1:])
	// Unquote simple quotes if present
	if len(v) >= 2 {
		if (v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"') {
			v = v[1 : len(v)-1]
		}
	}
	return k, v, true
}

func loadEnvFileInto(path string, dst map[string]string) error {
	// Expand ~ and relative paths
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		if home != "" {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// allow leading "export "
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		k, v, ok := splitOnce(line)
		if ok && k != "" {
			dst[k] = v
		}
	}
	return sc.Err()
}

// PromptBYOIAM prompts for BYO IAM role configuration
// Returns error if user confirms BYO but provides invalid input
func PromptBYOIAM(opts *models.LoadTestDeploymentOptions) error {
	var useIAM bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Use existing IAM roles for ECS (execution & task)?",
		Default: false,
	}, &useIAM)

	if !useIAM {
		return nil
	}

	opts.UseExistingIAMRoles = true

	var execArn, taskArn string
	_ = survey.AskOne(&survey.Input{
		Message: "Execution role (ARN on AWS):",
	}, &execArn)
	_ = survey.AskOne(&survey.Input{
		Message: "Task role (ARN on AWS; press Enter to reuse execution role, Make sure role has s3 and kms access):",
	}, &taskArn)

	execArn = strings.TrimSpace(execArn)
	taskArn = strings.TrimSpace(taskArn)

	if execArn == "" {
		return fmt.Errorf("execution role ARN is required when using existing IAM roles")
	}
	if taskArn == "" {
		taskArn = execArn
	}

	opts.ExecutionRoleARN = execArn
	opts.TaskRoleARN = taskArn

	return nil
}

// PromptBYOSecurityGroups prompts for BYO security group configuration
// Returns error if user confirms BYO but provides invalid input
func PromptBYOSecurityGroups(opts *models.LoadTestDeploymentOptions) error {
	var useSG bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Use existing Security Groups (ALB & ECS)?",
		Default: false,
	}, &useSG)

	if !useSG {
		return nil
	}

	opts.UseExistingSecurityGroups = true

	var albSG, ecsSG string
	_ = survey.AskOne(&survey.Input{
		Message: "ALB Security Group ID:",
	}, &albSG)
	_ = survey.AskOne(&survey.Input{
		Message: "ECS Tasks Security Group ID:",
	}, &ecsSG)

	albSG = strings.TrimSpace(albSG)
	ecsSG = strings.TrimSpace(ecsSG)

	if albSG == "" || ecsSG == "" {
		return fmt.Errorf("both ALB and ECS security group IDs are required when using existing security groups")
	}

	opts.ALBSecurityGroupID = albSG
	opts.ECSSecurityGroupID = ecsSG

	return nil
}

// PromptWorkerCount prompts for desired worker count
// Returns error if input cannot be parsed as a non-negative integer
func PromptWorkerCount(opts *models.LoadTestDeploymentOptions) error {
	var workerStr string
	_ = survey.AskOne(&survey.Input{
		Message: "Desired worker count (0 for none):",
		Default: "0",
	}, &workerStr)

	workerStr = strings.TrimSpace(workerStr)
	if workerStr == "" {
		workerStr = "0"
	}

	n, err := strconv.Atoi(workerStr)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid worker count: %s", workerStr)
	}

	opts.WorkerDesiredCount = n
	return nil
}

// PromptAllBYOOptions prompts for all BYO options in sequence
// This is a convenience function that combines all BYO prompts
func PromptAllBYOOptions(opts *models.LoadTestDeploymentOptions) error {
	// Prompt for networking
	if err := PromptBYONetworking(opts); err != nil {
		return err
	}

	// Optional: Internet Gateway prompt for BYO VPC
	if err := PromptBYOIGW(opts); err != nil {
		return err
	}

	// Prompt for security groups
	if err := PromptBYOSecurityGroups(opts); err != nil {
		return err
	}

	// Prompt for IAM
	if err := PromptBYOIAM(opts); err != nil {
		return err
	}

	// Prompt for extra environment (.env + manual)
	if err := PromptExtraEnvironment(opts); err != nil {
		return err
	}

	// Prompt for worker count
	return PromptWorkerCount(opts)
}
