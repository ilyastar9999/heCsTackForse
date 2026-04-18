package models

import "time"

// Sploit is an exploit script uploaded by a team for the Attack & Defence mode.
// The platform runs enabled sploits against all other teams' service instances
// every round.
type Sploit struct {
	ID          int64      `json:"id"`
	TeamID      int64      `json:"team_id"`
	ChallengeID int64      `json:"challenge_id"`
	Name        string     `json:"name"`
	Language    string     `json:"language"` // python3, bash, ...
	Script      string     `json:"script,omitempty"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
}

// SploitResult is the outcome of running one sploit against one target team in
// a given round.
type SploitResult struct {
	ID             int64     `json:"id"`
	SploitID       int64     `json:"sploit_id"`
	TargetTeamID   int64     `json:"target_team_id"`
	Round          int64     `json:"round"`
	Stdout         string    `json:"stdout,omitempty"`
	FlagsCaptured  int       `json:"flags_captured"`
	FlagsSubmitted int       `json:"flags_submitted"`
	Error          string    `json:"error,omitempty"`
	RanAt          time.Time `json:"ran_at"`
}

// VPNPeer stores the WireGuard keypair and IP allocation for a team.
type VPNPeer struct {
	ID         int64     `json:"id"`
	TeamID     int64     `json:"team_id"`
	PrivateKey string    `json:"-"` // never sent to clients
	PublicKey  string    `json:"public_key"`
	AllowedIP  string    `json:"allowed_ip"` // e.g. 10.8.1.0/24
	CreatedAt  time.Time `json:"created_at"`
}

// ADRound represents a completed or in-progress A&D round.
type ADRound struct {
	ID         int64      `json:"id"`
	Round      int64      `json:"round"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// ADServiceStatus holds the checker result for one team's service in one round.
type ADServiceStatus struct {
	ChallengeID int64     `json:"challenge_id"`
	TeamID      int64     `json:"team_id"`
	Status      string    `json:"status"` // up / down / corrupt
	Round       int64     `json:"round"`
	Score       int       `json:"score"`
	CheckedAt   time.Time `json:"checked_at"`
}
