// Package models provides shared data structures
package models

import (
	"fmt"
	"strings"
	"time"
)

// DeploymentMetadata tracks infrastructure deployment information
type DeploymentMetadata struct {
	ProjectName      string                 `json:"project_name"`
	DeploymentStatus string                 `json:"deployment_status"` // none, deploying, deployed, failed, destroyed
	DeployedAt       time.Time             `json:"deployed_at,omitempty"`
	CustomDomain     string                 `json:"custom_domain,omitempty"`      // persisted so destroy uses same vars
	CreateHostedZone bool                   `json:"create_hosted_zone,omitempty"` // persisted so destroy uses same vars
	PrivateALB       bool                   `json:"enable_private_alb,omitempty"` // persisted so destroy uses same vars
	Details          *InfrastructureOutputs `json:"details,omitempty"`
}

// InfrastructureOutputs contains Terraform outputs after deployment
type InfrastructureOutputs struct {
	MockServerURL         string                 `json:"mockserver_url"`
	DashboardURL          string                 `json:"dashboard_url"`
	ConfigBucket          string                 `json:"config_bucket"`
	IntegrationSummary    map[string]interface{} `json:"integration_summary"`
	CLICommands           map[string]string      `json:"cli_integration_commands"`
	InfrastructureSummary map[string]interface{} `json:"infrastructure_summary"`
	// CustomDomain is the base domain used during deployment (e.g. env.myhome.com).
	// Persisted so that the destroy command can reconstruct the correct tfvars.
	CustomDomain     string `json:"custom_domain,omitempty"`
	CreateHostedZone bool   `json:"create_hosted_zone,omitempty"`
	PrivateALB       bool   `json:"enable_private_alb,omitempty"`
}

// DeploymentOptions configures the infrastructure deployment
type DeploymentOptions struct {
	// === Common ===
	ProjectName string `json:"-"`
	Region      string `json:"-"` // AWS: region; Azure: location (e.g. "eastus")
	BucketName  string `json:"-"` // AWS: S3 bucket; Azure: storage account name
	Provider    string `json:"provider,omitempty"`

	// ── AWS-specific: Compute (ECS Fargate) ──────────────────────────────────
	InstanceSize string `json:"instance_size"`
	MinTasks     int    `json:"min_tasks"`
	MaxTasks     int    `json:"max_tasks"`
	MemoryUnits  int    `json:"memory_units"`
	CPUUnits     int    `json:"cpu_units"`

	// ── AWS-specific: Networking ──────────────────────────────────────────────
	UseExistingVPC            bool     `json:"-"`
	VpcID                     string   `json:"vpc_id,omitempty"`
	PublicSubnetIDs           []string `json:"public_subnet_ids,omitempty"`
	PrivateSubnetIDs          []string `json:"private_subnet_ids,omitempty"`
	SecurityGroupIDs          []string `json:"security_group_ids,omitempty"`
	UseExistingSubnets        bool     `json:"-"`
	UseExistingIGW            bool     `json:"-"`
	InternetGatewayID         string   `json:"internet_gateway_id,omitempty"`
	UseExistingNAT            bool     `json:"-"`
	NatGatewayIDs             []string `json:"nat_gateway_ids,omitempty"`
	UseExistingSecurityGroups bool     `json:"-"`

	// ── AWS-specific: IAM ─────────────────────────────────────────────────────
	UseExistingIAMRoles    bool    `json:"-"`
	ExecutionRoleARN       string  `json:"execution_role_arn,omitempty"`
	TaskRoleARN            string  `json:"task_role_arn,omitempty"`
	IAMRolePath            *string `json:"iam_role_path,omitempty"`
	IAMPermissionsBoundary *string `json:"iam_permissions_boundary,omitempty"`

	// ── AWS-specific: ALB / Custom Domain ────────────────────────────────────
	PrivateALB          bool     `json:"enable_private_alb,omitempty"`
	ALBIngressCIDRs     []string `json:"alb_ingress_cidr_blocks,omitempty"`
	CustomDomain        string   `json:"custom_domain,omitempty"`
	CreateHostedZone    bool     `json:"create_hosted_zone,omitempty"`

	// ── Azure-specific: Identity / Storage ───────────────────────────────────
	SubscriptionID       string `json:"subscription_id,omitempty"`
	ResourceGroup        string `json:"resource_group,omitempty"`
	StorageContainerName string `json:"container_name,omitempty"` // Blob container name
	// ── Azure-specific: AKS Node Pool ────────────────────────────────────────
	NodeVMSize string `json:"node_vm_size,omitempty"` // e.g. Standard_B2s
	NodeCount  int    `json:"node_count,omitempty"`   // default 1
}

// CreateTerraformVars renders terraform.tfvars as HCL based on DeploymentOptions.
// The output format varies by provider — AWS (ECS) and Azure (AKS) use different
// variable names and infrastructure primitives.
func (d *DeploymentOptions) CreateTerraformVars() string {
	switch d.Provider {
	case "azure":
		return d.createAzureTerraformVars()
	default:
		return d.createAWSTerraformVars()
	}
}

// createAWSTerraformVars renders the ECS/S3 tfvars for AWS deployments.
func (d *DeploymentOptions) createAWSTerraformVars() string {
	var b strings.Builder

	fmt.Fprintf(&b, `# AutoMock Terraform Variables (AWS)
# Generated automatically - do not edit manually

project_name         = "%s"
aws_region           = "%s"
instance_size        = "%s"
existing_bucket_name = "%s"
cloud_provider       = "%s"
`,
		d.ProjectName,
		d.Region,
		d.InstanceSize,
		d.BucketName,
		d.Provider,
	)

	// Sizing
	if d.CPUUnits != 0 {
		fmt.Fprintf(&b, "cpu_units          = %d\n", d.CPUUnits)
	}
	if d.MemoryUnits != 0 {
		fmt.Fprintf(&b, "memory_units       = %d\n", d.MemoryUnits)
	}
	if d.MinTasks != 0 {
		fmt.Fprintf(&b, "min_tasks          = %d\n", d.MinTasks)
	}
	if d.MaxTasks != 0 {
		fmt.Fprintf(&b, "max_tasks          = %d\n", d.MaxTasks)
	}

	// ───────────────────────── Networking (BYO vs Create) ─────────────────────────

	// VPC
	fmt.Fprintf(&b, "\nuse_existing_vpc   = %t\n", d.UseExistingVPC)
	if d.UseExistingVPC && d.VpcID != "" {
		fmt.Fprintf(&b, "vpc_id             = \"%s\"\n", d.VpcID)
	}

	// Subnets
	fmt.Fprintf(&b, "use_existing_subnets = %t\n", d.UseExistingSubnets)
	if d.UseExistingSubnets {
		if len(d.PublicSubnetIDs) > 0 {
			fmt.Fprintf(&b, "public_subnet_ids    = %s\n", formatStringList(d.PublicSubnetIDs))
		}
		if len(d.PrivateSubnetIDs) > 0 {
			fmt.Fprintf(&b, "private_subnet_ids   = %s\n", formatStringList(d.PrivateSubnetIDs))
		}
	}

	// IGW
	fmt.Fprintf(&b, "use_existing_igw   = %t\n", d.UseExistingIGW)
	if d.UseExistingIGW && d.InternetGatewayID != "" {
		fmt.Fprintf(&b, "internet_gateway_id = \"%s\"\n", d.InternetGatewayID)
	}

	// NAT
	fmt.Fprintf(&b, "use_existing_nat   = %t\n", d.UseExistingNAT)
	if d.UseExistingNAT && len(d.NatGatewayIDs) > 0 {
		fmt.Fprintf(&b, "nat_gateway_ids     = %s\n", formatStringList(d.NatGatewayIDs))
	}

	// Security Groups (ordered: [ALB, ECS])
	fmt.Fprintf(&b, "use_existing_security_groups = %t\n", d.UseExistingSecurityGroups)
	if d.UseExistingSecurityGroups && len(d.SecurityGroupIDs) > 0 {
		fmt.Fprintf(&b, "security_group_ids           = %s\n", formatStringList(d.SecurityGroupIDs))
	}

	// IAM Roles
	fmt.Fprintf(&b, "use_existing_iam_roles = %t\n", d.UseExistingIAMRoles)
	if d.UseExistingIAMRoles {
		if d.ExecutionRoleARN != "" {
			fmt.Fprintf(&b, "execution_role_arn = \"%s\"\n", d.ExecutionRoleARN)
		}
		if d.TaskRoleARN != "" {
			fmt.Fprintf(&b, "task_role_arn      = \"%s\"\n", d.TaskRoleARN)
		}
	} else {
		if d.IAMRolePath != nil {
			fmt.Fprintf(&b, "iam_role_path      = \"%s\"\n", *d.IAMRolePath)
		}
		if d.IAMPermissionsBoundary != nil {
			fmt.Fprintf(&b, "iam_permissions_boundary = \"%s\"\n", *d.IAMPermissionsBoundary)
		}
	}

	// Private ALB — always emitted so destroy uses the exact value from deploy
	fmt.Fprintf(&b, "\nenable_private_alb = %t\n", d.PrivateALB)

	// ALB ingress CIDRs — only emitted when explicitly set; Terraform default handles open access
	if len(d.ALBIngressCIDRs) > 0 {
		fmt.Fprintf(&b, "alb_ingress_cidr_blocks = %s\n", formatStringList(d.ALBIngressCIDRs))
	}

	// Custom domain (optional — emitted only when provided)
	if d.CustomDomain != "" {
		fmt.Fprintf(&b, "\ncustom_domain = \"%s\"\n", d.CustomDomain)
		if d.CreateHostedZone {
			fmt.Fprintf(&b, "create_hosted_zone = true\n")
		}
	}

	return b.String()
}

// createAzureTerraformVars renders the AKS/Blob tfvars for Azure deployments.
func (d *DeploymentOptions) createAzureTerraformVars() string {
	nodeVMSize := d.NodeVMSize
	if nodeVMSize == "" {
		nodeVMSize = "Standard_B2s"
	}
	nodeCount := d.NodeCount
	if nodeCount == 0 {
		nodeCount = 1
	}

	var b strings.Builder
	fmt.Fprintf(&b, `# AutoMock Terraform Variables (Azure)
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
`,
		d.ProjectName,
		d.Region,
		d.SubscriptionID,
		d.ResourceGroup,
		d.BucketName,
		d.StorageContainerName,
		d.Provider,
		nodeVMSize,
		nodeCount,
	)
	return b.String()
}

func formatStringList(xs []string) string {
	quoted := make([]string, 0, len(xs))
	for _, s := range xs {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	return fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
}
