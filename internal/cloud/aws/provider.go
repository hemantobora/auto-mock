// internal/cloud/aws/provider.go
package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/cloud/naming"
	"github.com/hemantobora/auto-mock/internal/models"
)

type Feature = models.Feature
type PreflightResult = models.PreflightResult
type Capability = models.Capability

const (
	FeatNetworking     = models.FeatNetworking
	FeatLoadBal        = models.FeatLoadBal
	FeatStorage        = models.FeatStorage
	FeatCerts          = models.FeatCerts
	FeatIAMWrite       = models.FeatIAMWrite
	FeatPassRole       = models.FeatPassRole
	FeatDNS            = models.FeatDNS
	FeatTags           = models.FeatTags
	FeatLogging        = models.FeatLogging
	FeatECSControl     = models.FeatECSControl
	FeatAppAutoScaling = models.FeatAppAutoScaling
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

// DeleteProject removes the S3 bucket and all associated resources for a project
func (p *Provider) DeleteProject() error {
	// Delete the S3 bucket
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, try to delete all objects in the bucket (if any)
	// Note: In the current MVP, we don't store objects, but this is future-proof

	// Delete the bucket itself
	_, err := p.S3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &p.BucketName,
	})
	if err != nil {
		return &models.ProviderError{
			Provider:  "aws",
			Operation: "delete",
			Resource:  p.BucketName,
			Cause:     fmt.Errorf("failed to delete bucket %s: %w", p.BucketName, err),
		}
	}

	fmt.Printf("🗑️ Deleted project: %s\n", p.projectID)

	return nil
}

func actionsFor(features []Feature) map[Feature][]string {
	all := map[Feature][]string{
		FeatNetworking: {
			// VPC + subnets + routing + NAT + SG
			"ec2:CreateVpc", "ec2:CreateSubnet",
			"ec2:CreateInternetGateway", "ec2:AttachInternetGateway",
			"ec2:AllocateAddress", "ec2:CreateNatGateway",
			"ec2:CreateRouteTable", "ec2:CreateRoute",
			"ec2:CreateSecurityGroup", "ec2:AuthorizeSecurityGroupIngress", "ec2:AuthorizeSecurityGroupEgress",
			// VPC Endpoint (S3) used in your TF
			"ec2:CreateVpcEndpoint",
			// describes
			"ec2:DescribeVpcs", "ec2:DescribeSubnets",
		},
		FeatLoadBal: {
			"elasticloadbalancing:CreateLoadBalancer",
			"elasticloadbalancing:CreateTargetGroup",
			"elasticloadbalancing:CreateListener",
			"elasticloadbalancing:RegisterTargets",
		},
		FeatStorage: {
			// config bucket (create once) + basic ops
			"s3:CreateBucket", "s3:PutBucketTagging",
			"s3:PutObject", "s3:GetObject", "s3:ListBucket",
		},
		FeatCerts: {"acm:RequestCertificate", "acm:DescribeCertificate", "acm:AddTagsToCertificate"},
		FeatDNS:   {"route53:ChangeResourceRecordSets"}, // records only
		FeatIAMWrite: {
			"iam:CreateRole", "iam:DeleteRole", "iam:UpdateAssumeRolePolicy",
			"iam:PutRolePolicy", "iam:DeleteRolePolicy",
			"iam:AttachRolePolicy", "iam:DetachRolePolicy",
		},
		FeatPassRole: {"iam:PassRole"},
		FeatLogging: {
			"logs:CreateLogGroup", "logs:PutRetentionPolicy", "logs:DeleteLogGroup",
		},
		FeatECSControl: {
			"ecs:CreateCluster", "ecs:DeleteCluster",
			"ecs:RegisterTaskDefinition", "ecs:DeregisterTaskDefinition",
			"ecs:CreateService", "ecs:UpdateService", "ecs:DeleteService",
		},
		FeatTags: {
			"ec2:CreateTags", "ec2:DeleteTags",
			"elasticloadbalancing:AddTags", "elasticloadbalancing:RemoveTags",
			"s3:PutBucketTagging", "s3:GetBucketTagging",
			"iam:TagRole", "iam:UntagRole",
			"ecs:TagResource", "ecs:UntagResource",
			"logs:TagLogGroup", "logs:UntagLogGroup",
			"acm:AddTagsToCertificate", "acm:RemoveTagsFromCertificate",
		},
		FeatAppAutoScaling: {
			"application-autoscaling:RegisterScalableTarget",
			"application-autoscaling:PutScalingPolicy",
			// CloudWatch alarms used by your step policies
			"cloudwatch:PutMetricAlarm", "cloudwatch:DeleteAlarms",
		},
	}
	if len(features) == 0 {
		features = []Feature{
			FeatNetworking, FeatLoadBal, FeatStorage, FeatCerts, FeatDNS,
			FeatIAMWrite, FeatPassRole, FeatLogging, FeatTags, FeatECSControl,
			FeatAppAutoScaling,
		}
	}
	out := map[Feature][]string{}
	for _, f := range features {
		if acts, ok := all[f]; ok {
			out[f] = acts
		}
	}
	return out
}

func (p *Provider) PreFlightCheck(ctx context.Context, needed []Feature) (*models.PreflightResult, error) {
	featureActions := actionsFor(needed)

	// Collect de-duped actions
	all := []string{}
	seen := map[string]struct{}{}
	for _, acts := range featureActions {
		for _, act := range acts {
			if _, ok := seen[act]; !ok {
				seen[act] = struct{}{}
				all = append(all, act)
			}
		}
	}
	stsClient := sts.NewFromConfig(p.AWSConfig)
	id, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get caller identity: %w", err)
	}
	iamClient := iam.NewFromConfig(p.AWSConfig)

	out, err := iamClient.SimulatePrincipalPolicy(ctx, &iam.SimulatePrincipalPolicyInput{
		PolicySourceArn: aws.String(*id.Arn),
		ActionNames:     all,
		// You can add ContextEntries to model iam:PassedToService if needed
	})
	if err != nil {
		return nil, fmt.Errorf("policy simulation failed: %w", err)
	}

	allowed := map[string]bool{}
	for _, e := range out.EvaluationResults {
		allowed[aws.ToString(e.EvalActionName)] = e.EvalDecision == iamtypes.PolicyEvaluationDecisionTypeAllowed
	}

	// Roll up by feature
	var caps []models.Capability
	advice := []string{}
	for f, acts := range featureActions {
		ok := true
		for _, a := range acts {
			if !allowed[a] {
				ok = false
				break
			}
		}
		caps = append(caps, models.Capability{Feature: f, Allow: ok})
		if !ok {
			switch f {
			case FeatNetworking:
				advice = append(advice, "Networking creation not permitted → require existing VPC/subnets/SGs.")
			case FeatIAMWrite:
				advice = append(advice, "IAM write not permitted → require BYO execution/task/cleanup role ARNs.")
			case FeatPassRole:
				advice = append(advice, "PassRole restricted → ensure allowed role paths or specific role ARNs.")
			case FeatDNS:
				advice = append(advice, "Route53 changes not permitted → user must supply hosted zone or skip DNS.")
			case FeatCerts:
				advice = append(advice, "ACM issuance not permitted → user must supply existing certificate ARN.")
			case FeatLoadBal:
				advice = append(advice, "Load Balancer creation not permitted → user must supply existing ALB/Target Group ARNs.")
			case FeatStorage:
				advice = append(advice, "S3 bucket operations not permitted → ensure bucket exists and is accessible.")
			// Add more as needed
			case FeatTags:
				advice = append(advice, "Tagging operations not permitted → resources may be untagged.")
			case FeatLogging:
				advice = append(advice, "CloudWatch Logs operations not permitted → logs may not be created or retained.")
			case FeatECSControl:
				advice = append(advice, "ECS control not permitted → cannot create cluster/service/task; supply existing or get these actions allowed.")
			case FeatAppAutoScaling:
				advice = append(advice, "App Auto Scaling / CW alarms not permitted → disable autoscaling or have these actions allowed.")
			}
		}
	}

	// Minimal suggestion only for missing features (scaffold policy)
	type stmt struct {
		Effect    string                 `json:"Effect"`
		Action    interface{}            `json:"Action"`
		Resource  interface{}            `json:"Resource"`
		Condition map[string]interface{} `json:"Condition,omitempty"`
	}
	doc := struct {
		Version   string `json:"Version"`
		Statement []stmt `json:"Statement"`
	}{Version: "2012-10-17"}

	isAllowed := func(f Feature) bool {
		i := slices.IndexFunc(caps, func(c Capability) bool { return c.Feature == f })
		return i >= 0 && caps[i].Allow
	}
	// S3 (scoped)
	if acts, ok := featureActions[FeatStorage]; ok && !isAllowed(FeatStorage) {
		doc.Statement = append(doc.Statement, stmt{
			Effect: "Allow", Action: acts,
			Resource: []string{"arn:aws:s3:::auto-mock-*", "arn:aws:s3:::auto-mock-*/*"},
		})
	}
	// IAM write (scoped to prefixes)
	if acts, ok := featureActions[FeatIAMWrite]; ok && !isAllowed(FeatIAMWrite) {
		doc.Statement = append(doc.Statement, stmt{
			Effect: "Allow", Action: acts,
			Resource: []string{"arn:aws:iam::*:role/auto-mock/*"},
		})
	}
	// PassRole (with conditions)
	if acts, ok := featureActions[FeatPassRole]; ok && !isAllowed(FeatPassRole) {
		doc.Statement = append(doc.Statement, stmt{
			Effect: "Allow", Action: acts,
			Resource: []string{"arn:aws:iam::*:role/auto-mock/*"},
			Condition: map[string]interface{}{"StringEquals": map[string]interface{}{
				"iam:PassedToService": []string{"ecs.amazonaws.com", "ecstasks.amazonaws.com"},
			}},
		})
	}
	// Others left broad (*) — tighten with tags later if you enforce them
	for _, f := range []Feature{FeatNetworking, FeatLoadBal, FeatCerts, FeatDNS, FeatTags} {
		if acts, ok := featureActions[f]; ok && !isAllowed(f) {
			doc.Statement = append(doc.Statement, stmt{Effect: "Allow", Action: acts, Resource: "*"})
		}
	}

	suggest := ""
	if len(doc.Statement) > 0 {
		b, _ := json.MarshalIndent(doc, "", "  ")
		suggest = string(b)
	}
	return &PreflightResult{Identity: *id.Arn, Capabilities: caps, Advice: advice, SuggestedPolicy: suggest}, nil
}
