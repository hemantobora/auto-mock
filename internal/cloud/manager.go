package cloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/hemantobora/auto-mock/internal/provider"
	awscloud "github.com/hemantobora/auto-mock/internal/cloud/aws"
	"github.com/hemantobora/auto-mock/internal/repl"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// AutoDetectAndInit is the main entrypoint for the CLI's `init` command.
// It supports both interactive (REPL) and CLI-driven project initialization.
func AutoDetectAndInit(profile string, project string) error {
	// Step 1: Detect available cloud providers (AWS only for now)
	validProviders := []string{}

	if checkAWSCredentials(profile) {
		validProviders = append(validProviders, "aws")
	}

	if len(validProviders) == 0 {
		return errors.New("❌ No valid cloud provider credentials found. Please configure AWS, GCP, or Azure")
	}

	if len(validProviders) > 1 {
		return errors.New("⚠️ Multiple cloud providers detected — interactive selection not yet implemented")
	}

	fmt.Println("🔍 Detected valid AWS credentials — proceeding with AWS provider...")

	   var selectedProject string

	   if project != "" {
		   // Step 2: Handle CLI-driven --project flag
		   buckets, err := awscloud.ListBucketsWithPrefix(profile, "auto-mock-")
		   if err != nil {
			   return err
		   }

		   // Case-insensitive match against existing base project names
		   for _, bucket := range buckets {
			   trimmed := utils.RemoveBucketPrefix(bucket)
			   parts := strings.Split(trimmed, "-")
			   if len(parts) < 2 {
					continue // skip invalid bucket name
			   }
			   base := strings.Join(parts[:len(parts)-1], "-")
			   if strings.EqualFold(base, project) {
				   selectedProject = trimmed

				   // Project exists — enter REPL for actions
				   action := repl.SelectProjectAction(project)
				   awsProvider, err := awscloud.NewProvider(profile, selectedProject)
				   if err != nil {
					   return fmt.Errorf("failed to initialize AWS provider: %w", err)
				   }
				   var prov provider.Provider = awsProvider
				   switch action {
				   case "Delete":
					   return prov.DeleteProject()
				   case "Edit":
					   fmt.Println("🛠️ Edit stubs coming soon...")
					   return nil
				   case "Cancel":
					   fmt.Println("❌ Cancelled.")
					   return nil
				   }
				   break
			   }
		   }

		   suffix, err := utils.GenerateRandomSuffix()
		   if err != nil {
			   return nil
		   }

		   selectedProject = fmt.Sprintf("%s-%s", project, suffix)

	   } else {
		   // Step 3: REPL-driven project selection
		   buckets, err := awscloud.ListBucketsWithPrefix(profile, "auto-mock-")
		   if err != nil {
			   return err
		   }

		   var exists bool
		   selectedProject, exists, err = repl.ResolveProjectInteractively(buckets)
		   if err != nil {
			   return err
		   }

		   if exists {
			   action := repl.SelectProjectAction(selectedProject)
			   awsProvider, err := awscloud.NewProvider(profile, selectedProject)
			   if err != nil {
				   return fmt.Errorf("failed to initialize AWS provider: %w", err)
			   }
			   var prov provider.Provider = awsProvider
			   switch action {
			   case "Delete":
				   return prov.DeleteProject()
			   case "Edit":
				   fmt.Println("🛠️ Edit stubs coming soon...")
				   return nil
			   case "Cancel":
				   fmt.Println("❌ Cancelled.")
				   return nil
			   }
		   }
	   }

	// Step 4: Provision project (S3 bucket, etc.)
	if selectedProject == "" {
		return errors.New("internal error: selected project name is empty")
	}

	   awsProvider, err := awscloud.NewProvider(profile, selectedProject)
	   if err != nil {
		   return fmt.Errorf("failed to initialize AWS provider: %w", err)
	   }
	   var prov provider.Provider = awsProvider
	   return prov.InitProject()
}

// checkAWSCredentials verifies if AWS credentials are configured and valid
func checkAWSCredentials(profile string) bool {
   cfg, err := awscloud.LoadAWSConfig(profile)
   if err != nil {
	   return false
   }
   client := sts.NewFromConfig(cfg)
   _, err = client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
   return err == nil
}
