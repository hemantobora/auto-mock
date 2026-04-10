package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/hemantobora/auto-mock/internal/loadtest"
	"github.com/hemantobora/auto-mock/internal/models"
)

// ── Config CRUD ───────────────────────────────────────────────────────────────

// SaveConfig serialises a MockConfiguration and writes it to Blob Storage.
// Blob key paths are identical to the S3 layout — only the underlying PUT differs.
func (p *Provider) SaveConfig(ctx context.Context, config *models.MockConfiguration) error {
	if err := models.ValidateConfiguration(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	cleanID := p.naming.ExtractProjectID(p.projectID)
	config.Metadata.ProjectID = cleanID
	config.Metadata.UpdatedAt = time.Now()
	if config.Metadata.CreatedAt.IsZero() {
		config.Metadata.CreatedAt = time.Now()
	}
	if config.Metadata.Version == "" {
		config.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())
	}

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	config.Metadata.Size = int64(len(jsonData))

	if err := p.putBlob(ctx, fmt.Sprintf("configs/%s/current.json", cleanID), jsonData, "application/json"); err != nil {
		return fmt.Errorf("failed to save current config: %w", err)
	}

	versionKey := fmt.Sprintf("configs/%s/versions/%s.json", cleanID, config.Metadata.Version)
	if err := p.putBlob(ctx, versionKey, jsonData, "application/json"); err != nil {
		fmt.Printf("Warning: failed to save version %s: %v\n", config.Metadata.Version, err)
	}

	if err := p.updateMetadataIndex(ctx, cleanID, config.Metadata); err != nil {
		fmt.Printf("Warning: failed to update metadata index: %v\n", err)
	}
	return nil
}

// GetConfig retrieves the current MockConfiguration for a project.
// Returns nil, nil when no config exists yet (project has no expectations).
func (p *Provider) GetConfig(ctx context.Context, projectID string) (*models.MockConfiguration, error) {
	cleanID := p.naming.ExtractProjectID(projectID)
	key := fmt.Sprintf("configs/%s/current.json", cleanID)

	data, err := p.getBlob(ctx, key)
	if err != nil {
		if isBlobNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	var config models.MockConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, nil
}

// UpdateConfig increments the version and saves the configuration.
func (p *Provider) UpdateConfig(ctx context.Context, config *models.MockConfiguration) error {
	existing, err := p.GetConfig(ctx, config.Metadata.ProjectID)
	if err == nil && existing != nil {
		config.Metadata.CreatedAt = existing.Metadata.CreatedAt
	}
	config.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())
	return p.SaveConfig(ctx, config)
}

// DeleteProject removes all mock blobs for the project.
// Mirrors the AWS behaviour: only deletes the container when no other context
// (load-test artifacts, deployed infrastructure) remains active.
func (p *Provider) DeleteProject(projectID string) error {
	cleanID := p.naming.ExtractProjectID(projectID)
	ctx := context.Background()

	_ = p.deleteAllBlobsWithPrefix(ctx, fmt.Sprintf("configs/%s/", cleanID))
	_ = p.deleteSingleBlob(ctx, fmt.Sprintf("metadata/%s.json", cleanID))

	ltBase := strings.TrimSuffix(p.naming.LoadTestBundlesPrefix(cleanID), "bundles/")
	ltExists := p.blobPrefixExists(ctx, ltBase) ||
		p.blobExists(ctx, p.naming.LoadTestMetadataKey(cleanID)) ||
		p.blobExists(ctx, "deployment-metadata-loadtest.json")

	mockDeployed := p.blobExists(ctx, "deployment-metadata.json")
	loadtestDeployed := p.blobExists(ctx, "deployment-metadata-loadtest.json")
	mockGone := !p.blobPrefixExists(ctx, fmt.Sprintf("configs/%s/", cleanID)) &&
		!p.blobExists(ctx, fmt.Sprintf("metadata/%s.json", cleanID))

	if mockGone && !mockDeployed && !ltExists && !loadtestDeployed {
		_ = p.deleteAllBlobsWithPrefix(ctx, "terraform/state/")
		_ = p.deleteAllBlobsWithPrefix(ctx, "terraform/loadtest/state/")

		if !p.containerHasBlobs(ctx) {
			cc, err := p.containerClientFor(p.containerName)
			if err == nil {
				if _, delErr := cc.Delete(ctx, nil); delErr == nil {
					fmt.Printf("✅ Project %q deleted (container removed)\n", cleanID)
					return nil
				}
			}
		}
	}

	fmt.Printf("✅ Project %q mock data deleted (container retained: other context active or deployed)\n", cleanID)
	return nil
}

// ── Versioning ────────────────────────────────────────────────────────────────

func (p *Provider) SaveVersion(ctx context.Context, config *models.MockConfiguration, version string) error {
	cleanID := p.naming.ExtractProjectID(config.Metadata.ProjectID)
	key := fmt.Sprintf("configs/%s/versions/%s.json", cleanID, version)
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	return p.putBlob(ctx, key, jsonData, "application/json")
}

func (p *Provider) GetVersion(ctx context.Context, projectID, version string) (*models.MockConfiguration, error) {
	cleanID := p.naming.ExtractProjectID(projectID)
	key := fmt.Sprintf("configs/%s/versions/%s.json", cleanID, version)
	data, err := p.getBlob(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %s: %w", version, err)
	}
	var config models.MockConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}
	return &config, nil
}

func (p *Provider) ListVersions(ctx context.Context, projectID string) ([]models.VersionInfo, error) {
	cleanID := p.naming.ExtractProjectID(projectID)
	prefix := fmt.Sprintf("configs/%s/versions/", cleanID)

	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return nil, err
	}

	var versions []models.VersionInfo
	pager := cc.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{Prefix: &prefix})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list versions: %w", err)
		}
		for _, item := range page.Segment.BlobItems {
			if item.Name == nil {
				continue
			}
			parts := strings.Split(*item.Name, "/")
			versionName := strings.TrimSuffix(parts[len(parts)-1], ".json")
			var size int64
			var modTime time.Time
			if item.Properties != nil {
				if item.Properties.ContentLength != nil {
					size = *item.Properties.ContentLength
				}
				if item.Properties.LastModified != nil {
					modTime = *item.Properties.LastModified
				}
			}
			versions = append(versions, models.VersionInfo{
				Version:   versionName,
				CreatedAt: modTime,
				Size:      size,
			})
		}
	}
	return versions, nil
}

// ── Metadata ──────────────────────────────────────────────────────────────────

func (p *Provider) GetMetadata(ctx context.Context, projectID string) (*models.ConfigMetadata, error) {
	cleanID := p.naming.ExtractProjectID(projectID)
	metadata, err := p.getMetadataFromIndex(ctx, cleanID)
	if err == nil {
		return metadata, nil
	}
	config, err := p.GetConfig(ctx, cleanID)
	if err != nil {
		return nil, err
	}
	return &config.Metadata, nil
}

func (p *Provider) updateMetadataIndex(ctx context.Context, projectID string, metadata models.ConfigMetadata) error {
	key := fmt.Sprintf("metadata/%s.json", projectID)
	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return p.putBlob(ctx, key, jsonData, "application/json")
}

func (p *Provider) getMetadataFromIndex(ctx context.Context, projectID string) (*models.ConfigMetadata, error) {
	key := fmt.Sprintf("metadata/%s.json", projectID)
	data, err := p.getBlob(ctx, key)
	if err != nil {
		return nil, err
	}
	var metadata models.ConfigMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

// ── Load test bundle operations ───────────────────────────────────────────────
// Blob key paths are identical to the AWS layout; only the PUT/GET calls differ.

func (p *Provider) UploadLoadTestBundle(ctx context.Context, projectID, bundleDir string) (*models.LoadTestPointer, *models.LoadTestVersion, error) {
	if p.containerName == "" {
		base := p.naming.ExtractProjectID(projectID)
		if exists, _ := p.ProjectExists(ctx, base); !exists {
			if err := p.InitProject(ctx, base); err != nil {
				return nil, nil, fmt.Errorf("init project: %w", err)
			}
		}
	}

	baseID := p.naming.ExtractProjectID(projectID)
	required := []string{"locustfile.py", "requirements.txt", "locust_endpoints.json"}
	optional := []string{"user_data.yaml", "manifest.json"}
	found := make(map[string]string)
	hashes := make(map[string]string)
	var missing []string

	for _, name := range required {
		fp := filepath.Join(bundleDir, name)
		if st, err := os.Stat(fp); err == nil && !st.IsDir() {
			if sum, _, err := computeFileHash(fp); err == nil {
				hashes[name] = sum
			}
			found[name] = fp
		} else {
			missing = append(missing, name)
		}
	}
	for _, name := range optional {
		fp := filepath.Join(bundleDir, name)
		if st, err := os.Stat(fp); err == nil && !st.IsDir() {
			if sum, _, err := computeFileHash(fp); err == nil {
				hashes[name] = sum
			}
			found[name] = fp
		}
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("missing required bundle files: %v", missing)
	}

	valRes, _ := loadtest.ValidateBundle(bundleDir)
	validation := &models.LoadTestValidationResult{
		LocustfilePresent:   true,
		RequirementsPresent: true,
		UserDataPresent:     found["user_data.yaml"] != "",
		ManifestPresent:     found["manifest.json"] != "",
		HostDefined:         valRes != nil && valRes.HostDefined,
	}
	if valRes != nil {
		validation.PlaceholderErrors = valRes.PlaceholderErrors
	}

	ts := time.Now().UTC()
	version := fmt.Sprintf("v%d", ts.Unix())
	bundleID := fmt.Sprintf("bndl_%d", ts.UnixNano())

	var fileRefs []models.LoadTestFileRef
	for name, fp := range found {
		st, err := os.Stat(fp)
		if err != nil {
			continue
		}
		fileRefs = append(fileRefs, models.LoadTestFileRef{Name: name, Size: st.Size(), SHA256: hashes[name]})
	}
	manifestWarnings := []string{}
	if !validation.HostDefined {
		manifestWarnings = append(manifestWarnings, "No host defined in locustfile.")
	}
	if len(validation.PlaceholderErrors) > 0 {
		manifestWarnings = append(manifestWarnings, fmt.Sprintf("Found %d unresolved placeholders in user_data.yaml", len(validation.PlaceholderErrors)))
	}
	manifest := &models.LoadTestManifest{
		BundleID:    bundleID,
		ProjectID:   baseID,
		GeneratedAt: ts,
		Files:       fileRefs,
		Entrypoints: []string{"locustfile.py"},
		Warnings:    manifestWarnings,
	}

	metrics := map[string]int{}
	if valRes != nil {
		metrics["tasks"] = valRes.Tasks
		metrics["endpoints"] = valRes.Endpoints
	}
	versionSnap := &models.LoadTestVersion{
		ProjectID:  baseID,
		Version:    version,
		BundleID:   bundleID,
		CreatedAt:  ts,
		Hashes:     hashes,
		Validation: validation,
		Metrics:    metrics,
	}
	filesMap := map[string]string{
		"locustfile":   p.naming.LoadTestBundleFileKey(baseID, bundleID, "locustfile.py"),
		"requirements": p.naming.LoadTestBundleFileKey(baseID, bundleID, "requirements.txt"),
		"endpoints":    p.naming.LoadTestBundleFileKey(baseID, bundleID, "locust_endpoints.json"),
		"user_data":    p.naming.LoadTestBundleFileKey(baseID, bundleID, "user_data.yaml"),
		"manifest":     p.naming.LoadTestBundleFileKey(baseID, bundleID, "manifest.json"),
	}
	pointer := models.NewDefaultLoadTestPointer(baseID, version, bundleID, filesMap,
		&models.LoadTestSummary{Tasks: metrics["tasks"], Endpoints: metrics["endpoints"], HasHost: validation.HostDefined})

	for name, local := range found {
		key := p.naming.LoadTestBundleFileKey(baseID, bundleID, name)
		data, err := os.ReadFile(local)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", name, err)
		}
		if err := p.putBlob(ctx, key, data, "application/octet-stream"); err != nil {
			return nil, nil, fmt.Errorf("upload %s: %w", name, err)
		}
	}

	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	if err := p.putBlob(ctx, p.naming.LoadTestBundleFileKey(baseID, bundleID, "manifest.json"), manifestJSON, "application/json"); err != nil {
		return nil, nil, fmt.Errorf("upload manifest: %w", err)
	}

	versionJSON, _ := json.MarshalIndent(versionSnap, "", "  ")
	if err := p.putBlob(ctx, p.naming.LoadTestVersionKey(baseID, version), versionJSON, "application/json"); err != nil {
		return nil, nil, fmt.Errorf("upload version snapshot: %w", err)
	}

	pointerJSON, _ := json.MarshalIndent(pointer, "", "  ")
	if err := p.putBlob(ctx, p.naming.LoadTestCurrentKey(baseID), pointerJSON, "application/json"); err != nil {
		return nil, nil, fmt.Errorf("upload pointer: %w", err)
	}

	idx := models.LoadTestMetadataIndex{ProjectID: baseID, LatestVersion: version, UpdatedAt: ts}
	idxJSON, _ := json.MarshalIndent(idx, "", "  ")
	_ = p.putBlob(ctx, p.naming.LoadTestMetadataKey(baseID), idxJSON, "application/json")

	return pointer, versionSnap, nil
}

func (p *Provider) GetLoadTestPointer(ctx context.Context, projectID string) (*models.LoadTestPointer, error) {
	baseID := p.naming.ExtractProjectID(projectID)
	data, err := p.getBlob(ctx, p.naming.LoadTestCurrentKey(baseID))
	if err != nil {
		return nil, fmt.Errorf("get loadtest pointer: %w", err)
	}
	var ptr models.LoadTestPointer
	if err := json.Unmarshal(data, &ptr); err != nil {
		return nil, err
	}
	return &ptr, nil
}

func (p *Provider) DownloadLoadTestBundle(ctx context.Context, projectID, destDir string) (*models.LoadTestPointer, string, error) {
	ptr, err := p.GetLoadTestPointer(ctx, projectID)
	if err != nil {
		return nil, "", err
	}
	target := filepath.Join(destDir, ptr.BundleID)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, "", fmt.Errorf("create dir: %w", err)
	}
	for _, key := range ptr.Files {
		if key == "" {
			continue
		}
		data, err := p.getBlob(ctx, key)
		if err != nil {
			return nil, "", fmt.Errorf("download %s: %w", key, err)
		}
		localPath := filepath.Join(target, path.Base(key))
		if err := os.WriteFile(localPath, data, 0o644); err != nil {
			return nil, "", fmt.Errorf("write %s: %w", localPath, err)
		}
	}
	abs, _ := filepath.Abs(target)
	return ptr, abs, nil
}

func (p *Provider) DeleteLoadTestPointer(ctx context.Context, projectID string) error {
	baseID := p.naming.ExtractProjectID(projectID)
	return p.deleteSingleBlob(ctx, p.naming.LoadTestCurrentKey(baseID))
}

func (p *Provider) DeleteActiveLoadTestBundleAndRollback(ctx context.Context, projectID string) (*models.LoadTestPointer, int, error) {
	curPtr, err := p.GetLoadTestPointer(ctx, projectID)
	if err != nil || curPtr == nil || curPtr.ActiveVersion == "" {
		_ = p.DeleteLoadTestPointer(ctx, projectID)
		return nil, 0, nil
	}

	baseID := p.naming.ExtractProjectID(projectID)
	bundlePrefix := p.naming.LoadTestBundleDir(baseID, curPtr.BundleID)
	deleted := p.deleteAllBlobsCount(ctx, bundlePrefix)

	versionsPrefix := fmt.Sprintf("configs/%s-loadtest/versions/", baseID)
	prevKey, err := p.findPreviousVersionKey(ctx, versionsPrefix, curPtr.ActiveVersion)
	if err != nil || prevKey == "" {
		_ = p.DeleteLoadTestPointer(ctx, projectID)
		return nil, deleted, nil
	}

	data, err := p.getBlob(ctx, prevKey)
	if err != nil {
		return nil, deleted, fmt.Errorf("read previous version: %w", err)
	}
	var prevVer models.LoadTestVersion
	if err := json.Unmarshal(data, &prevVer); err != nil {
		return nil, deleted, err
	}

	newPtr := models.NewDefaultLoadTestPointer(baseID, prevVer.Version, prevVer.BundleID,
		map[string]string{
			"locustfile":   p.naming.LoadTestBundleFileKey(baseID, prevVer.BundleID, "locustfile.py"),
			"requirements": p.naming.LoadTestBundleFileKey(baseID, prevVer.BundleID, "requirements.txt"),
			"endpoints":    p.naming.LoadTestBundleFileKey(baseID, prevVer.BundleID, "locust_endpoints.json"),
			"user_data":    p.naming.LoadTestBundleFileKey(baseID, prevVer.BundleID, "user_data.yaml"),
			"manifest":     p.naming.LoadTestBundleFileKey(baseID, prevVer.BundleID, "manifest.json"),
		},
		&models.LoadTestSummary{
			Tasks:    prevVer.Metrics["tasks"],
			Endpoints: prevVer.Metrics["endpoints"],
			HasHost:  prevVer.Validation != nil && prevVer.Validation.HostDefined,
		},
	)
	pointerJSON, _ := json.MarshalIndent(newPtr, "", "  ")
	if err := p.putBlob(ctx, p.naming.LoadTestCurrentKey(baseID), pointerJSON, "application/json"); err != nil {
		return nil, deleted, fmt.Errorf("update pointer: %w", err)
	}
	return newPtr, deleted, nil
}

func (p *Provider) PurgeLoadTestArtifacts(ctx context.Context, projectID string) (int, bool, error) {
	baseID := p.naming.ExtractProjectID(projectID)
	ltID := p.naming.LoadTestProjectID(baseID)
	deleted := 0

	deleted += p.deleteAllBlobsCount(ctx, fmt.Sprintf("configs/%s/", ltID))
	_ = p.deleteSingleBlob(ctx, fmt.Sprintf("metadata/%s.json", ltID))

	ltArtifactsExist := p.blobPrefixExists(ctx, fmt.Sprintf("configs/%s/", ltID)) ||
		p.blobExists(ctx, fmt.Sprintf("metadata/%s.json", ltID))
	mockArtifactsExist := p.blobPrefixExists(ctx, fmt.Sprintf("configs/%s/", baseID)) ||
		p.blobExists(ctx, fmt.Sprintf("metadata/%s.json", baseID))
	mockDeployed := p.blobExists(ctx, "deployment-metadata.json")
	loadtestDeployed := p.blobExists(ctx, "deployment-metadata-loadtest.json")

	containerDeleted := false
	if !ltArtifactsExist && !loadtestDeployed && !mockArtifactsExist && !mockDeployed {
		deleted += p.deleteAllBlobsCount(ctx, "terraform/loadtest/state/")
		deleted += p.deleteAllBlobsCount(ctx, "terraform/state/")

		if !p.containerHasBlobs(ctx) {
			cc, err := p.containerClientFor(p.containerName)
			if err == nil {
				if _, delErr := cc.Delete(ctx, nil); delErr == nil {
					containerDeleted = true
				}
			}
		}
	}
	return deleted, containerDeleted, nil
}

// ── Low-level blob helpers ────────────────────────────────────────────────────

// serviceURL returns the Azure Blob Service endpoint for the storage account.
func (p *Provider) serviceURL() string {
	return fmt.Sprintf("https://%s.blob.core.windows.net", p.accountName)
}

// serviceClient returns a service-level client used for container management
// (create/delete/list containers) and as the parent for container clients.
func (p *Provider) serviceClient() (*service.Client, error) {
	c, err := service.NewClient(p.serviceURL(), p.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob service client: %w", err)
	}
	return c, nil
}

// containerClientFor returns a container.Client for the named container.
// Used for container-level operations (create, delete, list blobs, check existence).
func (p *Provider) containerClientFor(containerName string) (*container.Client, error) {
	svc, err := p.serviceClient()
	if err != nil {
		return nil, err
	}
	return svc.NewContainerClient(containerName), nil
}

// putBlob uploads data to the named blob key in the current container.
func (p *Provider) putBlob(ctx context.Context, key string, data []byte, contentType string) error {
	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return err
	}
	bbc := cc.NewBlockBlobClient(key)
	_, err = bbc.UploadBuffer(ctx, data, &blockblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{BlobContentType: &contentType},
	})
	return err
}

// getBlob downloads the blob at key and returns its bytes.
func (p *Provider) getBlob(ctx context.Context, key string) ([]byte, error) {
	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return nil, err
	}
	resp, err := cc.NewBlobClient(key).DownloadStream(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// deleteSingleBlob removes one blob, silently ignoring "not found".
func (p *Provider) deleteSingleBlob(ctx context.Context, key string) error {
	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return err
	}
	_, err = cc.NewBlobClient(key).Delete(ctx, nil)
	if err != nil && isBlobNotFound(err) {
		return nil
	}
	return err
}

// deleteAllBlobsWithPrefix deletes every blob whose name starts with prefix.
func (p *Provider) deleteAllBlobsWithPrefix(ctx context.Context, prefix string) error {
	p.deleteAllBlobsCount(ctx, prefix)
	return nil
}

// deleteAllBlobsCount deletes blobs with the prefix and returns the count deleted.
func (p *Provider) deleteAllBlobsCount(ctx context.Context, prefix string) int {
	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return 0
	}
	deleted := 0
	pager := cc.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{Prefix: &prefix})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			break
		}
		for _, item := range page.Segment.BlobItems {
			if item.Name == nil {
				continue
			}
			if _, err := cc.NewBlobClient(*item.Name).Delete(ctx, nil); err == nil {
				deleted++
			}
		}
	}
	return deleted
}

// blobExists returns true if the blob at key is present in the current container.
func (p *Provider) blobExists(ctx context.Context, key string) bool {
	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return false
	}
	_, err = cc.NewBlobClient(key).GetProperties(ctx, nil)
	return err == nil
}

// blobPrefixExists returns true if at least one blob with the given prefix exists.
func (p *Provider) blobPrefixExists(ctx context.Context, prefix string) bool {
	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return false
	}
	pager := cc.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{Prefix: &prefix})
	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err == nil && len(page.Segment.BlobItems) > 0 {
			return true
		}
	}
	return false
}

// containerHasBlobs returns true if the current container contains any blobs.
func (p *Provider) containerHasBlobs(ctx context.Context) bool {
	empty := ""
	return p.blobPrefixExists(ctx, empty)
}

// findPreviousVersionKey lists version blob keys and returns the one immediately
// before currentVersion in chronological order (versions are named v<unix-ts>.json).
func (p *Provider) findPreviousVersionKey(ctx context.Context, versionsPrefix, currentVersion string) (string, error) {
	cc, err := p.containerClientFor(p.containerName)
	if err != nil {
		return "", err
	}
	var keys []string
	pager := cc.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{Prefix: &versionsPrefix})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", err
		}
		for _, item := range page.Segment.BlobItems {
			if item.Name != nil {
				keys = append(keys, *item.Name)
			}
		}
	}
	// Keys sort lexicographically = chronologically (v<unix-timestamp>.json).
	// Find the key immediately before the current version key.
	currentKey := versionsPrefix + currentVersion + ".json"
	var prev string
	for _, k := range keys {
		if k == currentKey {
			break
		}
		prev = k
	}
	return prev, nil
}

// isBlobNotFound returns true when the error indicates a 404 / blob-not-found response.
func isBlobNotFound(err error) bool {
	if err == nil {
		return false
	}
	return bloberror.HasCode(err, bloberror.BlobNotFound) ||
		bloberror.HasCode(err, bloberror.ContainerNotFound) ||
		strings.Contains(err.Error(), "BlobNotFound") ||
		strings.Contains(err.Error(), "404")
}
