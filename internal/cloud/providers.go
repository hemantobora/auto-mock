// Package cloud provides centralized cloud provider management.
// This file contains provider enumeration and selection logic.
package cloud

import (
	"fmt"
	"os"
	"strings"

	awscloud "github.com/hemantobora/auto-mock/internal/cloud/aws"
)

// ProviderType represents supported cloud providers
type ProviderType string

const (
	ProviderAWS   ProviderType = "aws"
	ProviderGCP   ProviderType = "gcp" 
	ProviderAzure ProviderType = "azure"
)

// ProviderInfo contains information about a cloud provider
type ProviderInfo struct {
	Type        ProviderType `json:"type"`
	Name        string       `json:"name"`
	Available   bool         `json:"available"`
	Description string       `json:"description"`
}

// GetAvailableProviders returns a list of available cloud providers
func GetAvailableProviders() []ProviderInfo {
	providers := []ProviderInfo{
		{
			Type:        ProviderAWS,
			Name:        "AWS",
			Available:   checkAWSAvailability(),
			Description: "Amazon Web Services",
		},
		// TODO: Add GCP and Azure providers
		// {
		// 	Type:        ProviderGCP,
		// 	Name:        "GCP",
		// 	Available:   checkGCPAvailability(),
		// 	Description: "Google Cloud Platform",
		// },
		// {
		// 	Type:        ProviderAzure,
		// 	Name:        "Azure",
		// 	Available:   checkAzureAvailability(),
		// 	Description: "Microsoft Azure",
		// },
	}

	return providers
}

// SelectProvider automatically selects the best available provider
func SelectProvider(profile string) (Provider, error) {
	providers := GetAvailableProviders()
	
	for _, provider := range providers {
		if provider.Available {
			switch provider.Type {
			case ProviderAWS:
				return awscloud.NewProvider(profile, "")
			// TODO: Add other providers
			}
		}
	}
	
	return nil, fmt.Errorf("no cloud providers available")
}

// checkAWSAvailability verifies if AWS credentials are configured
func checkAWSAvailability() bool {
	// Check for AWS credentials in environment or config files
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return true
	}
	
	// Check for AWS profile or default credentials
	if os.Getenv("AWS_PROFILE") != "" {
		return true
	}
	
	// Check for shared credentials file (basic check)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		credentialsPath := strings.Join([]string{homeDir, ".aws", "credentials"}, string(os.PathSeparator))
		if _, err := os.Stat(credentialsPath); err == nil {
			return true
		}
	}
	
	return false
}
