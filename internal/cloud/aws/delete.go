// Package aws provides AWS-specific cloud infrastructure management for auto-mock.
// This file handles project deletion operations.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// DeleteProject removes the S3 bucket and all associated resources for a project
func (p *Provider) DeleteProject() error {
	cleanName := utils.ExtractUserProjectName(p.ProjectName)
	
	// Delete the S3 bucket
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, try to delete all objects in the bucket (if any)
	// Note: In the current MVP, we don't store objects, but this is future-proof
	
	// Delete the bucket itself
	_, err := p.S3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &p.BucketName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %w", p.BucketName, err)
	}

	fmt.Printf("🗑️ Deleted project: %s\n", cleanName)
	fmt.Println("ℹ️ TTL Lambda deletion skipped — not yet provisioned in current MVP")
	
	return nil
}
