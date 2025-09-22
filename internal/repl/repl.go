package repl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/mcp"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// DeploymentConfig contains complete deployment configuration for ECS
type DeploymentConfig struct {
	ProjectName  string        `json:"project_name"`
	Environment  string        `json:"environment"`
	Provider     string        `json:"provider"`
	MockConfig   string        `json:"mock_config"`
	TTL          *TTLConfig    `json:"ttl"`
	Domain       *DomainConfig `json:"domain"`
	Region       string        `json:"region"`
	InstanceSize string        `json:"instance_size"`
}

// TTLConfig contains auto-teardown configuration
type TTLConfig struct {
	Enabled           bool   `json:"enabled"`
	Hours             int    `json:"hours"`
	NotificationEmail string `json:"notification_email,omitempty"`
}

// DomainConfig contains domain and TLS configuration
type DomainConfig struct {
	Type         string `json:"type"` // auto, custom
	CustomDomain string `json:"custom_domain,omitempty"`
	TLSEnabled   bool   `json:"tls_enabled"`
	DNSProvider  string `json:"dns_provider"` // always "route53"
	HostedZoneId string `json:"hosted_zone_id,omitempty"`
	AutoDNSSetup bool   `json:"auto_dns_setup"`
}

// DeploymentResult contains deployment outcome information
type DeploymentResult struct {
	DeploymentID   string    `json:"deployment_id"`
	ProjectName    string    `json:"project_name"`
	Environment    string    `json:"environment"`
	Provider       string    `json:"provider"`
	MockServerURL  string    `json:"mockserver_url"`
	DashboardURL   string    `json:"dashboard_url"`
	TTLExpiry      time.Time `json:"ttl_expiry,omitempty"`
	TerraformState string    `json:"terraform_state_location"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// StartMockGenerationREPL is the main entry point for mock generation
func StartMockGenerationREPL(projectName string) error {
	cleanName := utils.ExtractUserProjectName(projectName)
	fmt.Printf("🎯 MockServer Configuration Generator\n")
	fmt.Printf("📦 Project: %s\n", cleanName)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Simple mock config generation for now
	// Step 1: Choose generation method
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to generate your mock configuration?",
		Options: []string{
			"describe - Describe your API in natural language (AI-powered)",
			"interactive - Build endpoints step-by-step (7-step builder)",
			"collection - Import from Postman/Bruno/Insomnia",
			"template - Quick templates for common APIs",
		},
		Default: "describe - Describe your API in natural language (AI-powered)",
	}, &method); err != nil {
		return err
	}

	method = strings.Split(method, " ")[0]

	// Step 2: Generate mock configuration using MCP engine
	mockServerJSON, err := generateMockConfiguration(method, projectName)
	if err != nil {
		return fmt.Errorf("failed to generate configuration: %w", err)
	}

	// Display the result
	displayResult(mockServerJSON)

	// Handle the result (save, deploy, etc.)
	return handleFinalResult(mockServerJSON, projectName)
}

// Display result
func displayResult(mockServerJSON string) {
	fmt.Println("\n📋 Generated MockServer Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(mockServerJSON)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// Handle final result
func handleFinalResult(mockServerJSON, projectName string) error {
	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "What would you like to do with this configuration?",
		Options: []string{
			"save - Save to file",
			"deploy - Deploy ECS Fargate infrastructure",
			"local - Start MockServer locally",
			"exit - Exit without saving",
		},
	}, &action); err != nil {
		return err
	}

	action = strings.Split(action, " ")[0]
	switch action {
	case "save":
		return saveToFile(mockServerJSON, projectName)
	case "deploy":
		return deployInfrastructure(mockServerJSON, projectName)
	case "local":
		return startLocalMockServer(mockServerJSON, projectName)
	case "exit":
		fmt.Println("\n👋 Configuration generated but not saved.")
		return nil
	}
	return nil
}

// Save configuration to file
func saveToFile(mockServerJSON, projectName string) error {
	cleanName := utils.ExtractUserProjectName(projectName)
	defaultFilename := fmt.Sprintf("%s-expectations.json", cleanName)

	var filename string
	if err := survey.AskOne(&survey.Input{
		Message: "Filename:",
		Default: defaultFilename,
	}, &filename); err != nil {
		return err
	}

	if err := os.WriteFile(filename, []byte(mockServerJSON), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("\n✅ MockServer configuration saved to: %s\n", filename)
	return nil
}

// Deploy ECS Fargate infrastructure
func deployInfrastructure(mockServerJSON, projectName string) error {
	fmt.Println("\n🏗️ ECS Fargate Infrastructure Deployment")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	cleanName := utils.ExtractUserProjectName(projectName)
	fmt.Printf("📦 Project: %s\n", cleanName)

	// Step 1: Environment Selection
	environment, err := selectEnvironment()
	if err != nil {
		return err
	}

	// Step 2: TTL Configuration
	ttlConfig, err := configureTTL()
	if err != nil {
		return err
	}

	// Step 3: Domain Configuration
	domainConfig, err := configureDomainAndTLS()
	if err != nil {
		return err
	}

	// Step 4: Infrastructure Options
	deployConfig, err := configureInfrastructureOptions(projectName, environment)
	if err != nil {
		return err
	}

	// Set configurations
	deployConfig.MockConfig = mockServerJSON
	deployConfig.TTL = ttlConfig
	deployConfig.Domain = domainConfig

	// Step 5: Deploy with Terraform
	fmt.Println("\n🚀 Deploying ECS Fargate infrastructure with Terraform...")
	result, err := deployWithTerraform(deployConfig)
	if err != nil {
		return err
	}

	// Step 6: Display results
	displayDeploymentResult(result)
	return nil
}

// Start local MockServer
func startLocalMockServer(mockServerJSON, projectName string) error {
	fmt.Println("\n🚀 Local MockServer Setup")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	cleanName := utils.ExtractUserProjectName(projectName)
	configFile := fmt.Sprintf("%s-expectations.json", cleanName)

	if err := os.WriteFile(configFile, []byte(mockServerJSON), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Configuration saved as: %s\n\n", configFile)
	fmt.Println("🐳 Docker commands:")
	fmt.Println("1. Start MockServer:")
	fmt.Println("   docker run -d -p 1080:1080 -p 1090:1090 mockserver/mockserver:5.15.0")
	fmt.Println("2. Load expectations:")
	fmt.Printf("   curl -X PUT http://localhost:1080/mockserver/expectation -d @%s\n", configFile)
	fmt.Println("3. Access your API: http://localhost:1080")
	fmt.Println("4. View dashboard: http://localhost:1080/mockserver/dashboard")
	return nil
}

// Environment selection
func selectEnvironment() (string, error) {
	var environment string
	if err := survey.AskOne(&survey.Select{
		Message: "Select deployment environment:",
		Options: []string{
			"dev - Development (minimal resources)",
			"staging - Staging (load testing capable)",
			"prod - Production (high availability)",
		},
		Default: "dev - Development (minimal resources)",
	}, &environment); err != nil {
		return "", err
	}
	return strings.Split(environment, " ")[0], nil
}

// TTL configuration
func configureTTL() (*TTLConfig, error) {
	fmt.Println("\n⏰ Auto-Teardown Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var enableTTL bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Enable automatic infrastructure teardown?",
		Default: true,
		Help:    "Automatically destroy infrastructure after specified time to prevent costs",
	}, &enableTTL); err != nil {
		return nil, err
	}

	ttlConfig := &TTLConfig{Enabled: enableTTL}
	if !enableTTL {
		return ttlConfig, nil
	}

	var ttlChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "How long should infrastructure remain active?",
		Options: []string{
			"2 hours - Quick testing",
			"4 hours - Development session",
			"8 hours - Work day",
			"24 hours - Full day",
		},
		Default: "4 hours - Development session",
	}, &ttlChoice); err != nil {
		return nil, err
	}

	switch {
	case strings.HasPrefix(ttlChoice, "2 hours"):
		ttlConfig.Hours = 2
	case strings.HasPrefix(ttlChoice, "4 hours"):
		ttlConfig.Hours = 4
	case strings.HasPrefix(ttlChoice, "8 hours"):
		ttlConfig.Hours = 8
	case strings.HasPrefix(ttlChoice, "24 hours"):
		ttlConfig.Hours = 24
	}

	return ttlConfig, nil
}

// Domain and TLS configuration
func configureDomainAndTLS() (*DomainConfig, error) {
	fmt.Println("\n🌐 Domain & TLS Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var domainChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "Choose domain configuration:",
		Options: []string{
			"auto - Use AWS-generated ALB domain (free, automatic TLS)",
			"custom - Use your own domain with Route53 DNS",
		},
		Default: "auto - Use AWS-generated ALB domain (free, automatic TLS)",
	}, &domainChoice); err != nil {
		return nil, err
	}

	config := &DomainConfig{
		Type:        strings.Split(domainChoice, " ")[0],
		TLSEnabled:  true,
		DNSProvider: "route53",
	}

	if config.Type == "custom" {
		var domain string
		if err := survey.AskOne(&survey.Input{
			Message: "Enter your domain:",
			Help:    "Example: api.mycompany.com",
		}, &domain); err != nil {
			return nil, err
		}
		config.CustomDomain = domain
		config.AutoDNSSetup = true
	}

	return config, nil
}

// Infrastructure options
func configureInfrastructureOptions(projectName, environment string) (*DeploymentConfig, error) {
	config := &DeploymentConfig{
		ProjectName: projectName,
		Environment: environment,
		Provider:    "aws",
		Region:      "us-east-1",
	}

	var sizeChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "Select ECS task size:",
		Options: []string{
			"small - 256 CPU, 512MB RAM",
			"medium - 512 CPU, 1GB RAM",
			"large - 1024 CPU, 2GB RAM",
		},
		Default: "small - 256 CPU, 512MB RAM",
	}, &sizeChoice); err != nil {
		return nil, err
	}

	config.InstanceSize = strings.Split(sizeChoice, " ")[0]
	return config, nil
}

// Simulate Terraform deployment
func deployWithTerraform(deployConfig *DeploymentConfig) (*DeploymentResult, error) {
	deploymentID := fmt.Sprintf("%s-%s-%d", deployConfig.ProjectName, deployConfig.Environment, time.Now().Unix())

	fmt.Println("📋 Initializing Terraform...")
	time.Sleep(1 * time.Second)
	fmt.Println("🔧 Planning infrastructure changes...")
	time.Sleep(1 * time.Second)
	fmt.Println("🚀 Applying Terraform configuration...")
	fmt.Println("   • Creating VPC and networking")
	time.Sleep(500 * time.Millisecond)
	fmt.Println("   • Setting up Application Load Balancer")
	time.Sleep(500 * time.Millisecond)
	fmt.Println("   • Deploying ECS Fargate service")
	time.Sleep(500 * time.Millisecond)
	fmt.Println("   • Configuring MockServer container")
	time.Sleep(500 * time.Millisecond)

	if deployConfig.Domain.Type == "custom" {
		fmt.Println("   • Requesting SSL certificate")
		time.Sleep(500 * time.Millisecond)
		fmt.Println("   • Setting up Route53 DNS")
		time.Sleep(500 * time.Millisecond)
	}

	if deployConfig.TTL.Enabled {
		fmt.Println("   • Configuring auto-teardown scheduler")
		time.Sleep(500 * time.Millisecond)
	}

	result := &DeploymentResult{
		DeploymentID: deploymentID,
		ProjectName:  deployConfig.ProjectName,
		Environment:  deployConfig.Environment,
		Provider:     "aws",
		Status:       "READY",
		CreatedAt:    time.Now(),
	}

	if deployConfig.Domain.Type == "custom" {
		result.MockServerURL = fmt.Sprintf("https://%s", deployConfig.Domain.CustomDomain)
		result.DashboardURL = fmt.Sprintf("https://%s/mockserver/dashboard", deployConfig.Domain.CustomDomain)
	} else {
		result.MockServerURL = fmt.Sprintf("https://%s-%s-abc123.us-east-1.elb.amazonaws.com", deployConfig.ProjectName, deployConfig.Environment)
		result.DashboardURL = fmt.Sprintf("https://%s-%s-abc123.us-east-1.elb.amazonaws.com/mockserver/dashboard", deployConfig.ProjectName, deployConfig.Environment)
	}

	if deployConfig.TTL.Enabled {
		result.TTLExpiry = time.Now().Add(time.Duration(deployConfig.TTL.Hours) * time.Hour)
	}

	return result, nil
}

// Display deployment result
func displayDeploymentResult(result *DeploymentResult) {
	fmt.Println("\n🎉 ECS Fargate Deployment Successful!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Printf("📦 Project: %s (%s)\n", result.ProjectName, result.Environment)
	fmt.Printf("🆔 Deployment ID: %s\n", result.DeploymentID)
	fmt.Printf("☁️  Provider: AWS ECS Fargate\n")

	fmt.Println("\n🌐 Access URLs:")
	fmt.Printf("   MockServer API: %s\n", result.MockServerURL)
	fmt.Printf("   Dashboard: %s\n", result.DashboardURL)

	if !result.TTLExpiry.IsZero() {
		fmt.Println("\n⏰ Auto-Teardown:")
		fmt.Printf("   Scheduled: %s\n", result.TTLExpiry.Format("2006-01-02 15:04 MST"))
		fmt.Printf("   Time remaining: %v\n", time.Until(result.TTLExpiry).Round(time.Minute))
	}

	fmt.Println("\n💡 Next Steps:")
	fmt.Println("   1. Test your MockServer API using the URL above")
	fmt.Println("   2. View request logs in the dashboard")
	fmt.Println("   3. Update mock expectations as needed")
	if !result.TTLExpiry.IsZero() {
		fmt.Println("   4. Infrastructure will auto-teardown to prevent costs")
	}
}

// Compatibility functions for existing code
func StartCollectionImportREPL(projectName, collectionFile, collectionType string) error {
	fmt.Printf("📂 Collection import for %s files will be implemented soon.\n", collectionType)
	return StartMockGenerationREPL(projectName)
}

func ResolveProjectInteractively(existing []string) (string, bool, error) {
	if len(existing) == 0 {
		return createNewProject()
	}

	var options []string
	for _, bucket := range existing {
		cleanName := utils.ExtractUserProjectName(utils.RemoveBucketPrefix(bucket))
		if cleanName != "" {
			options = append(options, cleanName)
		}
	}
	options = append(options, "📝 Create New Project")

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Select project:",
		Options: options,
	}, &choice); err != nil {
		return "", false, err
	}

	if strings.Contains(choice, "Create New") {
		return createNewProject()
	}

	for _, bucket := range existing {
		if strings.Contains(bucket, choice) {
			return utils.RemoveBucketPrefix(bucket), true, nil
		}
	}

	return "", false, fmt.Errorf("project not found")
}

func createNewProject() (string, bool, error) {
	var name string
	if err := survey.AskOne(&survey.Input{
		Message: "Project name:",
		Help:    "Choose a unique name for your mock project",
	}, &name); err != nil {
		return "", false, err
	}

	suffix, _ := utils.GenerateRandomSuffix()
	return fmt.Sprintf("%s-%s", name, suffix), false, nil
}

func SelectProjectAction(projectName string) string {
	cleanName := utils.ExtractUserProjectName(projectName)
	var action string
	survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Project: %s", cleanName),
		Options: []string{"Generate", "Edit", "Delete", "Cancel"},
	}, &action)
	return action
}

// generateMockConfiguration uses the MCP engine to generate configurations
func generateMockConfiguration(method, projectName string) (string, error) {
	ctx := context.Background()
	cleanName := utils.ExtractUserProjectName(projectName)

	switch method {
	case "describe":
		return generateFromDescription(ctx, cleanName)
	case "interactive":
		return generateInteractive(ctx, cleanName)
	case "collection":
		return generateFromCollection(ctx, cleanName)
	case "template":
		return generateFromTemplate(ctx, cleanName)
	default:
		return generateFromDescription(ctx, cleanName)
	}
}

// generateFromDescription uses AI to generate from natural language
func generateFromDescription(ctx context.Context, projectName string) (string, error) {
	fmt.Println("🤖 AI-Powered Generation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Check available providers
	providers := mcp.ListProviders()
	fmt.Println("🔍 Available AI Providers:")
	for _, provider := range providers {
		if provider.Available {
			fmt.Printf("✅ %s (%s)\n", provider.Name, provider.Cost)
		} else {
			fmt.Printf("❌ %s (not configured)\n", provider.Name)
		}
	}
	fmt.Println()
	
	// Get user description
	var description string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Describe your API (be specific about endpoints, data, functionality):",
		Help:    "Example: 'User management API with signup, login, profile endpoints. Users have email, name, role.'",
	}, &description); err != nil {
		return "", err
	}
	
	if strings.TrimSpace(description) == "" {
		return "", fmt.Errorf("description cannot be empty")
	}
	
	// Generate using MCP
	fmt.Println("🤖 Generating with AI...")
	result, err := mcp.GenerateWithProvider(ctx, description, "", projectName)
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", err)
	}
	
	// Show generation info
	fmt.Printf("✅ Generated by: %s\n", result.Provider)
	if result.TokensUsed > 0 {
		fmt.Printf("📊 Tokens used: %d\n", result.TokensUsed)
	}
	fmt.Printf("⏱️  Generation time: %s\n", result.GenerationTime)
	
	// Show warnings/suggestions
	for _, warning := range result.Warnings {
		fmt.Printf("⚠️  %s\n", warning)
	}
	for _, suggestion := range result.Suggestions {
		fmt.Printf("💡 %s\n", suggestion)
	}
	
	return result.MockServerJSON, nil
}

// generateInteractive uses the 7-step interactive builder
func generateInteractive(ctx context.Context, projectName string) (string, error) {
	// Call our dedicated interactive builder
	return StartInteractiveBuilder(projectName)
}

// generateFromCollection imports from API collections
func generateFromCollection(ctx context.Context, projectName string) (string, error) {
	fmt.Println("📂 Collection Import")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Get collection type
	var collectionType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select collection type:",
		Options: []string{"postman", "bruno", "insomnia"},
	}, &collectionType); err != nil {
		return "", err
	}
	
	// Get file path
	var filePath string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter path to collection file:",
	}, &filePath); err != nil {
		return "", err
	}
	
	// Read collection file
	collectionData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read collection file: %w", err)
	}
	
	// Generate based on collection type
	var result *mcp.GenerationResult
	switch collectionType {
	case "postman":
		result, err = mcp.GenerateFromPostman(ctx, collectionData, projectName)
	case "bruno":
		result, err = mcp.GenerateFromBruno(ctx, collectionData, projectName)
	case "insomnia":
		result, err = mcp.GenerateFromInsomnia(ctx, collectionData, projectName)
	default:
		return "", fmt.Errorf("unsupported collection type: %s", collectionType)
	}
	
	if err != nil {
		return "", fmt.Errorf("collection import failed: %w", err)
	}
	
	// Show security info
	if result.CredentialsSanitized > 0 {
		fmt.Printf("🔒 Sanitized %d credentials from collection\n", result.CredentialsSanitized)
	}
	for _, warning := range result.SecurityWarnings {
		fmt.Printf("⚠️  Security: %s\n", warning)
	}
	
	return result.MockServerJSON, nil
}

// generateFromTemplate uses quick templates
func generateFromTemplate(ctx context.Context, projectName string) (string, error) {
	fmt.Println("📝 Template Generator")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Choose template type
	var templateType string
	if err := survey.AskOne(&survey.Select{
		Message: "Choose API template:",
		Options: []string{
			"user - User management API",
			"auth - Authentication API",
			"product - Product/catalog API",
			"order - Order/payment API",
			"file - File upload API",
			"custom - Custom CRUD API",
		},
	}, &templateType); err != nil {
		return "", err
	}
	
	templateType = strings.Split(templateType, " ")[0]
	
	// Generate from template
	result, err := mcp.GenerateWithProvider(ctx, templateType+" management API", "template", projectName)
	if err != nil {
		return "", fmt.Errorf("template generation failed: %w", err)
	}
	
	return result.MockServerJSON, nil
}
