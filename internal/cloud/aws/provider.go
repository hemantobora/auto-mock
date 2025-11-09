// internal/cloud/aws/provider.go
package aws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/cloud/naming"
	"github.com/hemantobora/auto-mock/internal/loadtest"
	"github.com/hemantobora/auto-mock/internal/models"
)

// Provider holds AWS-specific clients and config
type Provider struct {
	projectID  string
	naming     internal.NamingStrategy
	region     string
	BucketName string
	S3Client   *s3.Client
	AWSConfig  aws.Config
}

// ProviderOption is a functional option for provider configuration
type ProviderOption func(*providerOptions)

type providerOptions struct {
	profile string
	region  string
}

// WithRegion specifies the AWS region
func WithRegion(region string) ProviderOption {
	return func(o *providerOptions) {
		o.region = region
	}
}

// WithProfile specifies the AWS profile to use
func WithProfile(profile string) ProviderOption {
	return func(o *providerOptions) {
		o.profile = profile
	}
}

// loadAWSConfig loads AWS configuration with optional profile
func loadAWSConfig(ctx context.Context, profile string) (aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{}
	if profile != "" {
		optFns = append(optFns, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, &models.ProviderError{
			Provider:  "aws",
			Operation: "load-config",
			Resource:  fmt.Sprintf("profile:%s", profile),
			Cause:     fmt.Errorf("failed to load AWS config: %w", err),
		}
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg, nil
}

// NewProvider creates a new S3 storage provider
func NewProvider(ctx context.Context, options ...ProviderOption) (*Provider, error) {
	// Apply options
	opts := &providerOptions{}
	for _, opt := range options {
		opt(opts)
	}

	// Load AWS configuration
	cfg, err := loadAWSConfig(ctx, opts.profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override region if specified
	if opts.region != "" {
		cfg.Region = opts.region
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg)

	// Create provider
	naming := naming.NewDefaultNaming()
	provider := &Provider{
		S3Client:  s3Client,
		naming:    naming,
		region:    cfg.Region,
		AWSConfig: cfg,
	}
	return provider, nil
}

// ValidateCredentials checks if AWS credentials are valid
func ValidateCredentials(ctx context.Context, profile string) (bool, error) {
	cfg, err := loadAWSConfig(ctx, profile)
	if err != nil {
		return false, err
	}

	client := sts.NewFromConfig(cfg)
	_, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return false, err
	}

	return true, nil
}

// GetProviderType returns the provider type
func (p *Provider) GetProviderType() string {
	return "aws"
}

func (p *Provider) ValidateProjectName(projectID string) error {
	return p.naming.ValidateProjectID(projectID)
}

func (p *Provider) GetStorageName() string {
	return p.BucketName
}

func (p *Provider) GetProjectName() string {
	return p.projectID
}

func (p *Provider) SetStorageName(name string) {
	p.BucketName = name
}

func (p *Provider) SetProjectName(name string) {
	p.projectID = name
}

func (p *Provider) GetRegion() string {
	return p.region
}

func (p *Provider) InitProject(ctx context.Context, projectID string) error {
	// Check if bucket exists
	bucketName := p.naming.GenerateStorageName(projectID)
	_, err := p.S3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err == nil {
		// Bucket exists
		p.projectID = projectID
		p.BucketName = bucketName
		// Detect and align region to avoid 301 PermanentRedirect on PutObject
		if loc, lerr := p.S3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucketName)}); lerr == nil {
			resolved := string(loc.LocationConstraint)
			if resolved == "" { // us-east-1 returns empty per API
				resolved = "us-east-1"
			}
			if resolved != "" && resolved != p.region {
				p.region = resolved
				// Rebuild S3 client bound to correct region to prevent redirects
				cfg := p.AWSConfig
				cfg.Region = resolved
				p.S3Client = s3.NewFromConfig(cfg)
			}
		}
		fmt.Printf("✅ Project already initialized: %s\n", projectID)
		return nil
	}

	// Create bucket
	var input *s3.CreateBucketInput
	if p.region == "us-east-1" {
		input = &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		}
	} else {
		input = &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(p.region),
			},
		}
	}

	_, err = p.S3Client.CreateBucket(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "BucketAlreadyOwnedByYou":
				fmt.Printf("✅ Project already initialized: %s\n", projectID)
				return nil
			case "BucketAlreadyExists":
				return &models.ProviderError{
					Provider:  "aws",
					Operation: "init",
					Resource:  bucketName,
					Cause:     fmt.Errorf("bucket name '%s' already taken globally — choose a more unique project name", bucketName),
				}
			}
		}
		return &models.ProviderError{
			Provider:  "aws",
			Operation: "init",
			Resource:  bucketName,
			Cause:     fmt.Errorf("failed to create bucket: %w", err),
		}
	}
	fmt.Println("✅ Project initialized:", projectID)
	p.projectID = projectID
	p.BucketName = bucketName
	return nil
}

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

	hashFile := func(path string) (string, int64, error) {
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
		// Just record presence; we don't parse for now to keep scope small.
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
