package models

type Capability struct {
	Networking struct {
		VPC, SG bool
	}
	// NEW: separate IAM capability (was incorrectly inferred from TLS before)
	IAM struct{ Roles bool }
}

type Inputs struct {
	VPCID             string
	PublicSubnets     []string
	PrivateSubnets    []string
	InternetGatewayID string
	NatGatewayIDs     []string
	ALBSGID           string
	ECSSGID           string
	ExecutionRoleARN  string
	TaskRoleARN       string
}

type UseExisting struct {
	VPC, Subnets, IGW, NAT, SG, ECS, IAM, Logs bool
}

func (cap Capability) DeriveUseExisting() UseExisting {
	return UseExisting{
		VPC:     !cap.Networking.VPC,
		Subnets: !cap.Networking.VPC,
		IGW:     !cap.Networking.VPC,
		NAT:     !cap.Networking.VPC,
		SG:      !cap.Networking.SG,

		IAM: !cap.IAM.Roles,
	}
}
