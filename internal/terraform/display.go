// internal/terraform/display.go
package terraform

import (
	"fmt"

	"github.com/hemantobora/auto-mock/internal/models"
)

type InfrastructureOutputs = models.InfrastructureOutputs

// DisplayDestroyConfirmation shows a warning before destroying infrastructure
func DisplayDestroyConfirmation(projectName string) {
	fmt.Printf("\n⚠️  WARNING: This will permanently delete all infrastructure for project: %s\n", projectName)
	fmt.Println("   Includes: ECS service, load balancer, networking resources, storage, and logs.")
	fmt.Println("   This cannot be undone.")
	fmt.Println()
}
