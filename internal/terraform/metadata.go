// internal/terraform/metadata.go
// Deployment metadata management
package terraform

import (
	"context"
	"fmt"
	"time"

	"github.com/hemantobora/auto-mock/internal/state"
)

// saveDeploymentMetadata saves deployment information to S3
func (m *Manager) saveDeploymentMetadata(outputs *InfrastructureOutputs, options *DeploymentOptions) error {
	ctx := context.Background()
	
	// Use the actual config bucket from deployment outputs, not a generated name
	// The bucket may have a random suffix (e.g., auto-mock-project-abc123)
	bucketName := outputs.ConfigBucket
	if bucketName == "" {
		// Fallback to existing bucket name from manager
		bucketName = m.ExistingBucketName
	}
	
	if bucketName == "" {
		return fmt.Errorf("no config bucket available for metadata")
	}
	
	// Create S3 store with actual bucket name
	store, err := state.CreateS3StoreWithBucket(ctx, m.ProjectName, bucketName, m.AWSProfile)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	
	// Build metadata
	metadata := &state.DeploymentMetadata{
		ProjectName:      m.ProjectName,
		DeploymentStatus: "deployed",
		DeployedAt:       time.Now(),
		TTLHours:         options.TTLHours,
		Infrastructure: state.InfrastructureInfo{
			MockServerURL: outputs.MockServerURL,
			DashboardURL:  outputs.DashboardURL,
			Region:        m.Region,
		},
		Options: state.DeploymentOptions{
			InstanceSize:      options.InstanceSize,
			MinTasks:          10, // From terraform default
			MaxTasks:          200,
			CustomDomain:      options.CustomDomain,
			NotificationEmail: options.NotificationEmail,
		},
	}
	
	// Set TTL expiry if enabled
	if options.TTLHours > 0 {
		metadata.TTLExpiry = time.Now().Add(time.Duration(options.TTLHours) * time.Hour)
	}
	
	// Extract infrastructure details from outputs
	if summary, ok := outputs.InfrastructureSummary["compute"].(map[string]interface{}); ok {
		if clusterName, ok := summary["cluster"].(string); ok {
			metadata.Infrastructure.ClusterName = clusterName
		}
		if serviceName, ok := summary["service"].(string); ok {
			metadata.Infrastructure.ServiceName = serviceName
		}
	}
	
	if vpcId, ok := outputs.InfrastructureSummary["vpc_id"].(string); ok {
		metadata.Infrastructure.VPCId = vpcId
	}
	
	if albDNS, ok := outputs.InfrastructureSummary["alb_dns_name"].(string); ok {
		metadata.Infrastructure.ALBDNS = albDNS
	}
	
	// Store all outputs for reference
	metadata.Outputs = make(map[string]interface{})
	metadata.Outputs["mockserver_url"] = outputs.MockServerURL
	metadata.Outputs["dashboard_url"] = outputs.DashboardURL
	metadata.Outputs["config_bucket"] = outputs.ConfigBucket
	
	// Save to S3
	return store.SaveDeploymentMetadata(ctx, metadata)
}

// updateMetadataStatus updates deployment status in S3
func (m *Manager) updateMetadataStatus(status string) error {
	ctx := context.Background()
	
	store, err := state.StoreForProject(ctx, m.ProjectName)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	
	return store.UpdateDeploymentStatus(ctx, status)
}
