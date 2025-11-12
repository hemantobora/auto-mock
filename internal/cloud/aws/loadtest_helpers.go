package aws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	core "github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/models"
)

// hashFile computes a sha256 digest and returns it in the format "sha256:<hex>" with the byte count.
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), n, nil
}

// buildBundleFilesMap returns the standard logical-name to object-key mapping for a bundle.
func buildBundleFilesMap(n core.NamingStrategy, projectID, bundleID string) map[string]string {
	return map[string]string{
		"locustfile":   n.LoadTestBundleFileKey(projectID, bundleID, "locustfile.py"),
		"requirements": n.LoadTestBundleFileKey(projectID, bundleID, "requirements.txt"),
		"endpoints":    n.LoadTestBundleFileKey(projectID, bundleID, "locust_endpoints.json"),
		"user_data":    n.LoadTestBundleFileKey(projectID, bundleID, "user_data.yaml"),
		"manifest":     n.LoadTestBundleFileKey(projectID, bundleID, "manifest.json"),
	}
}

// deleteBundleObjects removes all objects under the bundle directory prefix and returns the number deleted.
func (p *Provider) deleteBundleObjects(ctx context.Context, baseID, bundleID string) (int, error) {
	prefix := p.naming.LoadTestBundleDir(baseID, bundleID)
	deleted := 0
	var token *string
	for {
		resp, err := p.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(p.BucketName), Prefix: aws.String(prefix), ContinuationToken: token})
		if err != nil {
			return deleted, fmt.Errorf("list bundle objects: %w", err)
		}
		if len(resp.Contents) == 0 {
			break
		}
		for _, obj := range resp.Contents {
			if _, derr := p.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(p.BucketName), Key: obj.Key}); derr == nil {
				deleted++
			}
		}
		if (resp.IsTruncated != nil && *resp.IsTruncated) && resp.NextContinuationToken != nil {
			token = resp.NextContinuationToken
			continue
		}
		break
	}
	return deleted, nil
}

// listVersionKeys returns all version file keys under versions/ for the project.
func (p *Provider) listVersionKeys(ctx context.Context, baseID string) ([]string, error) {
	versionsPrefix := fmt.Sprintf("configs/%s-loadtest/versions/", baseID)
	var versions []string
	var token *string
	for {
		resp, err := p.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(p.BucketName), Prefix: aws.String(versionsPrefix), ContinuationToken: token})
		if err != nil {
			return nil, fmt.Errorf("list versions: %w", err)
		}
		for _, obj := range resp.Contents {
			versions = append(versions, aws.ToString(obj.Key))
		}
		if (resp.IsTruncated != nil && *resp.IsTruncated) && resp.NextContinuationToken != nil {
			token = resp.NextContinuationToken
			continue
		}
		break
	}
	return versions, nil
}

// previousVersionKey sorts keys descending and returns the immediate predecessor to currentVersion.
func previousVersionKey(versions []string, versionsPrefix, currentVersion string) string {
	sort.Slice(versions, func(i, j int) bool { return versions[i] > versions[j] })
	currentKey := fmt.Sprintf("%s%s.json", versionsPrefix, currentVersion)
	for _, k := range versions {
		if k < currentKey {
			return k
		}
	}
	return ""
}

// loadVersion reads and unmarshals a LoadTestVersion given its key.
func (p *Provider) loadVersion(ctx context.Context, key string) (*models.LoadTestVersion, error) {
	obj, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(p.BucketName), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	defer obj.Body.Close()
	data, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, err
	}
	var ver models.LoadTestVersion
	if err := json.Unmarshal(data, &ver); err != nil {
		return nil, err
	}
	return &ver, nil
}

// writePointerForVersion constructs and writes current.json for the given version and returns the pointer.
func (p *Provider) writePointerForVersion(ctx context.Context, ver *models.LoadTestVersion) (*models.LoadTestPointer, error) {
	files := buildBundleFilesMap(p.naming, ver.ProjectID, ver.BundleID)
	ptr := models.NewDefaultLoadTestPointer(ver.ProjectID, ver.Version, ver.BundleID, files, &models.LoadTestSummary{
		Tasks:     ver.Metrics["tasks"],
		Endpoints: ver.Metrics["endpoints"],
		HasHost:   ver.Validation != nil && ver.Validation.HostDefined,
	})
	b, _ := json.MarshalIndent(ptr, "", "  ")
	pointerKey := p.naming.LoadTestCurrentKey(ver.ProjectID)
	if err := p.putObject(ctx, pointerKey, b, "application/json"); err != nil {
		return nil, err
	}
	return ptr, nil
}

// deleteAllVersionsWithPrefix deletes all versions and delete markers under a prefix.
func (p *Provider) deleteAllVersionsWithPrefix(ctx context.Context, prefix string) int {
	deleted := 0
	pager := s3.NewListObjectVersionsPaginator(p.S3Client, &s3.ListObjectVersionsInput{Bucket: aws.String(p.BucketName), Prefix: aws.String(prefix)})
	for pager.HasMorePages() {
		page, _ := pager.NextPage(ctx)
		var objs []types.ObjectIdentifier
		for _, v := range page.Versions {
			objs = append(objs, types.ObjectIdentifier{Key: v.Key, VersionId: v.VersionId})
		}
		for _, m := range page.DeleteMarkers {
			objs = append(objs, types.ObjectIdentifier{Key: m.Key, VersionId: m.VersionId})
		}
		if len(objs) > 0 {
			if _, err := p.S3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: aws.String(p.BucketName), Delete: &types.Delete{Objects: objs, Quiet: aws.Bool(true)}}); err == nil {
				deleted += len(objs)
			}
		}
	}
	return deleted
}

// deleteAllVersionsForKey deletes all versions and markers for a single key.
func (p *Provider) deleteAllVersionsForKey(ctx context.Context, key string) int {
	deleted := 0
	pager := s3.NewListObjectVersionsPaginator(p.S3Client, &s3.ListObjectVersionsInput{Bucket: aws.String(p.BucketName), Prefix: aws.String(key)})
	for pager.HasMorePages() {
		page, _ := pager.NextPage(ctx)
		var objs []types.ObjectIdentifier
		for _, v := range page.Versions {
			if v.Key != nil && aws.ToString(v.Key) == key {
				objs = append(objs, types.ObjectIdentifier{Key: v.Key, VersionId: v.VersionId})
			}
		}
		for _, m := range page.DeleteMarkers {
			if m.Key != nil && aws.ToString(m.Key) == key {
				objs = append(objs, types.ObjectIdentifier{Key: m.Key, VersionId: m.VersionId})
			}
		}
		if len(objs) > 0 {
			if _, err := p.S3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: aws.String(p.BucketName), Delete: &types.Delete{Objects: objs, Quiet: aws.Bool(true)}}); err == nil {
				deleted += len(objs)
			}
		}
	}
	return deleted
}
