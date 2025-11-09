package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/client"
	"github.com/hemantobora/auto-mock/internal/cloud"
	"github.com/hemantobora/auto-mock/internal/models"
	"github.com/hemantobora/auto-mock/internal/repl"
)

// Keeps CLI wiring lean by encapsulating provider detection and project checks here.
// edit: download existing bundle for modification (no upload unless reUpload chosen)
// deletePtr: remove current pointer
func RunLocust(profile, project string, options client.Options, upload, edit, deletePtr bool) error {
	// If no action flags and no collection input, enter REPL mode for load test management
	if !upload && !edit && !deletePtr && options.CollectionPath == "" {
		manager := cloud.NewCloudManager(profile)
		if err := manager.AutoDetectProvider(profile); err != nil {
			return err
		}
		return repl.StartLoadTestREPL(manager.Provider, project)
	}

	// When uploading we align with "init" flow: detect provider & ensure project first.
	var manager *cloud.CloudManager
	var ctx context.Context
	if upload || edit || deletePtr {
		if project == "" {
			return fmt.Errorf("--project is required for this operation")
		}
		manager = cloud.NewCloudManager(profile)
		if err := manager.AutoDetectProvider(profile); err != nil {
			return err
		}
		ctx = context.Background()
		exists, _ := manager.Provider.ProjectExists(ctx, project)
		if !exists {
			if err := manager.Provider.InitProject(ctx, project); err != nil {
				return fmt.Errorf("failed to init project for upload: %w", err)
			}
		}
	}

	// Delete pointer flow
	if deletePtr {
		if err := manager.Provider.DeleteLoadTestPointer(ctx, project); err != nil {
			return fmt.Errorf("delete pointer: %w", err)
		}
		fmt.Println("✅ Deleted load test current pointer. Versions & bundles remain intact.")
		return nil
	}

	// Edit flow: download current bundle before generation (skip generation entirely)
	if edit {
		// create edit workspace dir
		editBase := fmt.Sprintf("loadtest_edit_%d", time.Now().Unix())
		if options.OutDir == "" {
			options.OutDir = editBase
		}
		ptr, localDir, err := manager.Provider.DownloadLoadTestBundle(ctx, project, options.OutDir)
		if err != nil {
			return fmt.Errorf("download bundle: %w", err)
		}
		fmt.Printf("\n📦 Downloaded active bundle (version %s) to: %s\n", ptr.ActiveVersion, localDir)
		fmt.Println("Edit the files, then re-upload with: automock locust --project", project, "--upload --dir", localDir)
		// Optional immediate re-upload prompt
		var reupload bool
		prompt := &survey.Confirm{Message: "Re-upload immediately as a new version?"}
		if err := survey.AskOne(prompt, &reupload); err == nil && reupload {
			// use existing directory, perform upload
			pointer, versionSnap, err := manager.Provider.UploadLoadTestBundle(ctx, project, localDir)
			if err != nil {
				return fmt.Errorf("re-upload failed: %w", err)
			}
			fmt.Println("\n✅ Re-uploaded new version:")
			b, _ := json.MarshalIndent(struct {
				Pointer *models.LoadTestPointer `json:"pointer"`
				Version *models.LoadTestVersion `json:"version"`
			}{pointer, versionSnap}, "", "  ")
			fmt.Println(string(b))
		}
		return nil
	}

	// Generate bundle (local only). Done after project verification if uploading.
	if err := client.GenerateLoadtestBundle(options); err != nil {
		return err
	}

	if upload {
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
