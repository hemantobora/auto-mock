package models

// Feature is a provider-agnostic “thing you need to do”.
type Feature string

const (
	FeatNetworking     Feature = "networking" // VPC/VNet + subnets + routing + NAT/GW
	FeatLoadBal        Feature = "load_balancer"
	FeatStorage        Feature = "object_storage"   // S3/Blob/GCS
	FeatCerts          Feature = "certificates"     // ACM/KeyVault Certs/Certificate Manager
	FeatIAMWrite       Feature = "iam_write"        // create roles/policies/service-accounts
	FeatPassRole       Feature = "pass_role"        // pass/impersonate to workload service
	FeatDNS            Feature = "dns"              // Route53/Priv DNS/Cloud DNS
	FeatTags           Feature = "tags_labels"      // tagging/labels policy
	FeatLogging        Feature = "logging"          // log groups / sinks
	FeatECSControl     Feature = "ecs_control"      // ECS/Fargate task control APIs
	FeatAppAutoScaling Feature = "app_auto_scaling" // Application Auto Scaling APIs
)

type Capability struct {
	Feature Feature
	Allow   bool
	Notes   string // optional detail from provider
}

type PreflightResult struct {
	Identity     string       // e.g., ARN, Service Account Email, etc.
	Capabilities []Capability // granular allow/deny by feature
	Advice       []string     // human-readable next steps
	// Suggestion is a provider-specific doc (policy JSON, role binding YAML, etc.)
	SuggestedPolicy string
}
