// Package models provides shared data structures
package models

import "time"

// DeploymentMetadata tracks infrastructure deployment information
type DeploymentMetadata struct {
	ProjectName      string                 `json:"project_name"`
	DeploymentStatus string                 `json:"deployment_status"` // none, deploying, deployed, failed, destroyed
	DeployedAt       time.Time              `json:"deployed_at,omitempty"`
	DestroyedAt      time.Time              `json:"destroyed_at,omitempty"`
	TTLHours         int                    `json:"ttl_hours"`
	TTLExpiry        time.Time              `json:"ttl_expiry,omitempty"`
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

// DeploymentOptions contains the options used for deployment
type DeploymentOptions struct {
	InstanceSize      string `json:"instance_size"`
	MinTasks          int    `json:"min_tasks"`
	MaxTasks          int    `json:"max_tasks"`
	CustomDomain      string `json:"custom_domain,omitempty"`
	NotificationEmail string `json:"notification_email,omitempty"`
}
