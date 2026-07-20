// Package plugin defines the core plugin interfaces for heCsTackForse and
// provides built-in implementations for scoring and flag checking.
//
// # Plugin types
//
//   - [Scorer]       – calculates points awarded for solving a challenge.
//   - [FlagChecker]  – determines whether a submitted flag is correct.
//   - [Notifier]     – dispatches events (e.g. Slack/webhook notifications).
//
// All plugins are registered in the process-wide [Default] registry.
// Third-party plugins should call Default.Register* inside their own init().
package plugin

import (
	"math"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

// Plugin is the base interface that every plugin must satisfy.
type Plugin interface {
	// Name returns the unique identifier used to look up the plugin in the registry.
	Name() string
	// Init is called once after registration, passing any key-value settings
	// from the config file.  Return an error to abort startup.
	Init(cfg map[string]any) error
}

// Event carries a typed payload dispatched to Notifier plugins.
type Event struct {
	Type string
	Data map[string]any
}

type HomeWidget struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
}

type AdminMenuEntry struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Route string `json:"route"`
}

type ChallengeTypeSpec struct {
	ID                     string `json:"id"`
	Title                  string `json:"title"`
	Description            string `json:"description,omitempty"`
	SubmissionMode         string `json:"submission_mode"`
	AccessMode             string `json:"access_mode"`
	DefaultDeployType      string `json:"default_deploy_type,omitempty"`
	RequiresDeploy         bool   `json:"requires_deploy"`
	SupportsManualFlags    bool   `json:"supports_manual_flags"`
	SupportsCheckerConfig  bool   `json:"supports_checker_config"`
	SupportsFiles          bool   `json:"supports_files"`
	SupportsHints          bool   `json:"supports_hints"`
	SupportsConnectionInfo bool   `json:"supports_connection_info"`
	SupportsVPN            bool   `json:"supports_vpn"`
	SupportsExploitUpload  bool   `json:"supports_exploit_upload"`
	EngineManaged          bool   `json:"engine_managed"`
}

type ChallengeTypeDescriber interface {
	Descriptor() ChallengeTypeSpec
}

// Scorer calculates the points awarded to a user for solving a challenge.
// Register custom implementations via Default.RegisterScorer().
type Scorer interface {
	Plugin
	CalculateScore(challenge *models.Challenge, solveCount int) int
}

// Notifier receives platform events and forwards them to external systems
// (e.g. Slack, Discord, webhooks).
type Notifier interface {
	Plugin
	Notify(event Event) error
}

// ChallengeTypeExtension defines a custom challenge type that can be registered
// by a plugin. Custom types appear in the Admin challenge creation form.
type ChallengeTypeExtension interface {
	Plugin
	// TypeID is the unique string used as flag_type / challenge type identifier.
	TypeID() string
	// Verify checks whether submitted is correct for this challenge type.
	// The data parameter holds any plugin-specific config stored in the challenge.
	Verify(challengeData, submitted string) bool
}

// StaticScorer returns a fixed point value for challenges regardless of solve count.
// It simply returns the challenge's configured Points field.
type StaticScorer struct{}

func (s *StaticScorer) Name() string                { return "static" }
func (s *StaticScorer) Init(_ map[string]any) error { return nil }
func (s *StaticScorer) CalculateScore(challenge *models.Challenge, _ int) int {
	return challenge.Points
}

// DynamicScorer adjusts challenge points using an exponential decay formula based on solve count.
// Points decrease from MaxPoints toward MinPoints as more teams solve the challenge,
// following: score = MinPoints + (MaxPoints - MinPoints) * exp(-Decay * solveCount).
type DynamicScorer struct {
	MinPoints int
	MaxPoints int
	Decay     float64
}

func (d *DynamicScorer) Name() string { return "dynamic" }
func (d *DynamicScorer) Init(_ map[string]any) error {
	if d.MinPoints == 0 {
		d.MinPoints = 50
	}
	if d.Decay == 0 {
		d.Decay = 0.05
	}
	return nil
}
func (d *DynamicScorer) CalculateScore(challenge *models.Challenge, solveCount int) int {
	maxPts := d.MaxPoints
	if maxPts == 0 {
		maxPts = challenge.Points
	}
	// Exponential decay: score approaches MinPoints as solveCount grows.
	score := float64(d.MinPoints) + float64(maxPts-d.MinPoints)*math.Exp(-d.Decay*float64(solveCount))
	if score < float64(d.MinPoints) {
		score = float64(d.MinPoints)
	}
	return int(score)
}
