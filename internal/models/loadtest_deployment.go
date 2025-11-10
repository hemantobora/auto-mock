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
	Region      string `json:"-"`
	BucketName  string `json:"-"`
	Provider    string `json:"provider,omitempty"`

	// Sizing
	CPUUnits           int `json:"cpu_units"`
	MemoryUnits        int `json:"memory_units"`
	WorkerDesiredCount int `json:"worker_desired_count"`
	// ExtraEnvironment allows users to inject arbitrary KEY=VALUE pairs into Locust ECS task containers.
	ExtraEnvironment map[string]string `json:"extra_environment,omitempty"`

	// BYO Networking (like mockserver, collected from user when chosen)
	UseExistingVPC     bool     `json:"-"`
	VpcID              string   `json:"vpc_id,omitempty"`
	UseExistingSubnets bool     `json:"-"`
	PublicSubnetIDs    []string `json:"public_subnet_ids,omitempty"`
	UseExistingIGW     bool     `json:"-"`
	InternetGatewayID  string   `json:"internet_gateway_id,omitempty"`

	// BYO IAM Roles
	UseExistingIAMRoles bool   `json:"-"`
	ExecutionRoleARN    string `json:"execution_role_arn,omitempty"`
	TaskRoleARN         string `json:"task_role_arn,omitempty"`

	// BYO Security Groups
	UseExistingSecurityGroups bool   `json:"-"`
	ALBSecurityGroupID        string `json:"alb_security_group_id,omitempty"`
	ECSSecurityGroupID        string `json:"ecs_security_group_id,omitempty"`
}

// CreateTerraformVars renders terraform.tfvars for the loadtest stack
func (o *LoadTestDeploymentOptions) CreateTerraformVars() string {
	// Base variables (networking/IAM handled internally; BYO/env handled separately)
	base := fmt.Sprintf(`# AutoMock LoadTest Terraform Variables
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

	// BYO networking toggles (emit explicitly when user provides them)
	// VPC
	base += fmt.Sprintf("\nuse_existing_vpc   = %t\n", o.UseExistingVPC)
	if o.UseExistingVPC && o.VpcID != "" {
		base += fmt.Sprintf("vpc_id             = \"%s\"\n", o.VpcID)
	}
	// Subnets
	base += fmt.Sprintf("use_existing_subnets = %t\n", o.UseExistingSubnets)
	if o.UseExistingSubnets && len(o.PublicSubnetIDs) > 0 {
		base += fmt.Sprintf("public_subnet_ids    = %s\n", formatStringList(o.PublicSubnetIDs))
	}
	// IGW (optional; not required for BYO in this module but preserved for parity)
	base += fmt.Sprintf("use_existing_igw   = %t\n", o.UseExistingIGW)
	if o.UseExistingIGW && o.InternetGatewayID != "" {
		base += fmt.Sprintf("internet_gateway_id = \"%s\"\n", o.InternetGatewayID)
	}

	// IAM Roles
	base += fmt.Sprintf("use_existing_iam_roles = %t\n", o.UseExistingIAMRoles)
	if o.UseExistingIAMRoles {
		if o.ExecutionRoleARN != "" {
			base += fmt.Sprintf("execution_role_arn = \"%s\"\n", o.ExecutionRoleARN)
		}

		// Security Groups
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

	if len(o.ExtraEnvironment) > 0 {
		base += "extra_environment = {\n"
		keys := make([]string, 0, len(o.ExtraEnvironment))
		for k := range o.ExtraEnvironment {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := o.ExtraEnvironment[k]
			v = strings.ReplaceAll(v, "\"", "\\\"")
			base += fmt.Sprintf("  %s = \"%s\"\n", k, v)
		}
		base += "}\n"
	}
	return base
}

// formatStringList mirrors models.DeploymentOptions helper for consistent HCL lists
