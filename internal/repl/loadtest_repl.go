package repl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	core "github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/client"
	"github.com/hemantobora/auto-mock/internal/models"
	"github.com/hemantobora/auto-mock/internal/prompts"
	"github.com/hemantobora/auto-mock/internal/terraform"
)

// StartLoadTestREPL provides an interactive menu for managing Locust load test bundles.
// It supports generating a bundle, uploading, editing the current bundle, deleting pointer, and viewing status.
// If project is empty, it will prompt to select or create a project.
func StartLoadTestREPL(provider core.Provider, project string) error {
	ctx := context.Background()

	// Ensure project context (select or create)
	if strings.TrimSpace(project) == "" {
		projects, _ := provider.ListProjects(ctx)
		selected, err := ResolveProjectInteractively(projects)
		if err != nil {
			return err
		}
		if selected.ProjectID == "" {
			// create new
			var name string
			if err := survey.AskOne(&survey.Input{Message: "Enter new project name:"}, &name, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			if err := provider.InitProject(ctx, name); err != nil {
				return fmt.Errorf("init project: %w", err)
			}
			project = name
		} else {
			project = selected.ProjectID
		}
	}

	for {
		// probe pointer
		ptr, _ := provider.GetLoadTestPointer(ctx, project)
		hasActive := ptr != nil && ptr.ActiveVersion != ""

		// Build menu options with user-friendly labels & internal keys (first token before space)
		options := []string{
			"generate-local  – Generate a new bundle from a collection file",
			"upload-local-dir – Upload an existing local bundle directory",
			"view-status     – Show current pointer summary (if any)",
		}
		if hasActive {
			options = append(options,
				"edit-current    – Download & optionally re-upload active bundle",
				"delete-pointer  – Remove current pointer (keep versions)",
				"purge-bundle    – Delete active bundle files (destructive)",
			)
		}
		// Loadtest infra lifecycle actions
		isDeployed := false
		if md, err := provider.GetLoadTestDeploymentMetadata(); err == nil && md != nil && md.DeploymentStatus == "deployed" {
			isDeployed = true
		}
		if isDeployed {
			options = append(options,
				"destroy-loadtest – Destroy Locust infrastructure",
				"scale-workers   – Update desired worker count (terraform)",
				"show-deploy     – Show current Locust deployment metadata",
			)
		} else {
			options = append(options,
				"deploy-loadtest – Deploy Locust infrastructure",
			)
		}
		options = append(options, "exit            – Leave LoadTest REPL")

		fmt.Println("\nℹ️  Actions: generate → optional upload, upload-local-dir → direct cloud storage push, edit-current → modify & version bump.")
		fmt.Println("    Dry-run prompts let you simulate uploads without persisting objects.")

		var choice string
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("LoadTest REPL — project: %s", project),
			Options: options,
			Default: options[0],
		}, &choice); err != nil {
			return err
		}

		// Normalize to internal key (first token before space)
		choice = strings.Split(strings.TrimSpace(choice), " ")[0]

		switch choice {
		case "generate-local":
			if err := handleGenerateLocal(ctx, provider, project); err != nil {
				fmt.Printf("❌ Generation failed: %v\n", err)
			}
		case "upload-local-dir":
			if err := handleUploadLocalDir(ctx, provider, project); err != nil {
				fmt.Printf("❌ Upload failed: %v\n", err)
			}
		case "edit-current":
			if !hasActive {
				fmt.Println("⚠️  No active bundle pointer found.")
				break
			}
			if err := handleEditCurrent(ctx, provider, project); err != nil {
				fmt.Printf("❌ Edit failed: %v\n", err)
			}
		case "delete-pointer":
			if !hasActive {
				fmt.Println("⚠️  No active pointer to delete.")
				break
			}
			if err := handleDeletePointer(ctx, provider, project); err != nil {
				fmt.Printf("❌ Delete failed: %v\n", err)
			}
		case "purge-bundle":
			if !hasActive {
				fmt.Println("⚠️  No active bundle to purge.")
				break
			}
			if err := handlePurgeBundle(ctx, provider, ptr); err != nil {
				fmt.Printf("❌ Purge failed: %v\n", err)
			}
		case "view-status":
			ptr, err := provider.GetLoadTestPointer(ctx, project)
			if err != nil || ptr == nil {
				fmt.Println("ℹ️  No active pointer.")
				break
			}
			b, _ := json.MarshalIndent(ptr, "", "  ")
			fmt.Println(string(b))
		case "deploy-loadtest":
			if err := handleLoadTestDeploy(ctx, provider, project); err != nil {
				fmt.Printf("❌ Deploy failed: %v\n", err)
			}
		case "destroy-loadtest":
			if err := handleLoadTestDestroy(ctx, provider, project); err != nil {
				fmt.Printf("❌ Destroy failed: %v\n", err)
			}
		case "scale-workers":
			if err := handleLoadTestScaleWorkers(ctx, provider, project); err != nil {
				fmt.Printf("❌ Scale failed: %v\n", err)
			}
		case "show-deploy":
			if err := handleLoadTestShowDeployment(ctx, provider, project); err != nil {
				fmt.Printf("❌ Show failed: %v\n", err)
			}
		case "exit":
			return nil
		}
	}
}

// handleGenerateLocal generates a new load test bundle from a collection file
func handleGenerateLocal(ctx context.Context, provider core.Provider, project string) error {
	var answers struct {
		CollectionFile string `survey:"collectionFile"`
		CollectionType string `survey:"collectionType"`
		OutDir         string `survey:"outDir"`
	}
	_ = survey.Ask([]*survey.Question{
		{Name: "collectionFile", Prompt: &survey.Input{Message: "Collection file (Postman/Bruno/Insomnia path):"}},
		{Name: "collectionType", Prompt: &survey.Select{Message: "Collection type:", Options: []string{"postman", "bruno", "insomnia"}, Default: "postman"}},
		{Name: "outDir", Prompt: &survey.Input{Message: "Output directory:", Default: fmt.Sprintf("loadtest_%d", time.Now().Unix())}},
	}, &answers)

	// Build options; set safe defaults for pointer fields
	headless := false
	distributed := false
	opts := client.Options{
		CollectionType:             answers.CollectionType,
		CollectionPath:             answers.CollectionFile,
		OutDir:                     answers.OutDir,
		Headless:                   &headless,
		GenerateDistributedHelpers: &distributed,
	}
	if err := client.GenerateLoadtestBundle(opts); err != nil {
		return err
	}
	fmt.Printf("✅ Bundle generated at: %s\n", answers.OutDir)

	var doUpload bool
	_ = survey.AskOne(&survey.Confirm{Message: "Upload this bundle now?", Default: false}, &doUpload)
	if doUpload {
		pointer, version, err := provider.UploadLoadTestBundle(ctx, project, answers.OutDir)
		if err != nil {
			return err
		}
		fmt.Println("✅ Uploaded:")
		b, _ := json.MarshalIndent(struct {
			Pointer *models.LoadTestPointer `json:"pointer"`
			Version *models.LoadTestVersion `json:"version"`
		}{pointer, version}, "", "  ")
		fmt.Println(string(b))
	}
	return nil
}

// handleUploadLocalDir uploads an existing local bundle directory
func handleUploadLocalDir(ctx context.Context, provider core.Provider, project string) error {
	var dir string
	_ = survey.AskOne(&survey.Input{Message: "Directory to upload:", Default: "./loadtest"}, &dir)
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("no directory specified")
	}
	if !filepath.IsAbs(dir) {
		cwd, _ := os.Getwd()
		dir = filepath.Join(cwd, dir)
	}
	pointer, version, err := provider.UploadLoadTestBundle(ctx, project, dir)
	if err != nil {
		return err
	}
	fmt.Println("✅ Uploaded:")
	b, _ := json.MarshalIndent(struct {
		Pointer *models.LoadTestPointer `json:"pointer"`
		Version *models.LoadTestVersion `json:"version"`
	}{pointer, version}, "", "  ")
	fmt.Println(string(b))
	return nil
}

// handleEditCurrent downloads the active bundle for editing and optionally re-uploads
func handleEditCurrent(ctx context.Context, provider core.Provider, project string) error {
	workdir := fmt.Sprintf("loadtest_edit_%d", time.Now().Unix())
	ptr, localDir, err := provider.DownloadLoadTestBundle(ctx, project, workdir)
	if err != nil {
		return err
	}
	fmt.Printf("📦 Downloaded active bundle (version %s) to: %s\n", ptr.ActiveVersion, localDir)

	var re bool
	_ = survey.AskOne(&survey.Confirm{Message: "Re-upload now as a new version?", Default: false}, &re)
	if re {
		pointer, version, err := provider.UploadLoadTestBundle(ctx, project, localDir)
		if err != nil {
			return err
		}
		fmt.Println("✅ Re-uploaded:")
		b, _ := json.MarshalIndent(struct {
			Pointer *models.LoadTestPointer `json:"pointer"`
			Version *models.LoadTestVersion `json:"version"`
		}{pointer, version}, "", "  ")
		fmt.Println(string(b))
	}
	return nil
}

// handleDeletePointer removes current pointer and associated bundle, rolling back to previous version if available
func handleDeletePointer(ctx context.Context, provider core.Provider, project string) error {
	// Confirm destructive action based on current pointer
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
		fmt.Println("✅ Deleted current pointer.")
		return nil
	}

	var confirm bool
	_ = survey.AskOne(&survey.Confirm{Message: fmt.Sprintf("Delete bundle %s and roll back pointer to previous version?", curPtr.BundleID), Default: false}, &confirm)
	if !confirm {
		return nil
	}

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
func handlePurgeBundle(ctx context.Context, provider core.Provider, ptr *models.LoadTestPointer) error {
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

	deleted, bucketRemoved, err := provider.PurgeLoadTestArtifacts(ctx, ptr.ProjectID)
	if err != nil {
		return err
	}
	if bucketRemoved {
		fmt.Printf("✅ Purged load test artifacts. Deleted ~%d versions; project bucket removed (no remaining mock or other objects).\n", deleted)
		return nil
	}
	fmt.Printf("✅ Purged load test artifacts. Deleted ~%d object versions; bucket retained.\n", deleted)
	return nil
}

// === Loadtest infrastructure lifecycle handlers ===

// handleLoadTestDeploy deploys Locust infrastructure
func handleLoadTestDeploy(ctx context.Context, provider core.Provider, projectName string) error {
	fmt.Println("🚀 Deploying Locust infrastructure...")
	if md, err := provider.GetLoadTestDeploymentMetadata(); err == nil && md != nil && md.DeploymentStatus == "deployed" {
		fmt.Println("ℹ️  Locust infra already deployed. Use scale-workers or destroy.")
		return nil
	}

	opts := &models.LoadTestDeploymentOptions{WorkerDesiredCount: 0}

	// Collect BYO options using shared prompts package
	if err := prompts.PromptAllBYOOptions(opts); err != nil {
		return err
	}

	mgr, err := terraform.NewLoadTestManager(projectName, "", provider)
	if err != nil {
		return err
	}
	out, err := mgr.Deploy(opts)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Locust deployed: ALB=%s MasterFQDN=%s Workers=%d\n", out.ALBDNSName, out.CloudMapMasterFQDN, out.WorkerDesiredCount)
	return nil
}

// handleLoadTestDestroy tears down Locust infrastructure
func handleLoadTestDestroy(ctx context.Context, provider core.Provider, projectName string) error {
	fmt.Println("💥 Destroying Locust infrastructure...")
	mgr, err := terraform.NewLoadTestManager(projectName, "", provider)
	if err != nil {
		return err
	}
	if err := mgr.Destroy(); err != nil {
		return err
	}
	fmt.Println("✅ Locust infrastructure destroyed")
	return nil
}

// handleLoadTestScaleWorkers updates worker desired count
func handleLoadTestScaleWorkers(ctx context.Context, provider core.Provider, projectName string) error {
	md, err := provider.GetLoadTestDeploymentMetadata()
	if err != nil || md == nil || md.DeploymentStatus != "deployed" {
		return fmt.Errorf("Locust infrastructure not deployed")
	}

	var desiredStr string
	_ = survey.AskOne(&survey.Input{Message: "Enter desired worker count:"}, &desiredStr)
	desiredStr = strings.TrimSpace(desiredStr)
	if desiredStr == "" {
		return fmt.Errorf("no worker count provided")
	}

	mgr, err := terraform.NewLoadTestManager(projectName, "", provider)
	if err != nil {
		return err
	}

	n, convErr := strconv.Atoi(desiredStr)
	if convErr != nil {
		return fmt.Errorf("invalid worker count: %s", desiredStr)
	}

	if err := mgr.ScaleWorkers(n); err != nil {
		return fmt.Errorf("scale via terraform failed: %w", err)
	}
	fmt.Println("✅ Scaled workers to:", n)
	return nil
}

// handleLoadTestShowDeployment displays current Locust deployment metadata
func handleLoadTestShowDeployment(ctx context.Context, provider core.Provider, projectName string) error {
	md, err := provider.GetLoadTestDeploymentMetadata()
	if err != nil {
		return fmt.Errorf("fetch metadata: %w", err)
	}
	if md == nil {
		fmt.Println("No Locust deployment metadata found.")
		return nil
	}
	b, _ := json.MarshalIndent(md, "", "  ")
	fmt.Println(string(b))
	return nil
}
