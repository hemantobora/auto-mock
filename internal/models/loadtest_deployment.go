package models

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// LoadTestDeploymentOutputs captures outputs for the Locust load-testing stack
type LoadTestDeploymentOutputs struct {
	Project            string            `json:"project"`
	ClusterName        string            `json:"cluster_name"`
	MasterServiceName  string            `json:"master_service_name"`
	WorkerServiceName  string            `json:"worker_service_name"`
	WorkerDesiredCount int               `json:"worker_desired_count"`
	ALBDNSName         string            `json:"alb_dns_name"`
	CloudMapMasterFQDN string            `json:"cloud_map_master_fqdn"`
	Region             string            `json:"region"`
	Extras             map[string]string `json:"extras,omitempty"`
}

// LoadTestDeploymentMetadata tracks lifecycle for the loadtest infra
type LoadTestDeploymentMetadata struct {
	ProjectName      string                     `json:"project_name"`
	DeploymentStatus string                     `json:"deployment_status"` // none, deploying, deployed, failed, destroyed
	DeployedAt       time.Time                  `json:"deployed_at,omitempty"`
	Details          *LoadTestDeploymentOutputs `json:"details,omitempty"`
}

// LoadTestDeploymentOptions configures the Locust infrastructure deployment
type LoadTestDeploymentOptions struct {
	ProjectName string `json:"-"`
	Region      string `json:"-"` // AWS: region string; Azure: location (e.g. "eastus")
	BucketName  string `json:"-"` // AWS: S3 bucket name; Azure: storage account name
	Provider    string `json:"provider,omitempty"`

	// Sizing — AWS (ECS Fargate units)
	CPUUnits    int `json:"cpu_units"`
	MemoryUnits int `json:"memory_units"`

	WorkerDesiredCount int `json:"worker_desired_count"`
	// ExtraEnvironment allows users to inject arbitrary KEY=VALUE pairs into Locust containers.
	ExtraEnvironment map[string]string `json:"extra_environment,omitempty"`

	// ── AWS-specific BYO Networking ───────────────────────────────────────────
	UseExistingVPC     bool     `json:"-"`
	VpcID              string   `json:"vpc_id,omitempty"`
	UseExistingSubnets bool     `json:"-"`
	PublicSubnetIDs    []string `json:"public_subnet_ids,omitempty"`
	UseExistingIGW     bool     `json:"-"`
	InternetGatewayID  string   `json:"internet_gateway_id,omitempty"`

	// BYO IAM Roles (AWS)
	UseExistingIAMRoles bool   `json:"-"`
	ExecutionRoleARN    string `json:"execution_role_arn,omitempty"`
	TaskRoleARN         string `json:"task_role_arn,omitempty"`

	// BYO Security Groups (AWS)
	UseExistingSecurityGroups bool   `json:"-"`
	ALBSecurityGroupID        string `json:"alb_security_group_id,omitempty"`
	ECSSecurityGroupID        string `json:"ecs_security_group_id,omitempty"`

	// ── Azure-specific ────────────────────────────────────────────────────────
	SubscriptionID      string `json:"subscription_id,omitempty"`
	ResourceGroup       string `json:"resource_group,omitempty"`
	StorageContainerName string `json:"container_name,omitempty"` // Blob container
	NodeVMSize          string `json:"node_vm_size,omitempty"`   // AKS VM size
	NodeCount           int    `json:"node_count,omitempty"`     // AKS node count
}

// CreateTerraformVars renders terraform.tfvars for the loadtest stack.
// The output format varies by provider since AWS (ECS) and Azure (AKS) use
// fundamentally different variable names and sizing primitives.
func (o *LoadTestDeploymentOptions) CreateTerraformVars() string {
	switch o.Provider {
	case "azure":
		return o.createAzureTerraformVars()
	default:
		return o.createAWSTerraformVars()
	}
}

// createAWSTerraformVars renders terraform.tfvars for the AWS (ECS Fargate) loadtest stack.
func (o *LoadTestDeploymentOptions) createAWSTerraformVars() string {
	base := fmt.Sprintf(`# AutoMock LoadTest Terraform Variables (AWS)
# Generated automatically - do not edit manually

project_name         = "%s"
aws_region           = "%s"
existing_bucket_name = "%s"
cloud_provider       = "%s"

cpu_units            = %d
memory_units         = %d
worker_desired_count = %d
`,
		o.ProjectName,
		o.Region,
		o.BucketName,
		o.Provider,
		o.CPUUnits,
		o.MemoryUnits,
		o.WorkerDesiredCount,
	)

	// BYO networking
	base += fmt.Sprintf("\nuse_existing_vpc   = %t\n", o.UseExistingVPC)
	if o.UseExistingVPC && o.VpcID != "" {
		base += fmt.Sprintf("vpc_id             = \"%s\"\n", o.VpcID)
	}
	base += fmt.Sprintf("use_existing_subnets = %t\n", o.UseExistingSubnets)
	if o.UseExistingSubnets && len(o.PublicSubnetIDs) > 0 {
		base += fmt.Sprintf("public_subnet_ids    = %s\n", formatStringList(o.PublicSubnetIDs))
	}
	base += fmt.Sprintf("use_existing_igw   = %t\n", o.UseExistingIGW)
	if o.UseExistingIGW && o.InternetGatewayID != "" {
		base += fmt.Sprintf("internet_gateway_id = \"%s\"\n", o.InternetGatewayID)
	}

	// BYO IAM Roles
	base += fmt.Sprintf("use_existing_iam_roles = %t\n", o.UseExistingIAMRoles)
	if o.UseExistingIAMRoles {
		if o.ExecutionRoleARN != "" {
			base += fmt.Sprintf("execution_role_arn = \"%s\"\n", o.ExecutionRoleARN)
		}
		base += fmt.Sprintf("use_existing_security_groups = %t\n", o.UseExistingSecurityGroups)
		if o.UseExistingSecurityGroups {
			if o.ALBSecurityGroupID != "" {
				base += fmt.Sprintf("alb_security_group_id        = \"%s\"\n", o.ALBSecurityGroupID)
			}
			if o.ECSSecurityGroupID != "" {
				base += fmt.Sprintf("ecs_security_group_id        = \"%s\"\n", o.ECSSecurityGroupID)
			}
		}
		if o.TaskRoleARN != "" {
			base += fmt.Sprintf("task_role_arn      = \"%s\"\n", o.TaskRoleARN)
		}
	}

	base += appendExtraEnvironment(o.ExtraEnvironment)
	return base
}

// createAzureTerraformVars renders terraform.tfvars for the Azure (AKS) loadtest stack.
func (o *LoadTestDeploymentOptions) createAzureTerraformVars() string {
	nodeVMSize := o.NodeVMSize
	if nodeVMSize == "" {
		nodeVMSize = "Standard_B2s"
	}
	nodeCount := o.NodeCount
	if nodeCount == 0 {
		nodeCount = 1
	}

	base := fmt.Sprintf(`# AutoMock LoadTest Terraform Variables (Azure)
# Generated automatically - do not edit manually

project_name         = "%s"
location             = "%s"
subscription_id      = "%s"
resource_group       = "%s"
storage_account_name = "%s"
container_name       = "%s"
cloud_provider       = "%s"

node_vm_size         = "%s"
node_count           = %d
worker_desired_count = %d
`,
		o.ProjectName,
		o.Region,
		o.SubscriptionID,
		o.ResourceGroup,
		o.BucketName,
		o.StorageContainerName,
		o.Provider,
		nodeVMSize,
		nodeCount,
		o.WorkerDesiredCount,
	)

	base += appendExtraEnvironment(o.ExtraEnvironment)
	return base
}

// appendExtraEnvironment renders the extra_environment HCL map block (shared by AWS and Azure).
func appendExtraEnvironment(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	out := "extra_environment = {\n"
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := strings.ReplaceAll(env[k], "\"", "\\\"")
		out += fmt.Sprintf("  %s = \"%s\"\n", k, v)
	}
	out += "}\n"
	return out
}

// formatStringList mirrors models.DeploymentOptions helper for consistent HCL lists
