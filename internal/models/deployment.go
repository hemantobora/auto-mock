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
	DeployedAt       time.Time              `json:"deployed_at,omitempty"`
	DestroyedAt      time.Time              `json:"destroyed_at,omitempty"`
	Infrastructure   InfrastructureInfo     `json:"infrastructure"`
	Options          DeploymentOptions      `json:"options"`
	Outputs          map[string]interface{} `json:"outputs,omitempty"`
}

// InfrastructureInfo contains details about deployed resources
type InfrastructureInfo struct {
	ClusterName   string `json:"cluster_name"`
	ServiceName   string `json:"service_name"`
	ALBDNS        string `json:"alb_dns"`
	MockServerURL string `json:"mockserver_url"`
	DashboardURL  string `json:"dashboard_url"`
	VPCId         string `json:"vpc_id"`
	Region        string `json:"region"`
}

// DeploymentOptions configures the infrastructure deployment
type DeploymentOptions struct {
	// === Compute Configuration ===
	InstanceSize string `json:"instance_size"`
	MinTasks     int    `json:"min_tasks"`
	MaxTasks     int    `json:"max_tasks"`
	MemoryUnits  int    `json:"memory_units"`
	CPUUnits     int    `json:"cpu_units"`

	// === NETWORKING (FeatNetworking) ===
	// Most restricted in organizations - VPC/networking governance
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

	// === IAM ROLES (FeatIAMWrite, FeatPassRole) ===
	// Second most restricted - IAM governance
	UseExistingIAMRoles bool   `json:"-"`
	ExecutionRoleARN    string `json:"execution_role_arn,omitempty"` // ECS task execution role
	TaskRoleARN         string `json:"task_role_arn,omitempty"`      // ECS task role (app permissions)

	// === App Settings ===
	ProjectName string `json:"-"`
	Region      string `json:"-"`
	BucketName  string `json:"-"`
}

// CreateTerraformVars renders terraform.tfvars as HCL based on DeploymentOptions.
// It supports both BYO and "tool creates" modes by emitting explicit use_existing_* flags.
func (d *DeploymentOptions) CreateTerraformVars() string {
	var b strings.Builder

	fmt.Fprintf(&b, `# AutoMock Terraform Variables
# Generated automatically - do not edit manually

project_name         = "%s"
aws_region           = "%s"
instance_size        = "%s"
existing_bucket_name = "%s"
`,
		d.ProjectName,
		d.Region,
		d.InstanceSize,
		d.BucketName,
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
	}

	return b.String()
}

func formatStringList(xs []string) string {
	quoted := make([]string, 0, len(xs))
	for _, s := range xs {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	return fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
}
