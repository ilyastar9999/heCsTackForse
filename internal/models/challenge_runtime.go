package models

type ScoredFlagSpec struct {
	Key    string `json:"key"`
	Label  string `json:"label,omitempty"`
	Value  string `json:"value"`
	Type   string `json:"type,omitempty"`
	Points int    `json:"points,omitempty"`
}

type PentestCheckerConfig struct {
	Flags []ScoredFlagSpec `json:"flags"`
}

type ADBucketSpec struct {
	Key    string `json:"key"`
	Label  string `json:"label,omitempty"`
	Name   string `json:"name,omitempty"`
	Points int    `json:"points,omitempty"`
}

type ADDefenseCheckSpec struct {
	Key    string `json:"key"`
	Label  string `json:"label,omitempty"`
	Name   string `json:"name,omitempty"`
	Script string `json:"script"`
	Points int    `json:"points,omitempty"`
}

type ADCheckerConfig struct {
	AttackBuckets []ADBucketSpec       `json:"attack_buckets"`
	LegacyBuckets []ADBucketSpec       `json:"buckets,omitempty"`
	DefenseChecks []ADDefenseCheckSpec `json:"defense_checks"`
}

func (c ADCheckerConfig) Buckets() []ADBucketSpec {
	if len(c.AttackBuckets) > 0 {
		return c.AttackBuckets
	}
	return c.LegacyBuckets
}
