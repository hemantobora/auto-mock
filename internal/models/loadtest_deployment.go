package models

import (
	"fmt"
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
}

// CreateTerraformVars renders terraform.tfvars for the loadtest stack
func (o *LoadTestDeploymentOptions) CreateTerraformVars() string {
	// Keep a minimal set for MVP; networking/IAM are managed internally for now.
	return fmt.Sprintf(`# AutoMock LoadTest Terraform Variables
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
}
