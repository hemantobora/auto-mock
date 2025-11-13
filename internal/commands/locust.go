package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/client"
	"github.com/hemantobora/auto-mock/internal/cloud"
	"github.com/hemantobora/auto-mock/internal/models"
)

// Keeps CLI wiring lean by encapsulating provider detection and project checks here.
// edit: download existing bundle for modification (no upload unless reUpload chosen)
// deletePtr: remove current pointer
func RunLocust(profile, project string, options client.Options, upload, download, deletePtr, purge bool) error {
	// If no action flags and no collection input, enter REPL mode for load test management
	if !upload && !download && !deletePtr && !purge {
		fmt.Println("Entering load test management REPL...")
		// Generate bundle (local only). Done after project verification if uploading.
		if err := client.GenerateLoadtestBundle(options); err != nil {
			return err
		}
		return nil
	}

	manager := cloud.NewCloudManager(profile)
	// Use a non-nil context for all provider operations
	ctx := context.Background()
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}
	// return repl.StartLoadTestREPL(manager.Provider, project)
	if project == "" {
		return fmt.Errorf("--project is required for this operation")
	}
	exists, _ := manager.Provider.ProjectExists(ctx, project)
	if !exists {
		if deletePtr || download || purge {
			return fmt.Errorf("project '%s' does not exist", project)
		}
		if err := manager.Provider.InitProject(ctx, project); err != nil {
			return fmt.Errorf("failed to init project for upload: %w", err)
		}
	}

	// Delete pointer flow
	if deletePtr {
		return handleDeletePointer(ctx, manager.Provider, project)
	}

	if purge {
		return handlePurgeBundle(ctx, manager.Provider, &models.LoadTestPointer{ProjectID: project})
	}

	// Edit flow: download current bundle before generation (skip generation entirely)
	if download {
		// create download workspace dir
		downloadBase := fmt.Sprintf("loadtest_download_%d", time.Now().Unix())
		if options.OutDir == "" {
			options.OutDir = downloadBase
		}
		ptr, localDir, err := manager.Provider.DownloadLoadTestBundle(ctx, project, options.OutDir)
		if err != nil {
			return fmt.Errorf("download bundle: %w", err)
		}
		fmt.Printf("\n📦 Downloaded active bundle (version %s) to: %s\n", ptr.ActiveVersion, localDir)
		return nil
	}

	if upload {
		ll := models.NewLoader(os.Stdout, "Uploading load test scripts")
		ll.Start()
		defer ll.StopWithMessage("Upload complete")
		pointer, versionSnap, err := manager.Provider.UploadLoadTestBundle(ctx, project, options.OutDir)
		if err != nil {
			return err
		}
		fmt.Println("\n✅ Load test bundle uploaded:")
		b, _ := json.MarshalIndent(struct {
			Pointer *models.LoadTestPointer `json:"pointer"`
			Version *models.LoadTestVersion `json:"version"`
		}{pointer, versionSnap}, "", "  ")
		fmt.Println(string(b))
	}
	return nil
}

// handleDeletePointer removes current pointer and associated bundle, rolling back to previous version if available
func handleDeletePointer(ctx context.Context, provider internal.Provider, project string) error {
	// Confirm destructive action based on current pointer
	ll := models.NewLoader(os.Stdout, "Deleting load test active pointer, if any")
	curPtr, _ := provider.GetLoadTestPointer(ctx, project)
	if curPtr == nil || curPtr.ActiveVersion == "" {
		var sure bool
		_ = survey.AskOne(&survey.Confirm{Message: "No active bundle. Delete current pointer file if present?", Default: false}, &sure)
		if !sure {
			return nil
		}
		if err := provider.DeleteLoadTestPointer(ctx, project); err != nil {
			return err
		}
		fmt.Println("\n✅ Deleted current pointer.")
		return nil
	}

	var confirm bool
	_ = survey.AskOne(&survey.Confirm{Message: fmt.Sprintf("Delete bundle %s and roll back pointer to previous version?", curPtr.BundleID), Default: false}, &confirm)
	if !confirm {
		return nil
	}

	ll.Start()
	defer ll.StopWithMessage("Delete complete")
	newPtr, deleted, err := provider.DeleteActiveLoadTestBundleAndRollback(ctx, project)
	if err != nil {
		return err
	}
	if newPtr == nil {
		fmt.Printf("✅ Deleted bundle (%d objects) and removed pointer (no previous version).\n", deleted)
		return nil
	}
	fmt.Printf("✅ Deleted bundle (%d objects) and rolled back pointer to %s (%s)\n", deleted, newPtr.ActiveVersion, newPtr.BundleID)
	return nil
}

// handlePurgeBundle deletes all loadtest-related objects for a project
func handlePurgeBundle(ctx context.Context, provider internal.Provider, ptr *models.LoadTestPointer) error {
	ll := models.NewLoader(os.Stdout, "Purging load test artifacts")
	var confirm bool
	_ = survey.AskOne(&survey.Confirm{Message: "This will delete all loadtest bundles, versions, pointer, and index. Continue?", Default: false}, &confirm)
	if !confirm {
		return nil
	}
	var typed string
	_ = survey.AskOne(&survey.Input{Message: "Type 'permanently delete' to confirm:"}, &typed)
	if strings.TrimSpace(typed) != "permanently delete" {
		return fmt.Errorf("confirmation mismatch")
	}

	ll.Start()
	defer ll.StopWithMessage("Purge complete")
	deleted, bucketRemoved, err := provider.PurgeLoadTestArtifacts(ctx, ptr.ProjectID)
	if err != nil {
		return err
	}
	if bucketRemoved {
		fmt.Printf("\n✅ Purged load test artifacts. Deleted ~%d versions; project bucket removed (no remaining mock or other objects).\n", deleted)
		return nil
	}
	fmt.Printf("\n✅ Purged load test artifacts. Deleted ~%d object versions; bucket retained.\n", deleted)
	return nil
}
