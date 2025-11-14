package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hemantobora/auto-mock/internal/loadtest"
	"github.com/hemantobora/auto-mock/internal/models"
)

// UploadLoadTestBundle uploads a generated load test bundle directory to cloud storage and updates pointer & version files.
// bundleDir must contain at minimum: locustfile.py, requirements.txt, locust_endpoints.json
// user_data.yaml is optional; manifest.json is optional (we generate it if absent)
func (p *Provider) UploadLoadTestBundle(ctx context.Context, projectID, bundleDir string) (*models.LoadTestPointer, *models.LoadTestVersion, error) {
	// Ensure bucket context is established (supports REPL uploads without prior init)
	if p.BucketName == "" {
		base := p.naming.ExtractProjectID(projectID)
		if exists, _ := p.ProjectExists(ctx, base); !exists {
			if err := p.InitProject(ctx, base); err != nil {
				return nil, nil, fmt.Errorf("init project: %w", err)
			}
		} else {
			// Align region with existing bucket
			if loc, err := p.S3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(p.BucketName)}); err == nil {
				resolved := string(loc.LocationConstraint)
				if resolved == "" {
					resolved = "us-east-1"
				}
				if resolved != p.region && resolved != "" {
					cfg := p.AWSConfig
					cfg.Region = resolved
					p.S3Client = s3.NewFromConfig(cfg)
					p.region = resolved
				}
			}
		}
	}
	// Defensive: ensure S3 client region matches existing bucket to avoid 301 redirect
	if p.BucketName != "" {
		if loc, err := p.S3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(p.BucketName)}); err == nil {
			resolved := string(loc.LocationConstraint)
			if resolved == "" {
				resolved = "us-east-1"
			}
			if resolved != p.region && resolved != "" {
				cfg := p.AWSConfig
				cfg.Region = resolved
				p.S3Client = s3.NewFromConfig(cfg)
				p.region = resolved
			}
		}
	}
	baseID := p.naming.ExtractProjectID(projectID)

	// Collect files we care about
	// Ensure the core Locust spec (locust_endpoints.json) is always uploaded alongside the runner.
	required := []string{"locustfile.py", "requirements.txt", "locust_endpoints.json"}
	optional := []string{"user_data.yaml", "manifest.json"}
	found := make(map[string]string)
	hashes := make(map[string]string)
	var missing []string

	// use helper in loadtest_helpers.go
	hashFile := hashFile

	// Scan required
	for _, name := range required {
		fp := filepath.Join(bundleDir, name)
		if st, err := os.Stat(fp); err == nil && !st.IsDir() {
			sum, _, herr := hashFile(fp)
			if herr == nil {
				hashes[name] = sum
			}
			found[name] = fp
		} else {
			missing = append(missing, name)
		}
	}
	// Optional
	for _, name := range optional {
		fp := filepath.Join(bundleDir, name)
		if st, err := os.Stat(fp); err == nil && !st.IsDir() {
			sum, _, herr := hashFile(fp)
			if herr == nil {
				hashes[name] = sum
			}
			found[name] = fp
		}
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("missing required bundle files: %v", missing)
	}

	// Enhanced validation using validator utility
	valRes, _ := loadtest.ValidateBundle(bundleDir)
	validation := &models.LoadTestValidationResult{
		LocustfilePresent:   true,
		RequirementsPresent: true,
		UserDataPresent:     found["user_data.yaml"] != "",
		ManifestPresent:     found["manifest.json"] != "",
		HostDefined:         valRes != nil && valRes.HostDefined,
		PlaceholderErrors:   nil,
	}
	if valRes != nil {
		validation.PlaceholderErrors = valRes.PlaceholderErrors
	}

	ts := time.Now().UTC()
	version := fmt.Sprintf("v%d", ts.Unix())
	bundleID := fmt.Sprintf("bndl_%d", ts.UnixNano())

	// S3 key helpers
	versionKey := p.naming.LoadTestVersionKey(baseID, version)
	pointerKey := p.naming.LoadTestCurrentKey(baseID)
	bundlePrefix := p.naming.LoadTestBundleDir(baseID, bundleID)
	metadataKey := p.naming.LoadTestMetadataKey(baseID)

	// Prepare manifest (generate if absent)
	var manifest *models.LoadTestManifest
	if mfPath, ok := found["manifest.json"]; ok {
		_ = mfPath
	}
	// Build manifest from discovered files
	var fileRefs []models.LoadTestFileRef
	for name, path := range found {
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		fileRefs = append(fileRefs, models.LoadTestFileRef{Name: name, Size: st.Size(), SHA256: hashes[name]})
	}
	manifestWarnings := []string{}
	if !validation.HostDefined {
		manifestWarnings = append(manifestWarnings, "No host defined in locustfile; specify 'host =' or set via CLI when running Locust.")
	}
	if len(validation.PlaceholderErrors) > 0 {
		manifestWarnings = append(manifestWarnings, fmt.Sprintf("Found %d unresolved placeholders in user_data.yaml", len(validation.PlaceholderErrors)))
	}
	manifest = &models.LoadTestManifest{
		BundleID:    bundleID,
		ProjectID:   baseID,
		GeneratedAt: ts,
		Files:       fileRefs,
		Entrypoints: []string{"locustfile.py"},
		Warnings:    manifestWarnings,
	}

	// Build version snapshot
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

	// Build pointer (include endpoints JSON so consumers/downloader get the full bundle)
	pointer := models.NewDefaultLoadTestPointer(baseID, version, bundleID, map[string]string{
		"locustfile":   p.naming.LoadTestBundleFileKey(baseID, bundleID, "locustfile.py"),
		"requirements": p.naming.LoadTestBundleFileKey(baseID, bundleID, "requirements.txt"),
		"endpoints":    p.naming.LoadTestBundleFileKey(baseID, bundleID, "locust_endpoints.json"),
		"user_data":    p.naming.LoadTestBundleFileKey(baseID, bundleID, "user_data.yaml"),
		"manifest":     p.naming.LoadTestBundleFileKey(baseID, bundleID, "manifest.json"),
	}, &models.LoadTestSummary{Tasks: metrics["tasks"], Endpoints: metrics["endpoints"], HasHost: validation.HostDefined})
	// Upload bundle files
	for name, local := range found {
		key := p.naming.LoadTestBundleFileKey(baseID, bundleID, name)
		data, err := os.ReadFile(local)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", name, err)
		}
		if err := p.putObject(ctx, key, data, "application/octet-stream"); err != nil {
			return nil, nil, fmt.Errorf("upload %s: %w", name, err)
		}
	}
	// Upload generated manifest.json (override if existed)
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	if err := p.putObject(ctx, p.naming.LoadTestBundleFileKey(baseID, bundleID, "manifest.json"), manifestJSON, "application/json"); err != nil {
		return nil, nil, fmt.Errorf("upload manifest: %w", err)
	}
	// Upload version snapshot
	versionJSON, _ := json.MarshalIndent(versionSnap, "", "  ")
	if err := p.putObject(ctx, versionKey, versionJSON, "application/json"); err != nil {
		return nil, nil, fmt.Errorf("upload version snapshot: %w", err)
	}
	// Upload pointer (current.json)
	pointerJSON, _ := json.MarshalIndent(pointer, "", "  ")
	if err := p.putObject(ctx, pointerKey, pointerJSON, "application/json"); err != nil {
		return nil, nil, fmt.Errorf("upload pointer: %w", err)
	}
	// Update metadata index (best effort)
	idx := models.LoadTestMetadataIndex{ProjectID: baseID, LatestVersion: version, BundleCount: 0, UpdatedAt: ts}
	idxJSON, _ := json.MarshalIndent(idx, "", "  ")
	_ = p.putObject(ctx, metadataKey, idxJSON, "application/json")

	_ = bundlePrefix // prefix reserved for possible listing operations later
	return pointer, versionSnap, nil
}

// GetLoadTestPointer retrieves the current load test pointer (current.json) from S3 for a project
func (p *Provider) GetLoadTestPointer(ctx context.Context, projectID string) (*models.LoadTestPointer, error) {
	baseID := p.naming.ExtractProjectID(projectID)
	key := p.naming.LoadTestCurrentKey(baseID)
	out, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(p.BucketName), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("get loadtest pointer: %w", err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}
	var ptr models.LoadTestPointer
	if err := json.Unmarshal(data, &ptr); err != nil {
		return nil, err
	}
	return &ptr, nil
}

// DownloadLoadTestBundle downloads the active load test bundle files into destDir/<bundleID>
// Returns the pointer and the absolute local directory path containing the files
func (p *Provider) DownloadLoadTestBundle(ctx context.Context, projectID, destDir string) (*models.LoadTestPointer, string, error) {
	ptr, err := p.GetLoadTestPointer(ctx, projectID)
	if err != nil {
		return nil, "", err
	}
	// target directory
	target := filepath.Join(destDir, ptr.BundleID)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, "", fmt.Errorf("create dir: %w", err)
	}
	// iterate files map (logical name -> s3 key)
	for _, key := range ptr.Files {
		if key == "" {
			continue
		}
		obj, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(p.BucketName), Key: aws.String(key)})
		if err != nil {
			return nil, "", fmt.Errorf("download %s: %w", key, err)
		}
		fname := path.Base(key)
		localPath := filepath.Join(target, fname)
		f, err := os.Create(localPath)
		if err != nil {
			obj.Body.Close()
			return nil, "", fmt.Errorf("create %s: %w", localPath, err)
		}
		if _, err := io.Copy(f, obj.Body); err != nil {
			f.Close()
			obj.Body.Close()
			return nil, "", fmt.Errorf("write %s: %w", localPath, err)
		}
		f.Close()
		obj.Body.Close()
	}
	abs, _ := filepath.Abs(target)
	return ptr, abs, nil
}

// DeleteLoadTestPointer removes the current.json pointer (does not delete bundles)
func (p *Provider) DeleteLoadTestPointer(ctx context.Context, projectID string) error {
	baseID := p.naming.ExtractProjectID(projectID)
	key := p.naming.LoadTestCurrentKey(baseID)
	_, err := p.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(p.BucketName), Key: aws.String(key)})
	return err
}

// DeleteActiveLoadTestBundleAndRollback deletes the currently active bundle directory and attempts to
// roll back the pointer to the previous version if one exists. If no previous version exists, the
// pointer file is deleted. Returns the new pointer (nil if removed entirely) and count of deleted bundle objects.
func (p *Provider) DeleteActiveLoadTestBundleAndRollback(ctx context.Context, projectID string) (*models.LoadTestPointer, int, error) {
	// Load current pointer
	curPtr, err := p.GetLoadTestPointer(ctx, projectID)
	if err != nil || curPtr == nil || curPtr.ActiveVersion == "" {
		// Nothing active; just delete pointer if present
		_ = p.DeleteLoadTestPointer(ctx, projectID)
		return nil, 0, nil
	}

	baseID := p.naming.ExtractProjectID(projectID)
	// Delete bundle objects
	deleted, derr := p.deleteBundleObjects(ctx, baseID, curPtr.BundleID)
	if derr != nil {
		return nil, deleted, derr
	}

	// Find previous version
	versionsPrefix := fmt.Sprintf("configs/%s-loadtest/versions/", baseID)
	versions, verr := p.listVersionKeys(ctx, baseID)
	if verr != nil {
		return nil, deleted, verr
	}
	prevKey := previousVersionKey(versions, versionsPrefix, curPtr.ActiveVersion)
	if prevKey == "" { // no previous; delete pointer
		_ = p.DeleteLoadTestPointer(ctx, projectID)
		return nil, deleted, nil
	}
	// Load previous version and rewrite pointer
	prevVer, lerr := p.loadVersion(ctx, prevKey)
	if lerr != nil {
		return nil, deleted, fmt.Errorf("read previous version: %w", lerr)
	}
	newPtr, werr := p.writePointerForVersion(ctx, prevVer)
	if werr != nil {
		return nil, deleted, fmt.Errorf("update pointer: %w", werr)
	}
	return newPtr, deleted, nil
}

// PurgeLoadTestArtifacts deletes all load test objects (bundles, versions, pointer, metadata) for the project.
// Returns count of deleted object versions and whether the underlying bucket was deleted.
func (p *Provider) PurgeLoadTestArtifacts(ctx context.Context, projectID string) (int, bool, error) {
	baseID := p.naming.ExtractProjectID(projectID)
	ltID := p.naming.LoadTestProjectID(baseID)
	deleted := 0

	// Delete configs/<project>-loadtest/* and metadata/<project>-loadtest.json
	deleted += p.deleteAllVersionsWithPrefix(ctx, fmt.Sprintf("configs/%s/", ltID))
	deleted += p.deleteAllVersionsForKey(ctx, fmt.Sprintf("metadata/%s.json", ltID))

	// Evaluate cleanup conditions to avoid Terraform state drift.
	// We only delete Terraform state (both loadtest and mock) and the bucket if BOTH contexts have:
	// - no artifacts (configs/* or metadata/*.json) AND
	// - no deployment metadata present.
	// This prevents deleting Terraform state while any stack is still deployed.
	mk := int32(1)

	// Check remaining loadtest artifacts
	ltArtifactsExist := false
	if list, _ := p.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.BucketName),
		Prefix:  aws.String(fmt.Sprintf("configs/%s/", ltID)),
		MaxKeys: &mk,
	}); list != nil && len(list.Contents) > 0 {
		ltArtifactsExist = true
	}
	if !ltArtifactsExist {
		if _, err := p.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(p.BucketName),
			Key:    aws.String(fmt.Sprintf("metadata/%s.json", ltID)),
		}); err == nil {
			ltArtifactsExist = true
		}
	}

	// Check remaining mock artifacts
	mockArtifactsExist := false
	if list, _ := p.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.BucketName),
		Prefix:  aws.String(fmt.Sprintf("configs/%s/", baseID)),
		MaxKeys: &mk,
	}); list != nil && len(list.Contents) > 0 {
		mockArtifactsExist = true
	}
	if !mockArtifactsExist {
		if _, err := p.S3Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(p.BucketName), Key: aws.String(fmt.Sprintf("metadata/%s.json", baseID))}); err == nil {
			mockArtifactsExist = true
		}
	}

	// Check deployment metadata presence for both contexts
	mockDeployed := false
	if _, err := p.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String("deployment-metadata.json"),
	}); err == nil {
		mockDeployed = true
	}
	loadtestDeployed := false
	if _, err := p.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String("deployment-metadata-loadtest.json"),
	}); err == nil {
		loadtestDeployed = true
	}

	bucketDeleted := false

	// Only when nothing remains in both contexts and nothing is deployed, remove Terraform state and maybe the bucket.
	if !ltArtifactsExist && !loadtestDeployed && !mockArtifactsExist && !mockDeployed {
		// Safe to remove both stacks' Terraform state
		deleted += p.deleteAllVersionsWithPrefix(ctx, "terraform/loadtest/state/")
		deleted += p.deleteAllVersionsWithPrefix(ctx, "terraform/state/")

		// If the bucket is now empty, delete it
		if rem, _ := p.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(p.BucketName), MaxKeys: &mk}); rem != nil && len(rem.Contents) == 0 {
			if _, err := p.S3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(p.BucketName)}); err == nil {
				bucketDeleted = true
			}
		}
	}

	return deleted, bucketDeleted, nil
}
