package plugin

type StaticChallengeType struct{}

func (s *StaticChallengeType) Name() string                { return "static" }
func (s *StaticChallengeType) Init(_ map[string]any) error { return nil }
func (s *StaticChallengeType) TypeID() string              { return "static" }
func (s *StaticChallengeType) Verify(_, _ string) bool     { return false }
func (s *StaticChallengeType) Descriptor() ChallengeTypeSpec {
	return ChallengeTypeSpec{
		ID:                     "static",
		Title:                  "Static",
		Description:            "Classic jeopardy challenge with one or more manually submitted flags.",
		SubmissionMode:         "manual_flag",
		AccessMode:             "static",
		DefaultDeployType:      "no_deploy",
		SupportsManualFlags:    true,
		SupportsCheckerConfig:  true,
		SupportsFiles:          true,
		SupportsHints:          true,
		SupportsConnectionInfo: true,
	}
}

type DynamicDeployChallengeType struct{}

func (d *DynamicDeployChallengeType) Name() string                { return "dynamic_deploy" }
func (d *DynamicDeployChallengeType) Init(_ map[string]any) error { return nil }
func (d *DynamicDeployChallengeType) TypeID() string              { return "dynamic_deploy" }
func (d *DynamicDeployChallengeType) Verify(_, _ string) bool     { return false }
func (d *DynamicDeployChallengeType) Descriptor() ChallengeTypeSpec {
	return ChallengeTypeSpec{
		ID:                     "dynamic_deploy",
		Title:                  "Dynamic Deploy",
		Description:            "A per-instance challenge where the platform injects and verifies the flag from the deployment context.",
		SubmissionMode:         "dynamic_flag",
		AccessMode:             "instance",
		DefaultDeployType:      "per_instance",
		RequiresDeploy:         true,
		SupportsCheckerConfig:  false,
		SupportsFiles:          true,
		SupportsHints:          true,
		SupportsConnectionInfo: true,
	}
}

type PentestChallengeType struct{}

func (p *PentestChallengeType) Name() string                { return "pentest" }
func (p *PentestChallengeType) Init(_ map[string]any) error { return nil }
func (p *PentestChallengeType) TypeID() string              { return "pentest" }
func (p *PentestChallengeType) Verify(_, _ string) bool     { return false }
func (p *PentestChallengeType) Descriptor() ChallengeTypeSpec {
	return ChallengeTypeSpec{
		ID:                     "pentest",
		Title:                  "Pentest",
		Description:            "VPN-access challenge with manual submission for user, root, and additional flags defined by the checker configuration.",
		SubmissionMode:         "manual_flag",
		AccessMode:             "vpn",
		DefaultDeployType:      "always_on",
		SupportsManualFlags:    true,
		SupportsCheckerConfig:  true,
		SupportsFiles:          true,
		SupportsHints:          true,
		SupportsConnectionInfo: true,
		SupportsVPN:            true,
	}
}

type ADAttackChallengeType struct{}

func (a *ADAttackChallengeType) Name() string                { return "attack_defence_attack" }
func (a *ADAttackChallengeType) Init(_ map[string]any) error { return nil }
func (a *ADAttackChallengeType) TypeID() string              { return "attack_defence_attack" }
func (a *ADAttackChallengeType) Verify(_, _ string) bool     { return false }
func (a *ADAttackChallengeType) Descriptor() ChallengeTypeSpec {
	return ChallengeTypeSpec{
		ID:                     "attack_defence_attack",
		Title:                  "Attack & Defence Attack",
		Description:            "Exploit-upload challenge scored by the attack-defence engine against runtime services and weighted vulnerability buckets.",
		SubmissionMode:         "exploit_upload",
		AccessMode:             "exploit_runner",
		DefaultDeployType:      "always_on",
		RequiresDeploy:         true,
		SupportsCheckerConfig:  true,
		SupportsFiles:          true,
		SupportsHints:          true,
		SupportsConnectionInfo: true,
		SupportsExploitUpload:  true,
		EngineManaged:          true,
	}
}

type ADDefenseChallengeType struct{}

func (d *ADDefenseChallengeType) Name() string                { return "attack_defence_defense" }
func (d *ADDefenseChallengeType) Init(_ map[string]any) error { return nil }
func (d *ADDefenseChallengeType) TypeID() string              { return "attack_defence_defense" }
func (d *ADDefenseChallengeType) Verify(_, _ string) bool     { return false }
func (d *ADDefenseChallengeType) Descriptor() ChallengeTypeSpec {
	return ChallengeTypeSpec{
		ID:                     "attack_defence_defense",
		Title:                  "Attack & Defence Defense",
		Description:            "Service-defense challenge scored by checker rounds and exploit resistance over VPN-backed infrastructure.",
		SubmissionMode:         "service_score",
		AccessMode:             "vpn",
		DefaultDeployType:      "always_on",
		RequiresDeploy:         true,
		SupportsCheckerConfig:  true,
		SupportsFiles:          true,
		SupportsHints:          true,
		SupportsConnectionInfo: true,
		SupportsVPN:            true,
		EngineManaged:          true,
	}
}
