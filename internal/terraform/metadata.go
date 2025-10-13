// internal/terraform/metadata.go
// Deployment metadata management
package terraform

import (
	"context"
	"fmt"
	"time"

	"github.com/hemantobora/auto-mock/internal/models"
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

	// Build metadata
	metadata := &models.DeploymentMetadata{
		ProjectName:      m.ProjectName,
		DeploymentStatus: "deployed",
		DeployedAt:       time.Now(),
		TTLHours:         options.TTLHours,
		Infrastructure: models.InfrastructureInfo{
			MockServerURL: outputs.MockServerURL,
			DashboardURL:  outputs.DashboardURL,
			Region:        m.Region,
		},
		Options: models.DeploymentOptions{
			InstanceSize:      options.InstanceSize,
			MinTasks:          options.MinTasks, // From terraform default
			MaxTasks:          options.MaxTasks,
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

	// Extract cluster
	if cluster, ok := outputs.InfrastructureSummary["cluster"].(map[string]interface{}); ok {
		if name, ok := cluster["name"].(string); ok {
			metadata.Infrastructure.ClusterName = name
		}
	}

	// Extract service
	if service, ok := outputs.InfrastructureSummary["service"].(map[string]interface{}); ok {
		if name, ok := service["name"].(string); ok {
			metadata.Infrastructure.ServiceName = name
		}
	}

	// Extract VPC
	if networking, ok := outputs.InfrastructureSummary["networking"].(map[string]interface{}); ok {
		if vpcId, ok := networking["vpc_id"].(string); ok {
			metadata.Infrastructure.VPCId = vpcId
		}
	}

	// Store all outputs for reference
	metadata.Outputs = make(map[string]interface{})
	metadata.Outputs["mockserver_url"] = outputs.MockServerURL
	metadata.Outputs["dashboard_url"] = outputs.DashboardURL
	metadata.Outputs["config_bucket"] = outputs.ConfigBucket

	// Save to S3
	return m.Provider.SaveDeploymentMetadata(ctx, metadata)
}
