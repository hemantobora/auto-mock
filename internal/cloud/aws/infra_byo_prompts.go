package aws

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hemantobora/auto-mock/internal/models"
)

type Capability = models.Capability
type Inputs = models.Inputs

/* ===================== Validators ===================== */

var (
	// AWS resource IDs are fixed-length hex strings: usually 8 or 17 chars.
	reVPC = regexp.MustCompile(`^vpc-[0-9a-f]{8,17}$`)
	reSub = regexp.MustCompile(`^subnet-[0-9a-f]{8,17}$`)
	reSG  = regexp.MustCompile(`^sg-[0-9a-f]{8,17}$`)
	reIGW = regexp.MustCompile(`^igw-[0-9a-f]{8,17}$`)
	reNAT = regexp.MustCompile(`^nat-[0-9a-f]{8,17}$`)

	// More precise ARN pattern (accounts, partitions, and regions)
	reARN = regexp.MustCompile(
		`^arn:(aws|aws-cn|aws-us-gov):[a-z0-9-]+:[a-z0-9-]*:\d{12}:[A-Za-z0-9_.:/+=,@-]+$`,
	)
)

func idOrEmpty(v string, rx *regexp.Regexp, label string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	if !rx.MatchString(strings.TrimSpace(v)) {
		return fmt.Errorf("%s looks invalid", label)
	}
	return nil
}

func listOrEmpty(v string, rx *regexp.Regexp, label string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	for _, s := range splitCSV(v) {
		if !rx.MatchString(s) {
			return fmt.Errorf("%s item %q looks invalid", label, s)
		}
	}
	return nil
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateInputs(u models.UseExisting, in models.Inputs) error {
	req := func(ok bool, what string) error {
		if !ok {
			return fmt.Errorf("%s is required when not creating it", what)
		}
		return nil
	}

	if u.VPC {
		if err := req(in.VPCID != "", "VPC ID"); err != nil {
			return err
		}
	}
	if u.Subnets {
		if err := req(len(in.PublicSubnets) > 0, "Public Subnet IDs"); err != nil {
			return err
		}
		if err := req(len(in.PrivateSubnets) > 0, "Private Subnet IDs"); err != nil {
			return err
		}
	}
	if u.IGW {
		if err := req(in.InternetGatewayID != "", "Internet Gateway ID"); err != nil {
			return err
		}
	}
	if u.SG {
		if err := req(in.ALBSGID != "", "ALB Security Group ID"); err != nil {
			return err
		}
		if err := req(in.ECSSGID != "", "ECS Security Group ID"); err != nil {
			return err
		}
	}
	if u.IAM {
		if err := req(in.ExecutionRoleARN != "", "Execution Role ARN"); err != nil {
			return err
		}
		if err := req(in.TaskRoleARN != "", "Task Role ARN"); err != nil {
			return err
		}
	}
	return nil
}

func validateScalingConfiguration(minTasks, maxTasks int) error {
	// For percentage-based scaling with +200% max adjustment
	recommendedMax := minTasks * 6
	absoluteMinMax := minTasks * 3

	if maxTasks < absoluteMinMax {
		return fmt.Errorf(
			"max_tasks (%d) is too low for min_tasks (%d)\n"+
				"  With +200%% scaling, you need at least: %d tasks\n"+
				"  Recommended max: %d tasks",
			maxTasks, minTasks, absoluteMinMax, recommendedMax)
	}

	if maxTasks < recommendedMax {
		fmt.Printf("⚠️  Warning: max_tasks (%d) may be too low for optimal scaling\n", maxTasks)
		fmt.Printf("   Recommended max for min=%d: %d tasks\n", minTasks, recommendedMax)
		fmt.Printf("   Current max allows only %.1fx growth\n", float64(maxTasks)/float64(minTasks))
	}

	return nil
}

// promptDeploymentOptionsREPL prompts for deployment configuration in REPL
func promptDeploymentOptionsREPL(options *models.DeploymentOptions) error {

	// Your size map (cpu in CPU units; memory in MiB)
	taskConfig := map[string]struct{ CPU, MemMiB int }{
		"small":  {CPU: 256, MemMiB: 512},   // 0.25 vCPU, 0.5 GB
		"medium": {CPU: 512, MemMiB: 1024},  // 0.5 vCPU, 1 GB
		"large":  {CPU: 1024, MemMiB: 2048}, // 1 vCPU, 2 GB
		"xlarge": {CPU: 2048, MemMiB: 4096}, // 2 vCPU, 4 GB
	}

	// Instance size
	if options.InstanceSize == "" {
		var instanceSize string
		sizePrompt := &survey.Select{
			Message: "Select task size (Fargate CPU / memory allocation):",
			Options: []string{"small", "medium", "large", "xlarge"},
			Default: "small",
			Description: func(value string, index int) string {
				switch value {
				case "small":
					return "0.25 vCPU, 0.5 GB RAM (light load / testing)"
				case "medium":
					return "0.5 vCPU, 1 GB RAM (moderate load)"
				case "large":
					return "1 vCPU, 2 GB RAM (high load)"
				case "xlarge":
					return "2 vCPU, 4 GB RAM (very high load)"
				default:
					return ""
				}
			},
		}

		if err := survey.AskOne(sizePrompt, &instanceSize); err != nil {
			return err
		}
		options.InstanceSize = instanceSize
	}

	cfg, ok := taskConfig[options.InstanceSize]
	if !ok {
		return fmt.Errorf("  Unknown size %q. Valid: small, medium, large, xlarge", options.InstanceSize)
	}
	options.CPUUnits = cfg.CPU
	options.MemoryUnits = cfg.MemMiB

	// Min tasks
	if options.MinTasks == 0 {
		var minTask string
		minPrompt := &survey.Input{
			Message: "Minimum number of tasks:",
			Default: "2",
			Help:    "Minimum number of Fargate tasks running at all times. 1–2 is fine for testing; use higher values for production.",
		}

		if err := survey.AskOne(minPrompt, &minTask); err != nil {
			return err
		}

		// Convert to int
		var minTaskInt int
		fmt.Sscanf(minTask, "%d", &minTaskInt)
		options.MinTasks = minTaskInt

		// Validate min > 0
		if options.MinTasks <= 0 {
			return fmt.Errorf("minimum tasks must be greater than zero")
		}
	}

	// Max tasks
	if options.MaxTasks == 0 {
		// Calculate recommended max (min × 6 matches the scaling validator threshold)
		recommendedMax := options.MinTasks * 6

		for {
			var maxTask string
			maxPrompt := &survey.Input{
				Message: fmt.Sprintf("Maximum number of tasks [recommended: %d]:", recommendedMax),
				Default: fmt.Sprintf("%d", recommendedMax),
				Help:    fmt.Sprintf("Maximum number of Fargate tasks to run. Recommended: %d (min × 6 gives headroom for the +200%% scaling step).", recommendedMax),
			}

			if err := survey.AskOne(maxPrompt, &maxTask); err != nil {
				return err
			}

			// Convert to int
			var maxTaskInt int
			if _, err := fmt.Sscanf(maxTask, "%d", &maxTaskInt); err != nil || maxTaskInt <= 0 {
				fmt.Println("❌ Invalid input. Please enter a positive number.")
				continue
			}
			options.MaxTasks = maxTaskInt

			// Validate max >= min
			if options.MaxTasks < options.MinTasks {
				fmt.Printf("❌ Maximum tasks (%d) must be greater than or equal to minimum tasks (%d)\n",
					options.MaxTasks, options.MinTasks)
				continue
			}

			// Validate scaling configuration
			if err := validateScalingConfiguration(options.MinTasks, options.MaxTasks); err != nil {
				fmt.Printf("❌ %v\n", err)

				// Ask if they want to continue anyway
				var continueAnyway bool
				continuePrompt := &survey.Confirm{
					Message: "Continue with this configuration anyway?",
					Default: false,
				}
				if err := survey.AskOne(continuePrompt, &continueAnyway); err != nil {
					return err
				}

				if !continueAnyway {
					continue // Go back to max tasks input
				}
			}

			// All validations passed, break out of loop
			break
		}
	}

	var confirmed bool
	confirmPrompt := &survey.Confirm{
		Message: "Would you like to create a private ALB for your mocks? This is essentially useful if your service connecting to mocks is also in the same VPC.",
		Default: false,
		Help:    "A private ALB allows internal access within your VPC without exposing the mocks to the public internet.",
	}
	if err := survey.AskOne(confirmPrompt, &confirmed); err == nil {
		options.PrivateALB = confirmed
	}

	// ── Custom domain (optional) ──────────────────────────────────────────────
	// Blank  → self-signed cert (no Route53 involvement).
	// Filled → ask whether the zone already exists in R53 or needs to be created.
	//   BYO zone  (N): data source lookup → ACM cert → validation records → A-alias
	//   New zone  (Y): resource aws_route53_zone → ACM cert → validation records → A-alias
	//                  (NS records are output so the user can delegate at their registrar)
	var customDomain string
	if err := survey.AskOne(&survey.Input{
		Message: "Custom domain (leave blank to skip, e.g. mock.offers.com):",
		Help:    "The mock will be reachable at <project-name>.<domain> over HTTPS with a valid ACM certificate.",
	}, &customDomain, survey.WithValidator(func(ans interface{}) error {
		s := strings.TrimSpace(ans.(string))
		if s == "" {
			return nil // optional
		}
		if !strings.Contains(s, ".") || strings.ContainsAny(s, " \t") {
			return fmt.Errorf("domain looks invalid (expected format: mock.offers.com)")
		}
		return nil
	})); err == nil && strings.TrimSpace(customDomain) != "" {
		options.CustomDomain = strings.TrimSpace(customDomain)

		var createZone bool
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("Would you like to host %s in Route53?", options.CustomDomain),
			Default: false,
			Help: "No  → the zone already exists in Route53; the tool will look it up and add the necessary records.\n" +
				"Yes → the tool will create a new Route53 public hosted zone. After deploy, point your registrar to the NS records shown in the output.",
		}, &createZone); err == nil {
			options.CreateHostedZone = createZone
		}

		fmt.Printf("✓ Mock will be accessible at https://%s.%s\n", options.ProjectName, options.CustomDomain)
		if options.CreateHostedZone {
			fmt.Println("  ℹ️  A new hosted zone will be created. After deploy, update your registrar with the NS records printed in the Terraform output.")
		}
	}

	return nil
}

func assembleOptions(cap models.Capability, in models.Inputs) (*models.DeploymentOptions, error) {
	// from earlier snippet
	u := cap.DeriveUseExisting()
	if err := validateInputs(u, in); err != nil {
		return nil, err
	}

	// only pass SG IDs if using existing
	var sgIDs []string
	if u.SG {
		sgIDs = []string{in.ALBSGID, in.ECSSGID} // [ALB, ECS]
	}

	opts := &models.DeploymentOptions{
		// Networking
		UseExistingVPC:     u.VPC,
		VpcID:              ifThen(u.VPC, in.VPCID, ""),
		UseExistingSubnets: u.Subnets,
		PublicSubnetIDs:    ifThenSlice(u.Subnets, in.PublicSubnets, nil),
		PrivateSubnetIDs:   ifThenSlice(u.Subnets, in.PrivateSubnets, nil),
		UseExistingIGW:     u.IGW,
		InternetGatewayID:  ifThen(u.IGW, in.InternetGatewayID, ""),
		UseExistingNAT:     u.NAT,
		NatGatewayIDs:      ifThenSlice(u.NAT, in.NatGatewayIDs, nil),

		// Security groups
		UseExistingSecurityGroups: u.SG,
		SecurityGroupIDs:          sgIDs,

		// Compute / IAM / Logs
		UseExistingIAMRoles:    u.IAM,
		ExecutionRoleARN:       ifThen(u.IAM, in.ExecutionRoleARN, ""),
		TaskRoleARN:            ifThen(u.IAM, in.TaskRoleARN, ""),
		IAMRolePath:            in.IAMRolePath,
		IAMPermissionsBoundary: in.IAMPermissionsBoundary,
	}
	return opts, nil
}

// tiny helpers
func ifThen(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
func ifThenSlice[T any](cond bool, a, b []T) []T {
	if cond {
		return a
	}
	return b
}

// cloud/aws/provider.go
func (p *Provider) promptCapabilityAndInputs(ctx context.Context) (*models.Capability, *models.Inputs, error) {
	// 2) Interactive (TTY) – use survey

	stsClient := sts.NewFromConfig(p.AWSConfig)
	id, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get caller identity: %w", err)
	}
	identity := aws.ToString(id.Arn)
	// GetCallerIdentity is special-cased by AWS — no IAM permission is required
	// and even an explicit Deny cannot block it. However, if credentials are
	// invalid the call will fail above, so accountID is always non-empty here.
	// We pass it down so the permissions boundary prompt can auto-construct the
	// full ARN from just a policy name. If it were ever empty, the prompt falls
	// back to requiring a full ARN from the user.
	accountID := aws.ToString(id.Account)
	cap, err := promptCapabilitiesSurvey(identity)
	if err != nil {
		return nil, nil, err
	}

	in, err := promptInputsForMissingSurvey(cap, accountID)
	if err != nil {
		return nil, nil, err
	}

	return &cap, &in, nil
}

/* ===================== Capability Prompt ===================== */

func promptCapabilitiesSurvey(identity string) (Capability, error) {
	var cap Capability

	// 🪪 Identity Banner
	fmt.Printf("\n👤 Using AWS identity: %s\n", identity)
	fmt.Println("✨ Let’s review what this identity can create automatically.")
	fmt.Println("🔹 The tool assumes this identity is a Power User with permission to create all resources by default.")
	fmt.Println("🔹 Use SPACE to unselect any resources you want to BYO (Bring Your Own) instead of letting Terraform create them.")
	fmt.Println("🔹 If creation isn’t allowed for this identity, uncheck it — I’ll then prompt you for the required IDs (VPC, Subnets, SGs, etc).")
	fmt.Println()

	// Networking
	netChoices := []string{
		"Networks, especially VPC with DNS resolution, Subnets, IGW, NAT Gateway",
		"Security Groups",
		"IAM roles for ECS (execution & task)",
	}
	var netSel []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Create/BYO Networking & IAM Resources (CREATE permissions):",
		Options: netChoices,
		Default: netChoices, // sane default: assume greenfield
	}, &netSel); err != nil {
		return cap, err
	}
	has := func(s string) bool { return contains(netSel, s) }
	cap.Networking.VPC = has("Networks, especially VPC with DNS resolution, Subnets, IGW, NAT Gateway")
	cap.Networking.SG = has("Security Groups")
	cap.IAM.Roles = has("IAM roles for ECS (execution & task)")

	return cap, nil
}

/* ===================== Inputs Prompt (BYO for unchecked) ===================== */

func promptInputsForMissingSurvey(cap Capability, accountID string) (Inputs, error) {
	in := Inputs{} // MVP default

	// Networking BYO
	if !cap.Networking.VPC {
		if err := survey.AskOne(&survey.Input{
			Message: "VPC ID (vpc-xxxx) [Enable DNS support][required if you cannot create one]:",
		}, &in.VPCID, survey.WithValidator(func(ans interface{}) error {
			s := strings.TrimSpace(ans.(string))
			if s == "" {
				return errors.New("VPC ID required since you cannot create it")
			}
			return idOrEmpty(s, reVPC, "VPC ID")
		})); err != nil {
			return in, err
		}
	}

	if !cap.Networking.VPC {
		var pubCSV, privCSV string
		if err := survey.AskOne(&survey.Input{
			Message: "Load balancer Subnet IDs (comma-separated)",
			Help:    "Example: subnet-aaaa,subnet-bbbb",
		}, &pubCSV, survey.WithValidator(func(ans interface{}) error {
			return listOrEmpty(ans.(string), reSub, "Load balancer Subnet IDs")
		})); err != nil {
			return in, err
		}
		if err := survey.AskOne(&survey.Input{
			Message: "Application Subnet IDs (comma-separated)",
			Help:    "Example: subnet-aaaa,subnet-bbbb",
		}, &privCSV, survey.WithValidator(func(ans interface{}) error {
			return listOrEmpty(ans.(string), reSub, "Application Subnet IDs")
		})); err != nil {
			return in, err
		}
		in.PublicSubnets = splitCSV(pubCSV)
		in.PrivateSubnets = splitCSV(privCSV)
	}

	if !cap.Networking.VPC {
		if err := survey.AskOne(&survey.Input{
			Message: "Internet Gateway ID (igw-xxxx)]",
		}, &in.InternetGatewayID, survey.WithValidator(func(ans interface{}) error {
			return idOrEmpty(ans.(string), reIGW, "IGW ID")
		})); err != nil {
			return in, err
		}
	}

	if !cap.Networking.VPC {
		var natCSV string
		if err := survey.AskOne(&survey.Input{
			Message: "NAT Gateway IDs (comma-separated)",
		}, &natCSV, survey.WithValidator(func(ans interface{}) error {
			return listOrEmpty(ans.(string), reNAT, "NAT Gateway IDs")
		})); err != nil {
			return in, err
		}
		in.NatGatewayIDs = splitCSV(natCSV)
	}

	if !cap.Networking.SG {
		if err := survey.AskOne(&survey.Input{
			Message: "ALB Security Group ID (sg-xxxx)",
		}, &in.ALBSGID, survey.WithValidator(func(ans interface{}) error {
			s := strings.TrimSpace(ans.(string))
			if s == "" {
				return errors.New("ALB SG is required when not creating SGs. Generally its the one in public subnets")
			}
			return idOrEmpty(s, reSG, "ALB SG")
		})); err != nil {
			return in, err
		}
		if err := survey.AskOne(&survey.Input{
			Message: "ECS Security Group ID (sg-xxxx)",
		}, &in.ECSSGID, survey.WithValidator(func(ans interface{}) error {
			s := strings.TrimSpace(ans.(string))
			if s == "" {
				return errors.New("ECS SG is required when not creating SGs. Generally its the one in private subnets")
			}
			return idOrEmpty(s, reSG, "ECS SG")
		})); err != nil {
			return in, err
		}
	}

	if !cap.IAM.Roles {
		// Require both ARNs
		PrintECSIAMPolicies()
		fmt.Println("Make sure the roles are created and attach the necessary policies shown above before proceeding.")
		if err := survey.AskOne(&survey.Input{
			Message: "Existing ECS Execution Role ARN (required)",
		}, &in.ExecutionRoleARN, survey.WithValidator(func(ans interface{}) error {
			s := strings.TrimSpace(ans.(string))
			if s == "" {
				return errors.New("execution role ARN is required")
			}
			return idOrEmpty(s, reARN, "Execution Role ARN")
		})); err != nil {
			return in, err
		}

		PrintECSRoleIAMPolicies()
		if err := survey.AskOne(&survey.Input{
			Message: "Existing ECS Task Role ARN (required)",
		}, &in.TaskRoleARN, survey.WithValidator(func(ans interface{}) error {
			s := strings.TrimSpace(ans.(string))
			if s == "" {
				return errors.New("task role ARN is required")
			}
			return idOrEmpty(s, reARN, "Task Role ARN")
		})); err != nil {
			return in, err
		}
	} else {
		// Prompt for optional IAM role path and permissions boundary
		var rolePath string
		var permissionsBoundary string
		if err := survey.AskOne(&survey.Input{
			Message: "Any IAM Role Path (leave blank for default)",
			Help:    "If set, this will be used as the path for created IAM roles (e.g. /service-role/). Leave blank to use AWS default.",
		}, &rolePath); err != nil {
			return in, err
		}
		if rolePath != "" {
			// AWS requires the path to start and end with "/".
			// Normalize silently so the user doesn't have to remember the format.
			if !strings.HasPrefix(rolePath, "/") {
				rolePath = "/" + rolePath
			}
			if !strings.HasSuffix(rolePath, "/") {
				rolePath = rolePath + "/"
			}
			in.IAMRolePath = &rolePath
		}
		// Accept either a bare policy name or a full ARN.
		// When accountID is available (always, given GetCallerIdentity's special
		// AWS treatment) the tool auto-constructs the full ARN from just the name.
		// If accountID were somehow empty we fall back to requiring the full ARN.
		canAutoARN := accountID != ""
		var boundaryHelp string
		if canAutoARN {
			boundaryHelp = fmt.Sprintf(
				"Enter the policy name (e.g. my-boundary) or the full ARN.\n"+
					"If you provide just the name, the tool will build: arn:aws:iam::%s:policy/<name>",
				accountID,
			)
		} else {
			boundaryHelp = "Enter the full ARN (e.g. arn:aws:iam::123456789012:policy/my-boundary)."
		}
		if err := survey.AskOne(&survey.Input{
			Message: "IAM Permissions Boundary (leave blank for none)",
			Help:    boundaryHelp,
		}, &permissionsBoundary, survey.WithValidator(func(ans interface{}) error {
			s := strings.TrimSpace(ans.(string))
			if s == "" {
				return nil
			}
			if strings.HasPrefix(s, "arn:") {
				return idOrEmpty(s, reARN, "Permissions Boundary ARN")
			}
			if !canAutoARN {
				return fmt.Errorf("account ID unavailable — please provide the full ARN")
			}
			if strings.ContainsAny(s, " /") {
				return fmt.Errorf("policy name looks invalid — use just the name (e.g. my-boundary) or a full ARN")
			}
			return nil
		})); err != nil {
			return in, err
		}
		if permissionsBoundary != "" {
			var fullARN string
			if strings.HasPrefix(permissionsBoundary, "arn:") {
				fullARN = permissionsBoundary
			} else {
				fullARN = fmt.Sprintf("arn:aws:iam::%s:policy/%s", accountID, permissionsBoundary)
			}
			in.IAMPermissionsBoundary = &fullARN
		}
	}

	return in, nil
}

/* ===================== Helpers ===================== */

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
