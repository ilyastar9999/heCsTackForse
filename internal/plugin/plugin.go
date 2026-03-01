package plugin

import (
	"math"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type Plugin interface {
	Name() string
	Init(cfg map[string]any) error
}

type Event struct {
	Type string
	Data map[string]any
}

type Scorer interface {
	Plugin
	CalculateScore(challenge *models.Challenge, solveCount int) int
}

type Notifier interface {
	Plugin
	Notify(event Event) error
}

type StaticScorer struct{}

func (s *StaticScorer) Name() string                { return "static" }
func (s *StaticScorer) Init(_ map[string]any) error { return nil }
func (s *StaticScorer) CalculateScore(challenge *models.Challenge, _ int) int {
	return challenge.Points
}

type DynamicScorer struct {
	MinPoints int
	MaxPoints int
	Decay     float64
}

func (d *DynamicScorer) Name() string                { return "dynamic" }
func (d *DynamicScorer) Init(_ map[string]any) error { return nil }
func (d *DynamicScorer) CalculateScore(challenge *models.Challenge, solveCount int) int {
	if d.MaxPoints == 0 {
		d.MaxPoints = challenge.Points
	}
	if d.MinPoints == 0 {
		d.MinPoints = 50
	}
	if d.Decay == 0 {
		d.Decay = 0.05
	}
	// Exponential decay: score approaches MinPoints as solveCount grows.
	score := float64(d.MinPoints) + float64(d.MaxPoints-d.MinPoints)*math.Exp(-d.Decay*float64(solveCount))
	if score < float64(d.MinPoints) {
		score = float64(d.MinPoints)
	}
	return int(score)
}
