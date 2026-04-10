// Package azure provides an Azure Blob Storage implementation of the Provider interface.
// Storage layout (mirrors the S3 layout exactly — only the top-level resource changes):
//
//	Storage Account : automock<8-char-suffix>   (one per subscription, shared across projects)
//	Container       : auto-mock-<projectid>     (one per project, = GetStorageName())
//	Blob paths      : configs/<id>/current.json, metadata/<id>.json, terraform/state/, …
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/cloud/naming"
	"github.com/hemantobora/auto-mock/internal/models"
)

const (
	defaultLocation      = "eastus"
	defaultResourceGroup = "auto-mock-rg"

	// accountNamePrefix is the prefix for the shared Azure Storage Account.
	// Storage account names: max 24 chars, lowercase alphanumeric only (no hyphens).
	accountNamePrefix = "automock"

	// containerNamePrefix is the prefix for per-project Blob Containers.
	// Container names allow hyphens, so this mirrors the S3 bucket naming convention.
	containerNamePrefix = "auto-mock-"
)

// Provider holds Azure-specific state and implements internal.Provider.
type Provider struct {
	projectID      string
	naming         internal.NamingStrategy
	location       string // Azure region, e.g. "eastus"
	subscriptionID string
	resourceGroup  string
	accountName    string // Storage Account name, e.g. "automock1a2b3c4d"
	containerName  string // Blob Container name, e.g. "auto-mock-myproject" — returned by GetStorageName()
	credential     *azidentity.DefaultAzureCredential
}

// ProviderOption is a functional option for NewProvider.
type ProviderOption func(*providerOptions)

type providerOptions struct {
	location       string
	resourceGroup  string
	subscriptionID string
}

// WithLocation overrides the default Azure region ("eastus").
func WithLocation(location string) ProviderOption {
	return func(o *providerOptions) { o.location = location }
}

// WithResourceGroup overrides the default resource group ("auto-mock-rg").
func WithResourceGroup(rg string) ProviderOption {
	return func(o *providerOptions) { o.resourceGroup = rg }
}

// WithSubscriptionID pins a specific subscription instead of auto-discovering.
func WithSubscriptionID(id string) ProviderOption {
	return func(o *providerOptions) { o.subscriptionID = id }
}

// NewProvider creates and initialises an Azure Blob Storage provider.
// It will:
//  1. Load credentials via DefaultAzureCredential (honours `az login`).
//  2. Discover the active subscription from ~/.azure/azureProfile.json.
//  3. Ensure the resource group exists (creates it if missing and permitted).
//  4. Ensure a shared storage account exists (creates one if none found).
func NewProvider(ctx context.Context, options ...ProviderOption) (*Provider, error) {
	opts := &providerOptions{
		location:      defaultLocation,
		resourceGroup: defaultResourceGroup,
	}
	for _, opt := range options {
		opt(opts)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("Azure credential not found — run 'az login' first: %w", err)
	}

	subID := opts.subscriptionID
	if subID == "" {
		subID, err = discoverSubscriptionID()
		if err != nil {
			return nil, err
		}
	}

	p := &Provider{
		naming:         naming.NewDefaultNaming(),
		location:       opts.location,
		subscriptionID: subID,
		resourceGroup:  opts.resourceGroup,
		credential:     cred,
	}

	if err := p.ensureResourceGroup(ctx); err != nil {
		return nil, err
	}
	if err := p.ensureStorageAccount(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

// ValidateCredentials checks that Azure credentials are available and returns the
// active subscription ID. Equivalent to AWS STS GetCallerIdentity.
func ValidateCredentials(ctx context.Context) (string, error) {
	// Discovering the subscription ID from the Azure CLI profile is sufficient:
	// if `az login` has not been run, the profile file will not exist.
	subID, err := discoverSubscriptionID()
	if err != nil {
		return "", fmt.Errorf("Azure credentials not found — run 'az login' first: %w", err)
	}
	// Also confirm the SDK can load a credential (catches env-var misconfigurations).
	if _, err := azidentity.NewDefaultAzureCredential(nil); err != nil {
		return "", fmt.Errorf("failed to initialise Azure credential: %w", err)
	}
	return subID, nil
}

// ── Provider interface: identity ─────────────────────────────────────────────

func (p *Provider) GetProviderType() string { return "azure" }
func (p *Provider) GetRegion() string       { return p.location }

// GetBackendConfig returns an azurerm backend block for Terraform.
func (p *Provider) GetBackendConfig(stateKey string) string {
	return fmt.Sprintf(`terraform {
  backend "azurerm" {
    resource_group_name  = "%s"
    storage_account_name = "%s"
    container_name       = "%s"
    key                  = "%s"
  }
}
`, p.resourceGroup, p.accountName, p.containerName, stateKey)
}

// ── Provider interface: project / storage names ──────────────────────────────

func (p *Provider) GetProjectName() string     { return p.projectID }
func (p *Provider) SetProjectName(name string) { p.projectID = name }
func (p *Provider) GetStorageName() string     { return p.containerName }
func (p *Provider) SetStorageName(name string) { p.containerName = name }

func (p *Provider) ValidateProjectName(projectID string) error {
	return p.naming.ValidateProjectID(projectID)
}

// ── Provider interface: project lifecycle ────────────────────────────────────

// InitProject ensures the Blob Container for the project exists, creating it if needed.
// Equivalent to creating an S3 bucket.
func (p *Provider) InitProject(ctx context.Context, projectID string) error {
	containerName := containerNamePrefix + projectID

	cc, err := p.containerClientFor(containerName)
	if err != nil {
		return err
	}

	_, err = cc.Create(ctx, nil)
	if err != nil {
		// Swallow "already exists" — same behaviour as AWS BucketAlreadyOwnedByYou
		if strings.Contains(err.Error(), "ContainerAlreadyExists") {
			fmt.Printf("✅ Project already initialised: %s\n", projectID)
			p.projectID = projectID
			p.containerName = containerName
			return nil
		}
		return &models.ProviderError{
			Provider:  "azure",
			Operation: "init",
			Resource:  containerName,
			Cause:     fmt.Errorf("failed to create container: %w", err),
		}
	}

	fmt.Printf("✅ Project initialised: %s\n", projectID)
	p.projectID = projectID
	p.containerName = containerName
	return nil
}

// ListProjects returns all projects found as Blob Containers in the storage account.
func (p *Provider) ListProjects(ctx context.Context) ([]models.ProjectInfo, error) {
	fmt.Println("✅ Checking existence of projects")

	svc, err := p.serviceClient()
	if err != nil {
		return nil, err
	}

	var projects []models.ProjectInfo
	pager := svc.NewListContainersPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers: %w", err)
		}
		for _, item := range page.ContainerItems {
			if item.Name == nil {
				continue
			}
			name := *item.Name
			if !strings.HasPrefix(name, containerNamePrefix) {
				continue
			}
			projectID := p.naming.ExtractProjectID(name)
			if projectID == "" {
				continue
			}
			projects = append(projects, models.ProjectInfo{
				ProjectID:   projectID,
				DisplayName: projectID,
				StorageName: name,
			})
		}
	}
	return projects, nil
}

// ProjectExists checks whether a project's Blob Container exists.
func (p *Provider) ProjectExists(ctx context.Context, projectID string) (bool, error) {
	fmt.Printf("✅ Checking existence of project: %s\n", projectID)
	containerName := containerNamePrefix + projectID

	cc, err := p.containerClientFor(containerName)
	if err != nil {
		return false, err
	}
	_, err = cc.GetProperties(ctx, nil)
	if err != nil {
		return false, nil // container does not exist
	}
	p.containerName = containerName
	p.projectID = projectID
	return true, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// discoverSubscriptionID reads the active (default) subscription ID from the
// Azure CLI profile at ~/.azure/azureProfile.json, which is written by `az login`.
func discoverSubscriptionID() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	profilePath := filepath.Join(home, ".azure", "azureProfile.json")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return "", fmt.Errorf(
			"Azure CLI profile not found (%s) — run 'az login' first: %w",
			profilePath, err,
		)
	}

	var profile struct {
		Subscriptions []struct {
			ID        string `json:"id"`
			IsDefault bool   `json:"isDefault"`
			State     string `json:"state"`
		} `json:"subscriptions"`
	}
	// Strip UTF-8 BOM (0xEF 0xBB 0xBF) if present — Azure CLI on some
	// platforms writes the profile with a BOM that json.Unmarshal rejects.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	if err := json.Unmarshal(data, &profile); err != nil {
		return "", fmt.Errorf("failed to parse Azure CLI profile: %w", err)
	}

	// Prefer the subscription marked as default.
	for _, sub := range profile.Subscriptions {
		if sub.IsDefault && sub.State == "Enabled" {
			return sub.ID, nil
		}
	}
	// Fall back to the first enabled subscription.
	for _, sub := range profile.Subscriptions {
		if sub.State == "Enabled" {
			return sub.ID, nil
		}
	}
	return "", fmt.Errorf(
		"no enabled Azure subscription found in CLI profile — run 'az login' first",
	)
}

// ensureResourceGroup creates the resource group if it does not already exist.
// If creation fails due to permissions, it prints a clear manual-creation hint.
func (p *Provider) ensureResourceGroup(ctx context.Context) error {
	rgClient, err := armresources.NewResourceGroupsClient(p.subscriptionID, p.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource-group client: %w", err)
	}

	_, err = rgClient.Get(ctx, p.resourceGroup, nil)
	if err == nil {
		return nil // already exists
	}

	fmt.Printf("📦 Resource group '%s' not found — creating in %s\n",
		p.resourceGroup, p.location)

	location := p.location
	done := make(chan struct{})
	var createErr error
	go func() {
		defer close(done)
		_, createErr = rgClient.CreateOrUpdate(ctx, p.resourceGroup, armresources.ResourceGroup{
			Location: &location,
		}, nil)
	}()
	showSpinner(fmt.Sprintf("Creating resource group '%s'", p.resourceGroup), done)

	if createErr != nil {
		return fmt.Errorf(
			"failed to create resource group '%s': %w\n\n"+
				"💡 If you don't have permission, create it manually:\n"+
				"   az group create --name %s --location %s",
			p.resourceGroup, createErr,
			p.resourceGroup, p.location,
		)
	}
	fmt.Printf("✅ Resource group '%s' created\n", p.resourceGroup)
	return nil
}

// ensureStorageAccount finds or creates the shared Storage Account.
// The account name is auto-generated as "automock<8-char-suffix>".
func (p *Provider) ensureStorageAccount(ctx context.Context) error {
	accountsClient, err := armstorage.NewAccountsClient(p.subscriptionID, p.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage-accounts client: %w", err)
	}

	// Search for an existing AutoMock storage account in the resource group.
	pager := accountsClient.NewListByResourceGroupPager(p.resourceGroup, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			break // ignore listing errors; fall through to creation
		}
		for _, account := range page.Value {
			if account.Name != nil && strings.HasPrefix(*account.Name, accountNamePrefix) {
				p.accountName = *account.Name
				return nil
			}
		}
	}

	// None found — create a new one.
	newName := generateAccountName()
	fmt.Printf("📦 Creating storage account '%s' in resource group '%s'\n",
		newName, p.resourceGroup)

	kind := armstorage.KindStorageV2
	skuName := armstorage.SKUNameStandardLRS
	allowPublic := false
	location := p.location

	poller, err := accountsClient.BeginCreate(ctx, p.resourceGroup, newName,
		armstorage.AccountCreateParameters{
			Kind:     &kind,
			Location: &location,
			SKU:      &armstorage.SKU{Name: &skuName},
			Properties: &armstorage.AccountPropertiesCreateParameters{
				AllowBlobPublicAccess: &allowPublic,
			},
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin storage account creation: %w", err)
	}

	done := make(chan struct{})
	var pollErr error
	go func() {
		defer close(done)
		_, pollErr = poller.PollUntilDone(ctx, nil)
	}()
	showSpinner(fmt.Sprintf("Creating storage account '%s' (this may take ~60s)", newName), done)

	if pollErr != nil {
		return fmt.Errorf("failed to create storage account '%s': %w", newName, pollErr)
	}

	p.accountName = newName
	fmt.Printf("✅ Storage account '%s' created\n", p.accountName)
	return nil
}

// generateAccountName produces a unique storage account name.
// Format: "automock" + 8 random alphanumeric chars = 16 chars total (well under 24-char limit).
func generateAccountName() string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	suffix := make([]byte, 8)
	for i := range suffix {
		suffix[i] = charset[rng.Intn(len(charset))]
	}
	return accountNamePrefix + string(suffix)
}
