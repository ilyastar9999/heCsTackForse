package deployer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type InstanceStatus string

const (
	StatusRunning InstanceStatus = "running"
	StatusStopped InstanceStatus = "stopped"
	StatusError   InstanceStatus = "error"
	StatusPending InstanceStatus = "pending"
)

type DeployRequest struct {
	ChallengeID  int64
	UserID       *int64
	TeamID       *int64
	Image        string
	VMTemplate   int
	DeployConfig map[string]any
	DeployType   string
	Backend      string
	InstanceTTL  string
}

type Deployer interface {
	Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error)
	Destroy(ctx context.Context, instanceID string) error
	Status(ctx context.Context, instanceID string) (InstanceStatus, error)
	Type() string
}

type TargetConfig struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Enabled      bool    `json:"enabled"`
	Order        int     `json:"order"`
	Weight       int     `json:"weight"`
	MaxInstances int     `json:"max_instances"`
	MaxCPU       float64 `json:"max_cpu"`
	MaxMemoryMB  int     `json:"max_memory_mb"`

	Network        string `json:"network,omitempty"`
	Host           string `json:"host,omitempty"`
	TLSVerify      bool   `json:"tls_verify,omitempty"`
	CertPath       string `json:"cert_path,omitempty"`
	Kubeconfig     string `json:"kubeconfig,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	Registry       string `json:"registry,omitempty"`
	ChallengesDir  string `json:"challenges_dir,omitempty"`
	LocalTagPrefix string `json:"local_tag_prefix,omitempty"`
}

type SchedulerConfig struct {
	Mode    string         `json:"mode"`
	Targets []TargetConfig `json:"targets"`
}

type TargetUsage struct {
	Instances int
	CPU       float64
	MemoryMB  int
}

type registeredTarget struct {
	cfg      TargetConfig
	deployer Deployer
}

type Selection struct {
	ID       string
	Type     string
	Deployer Deployer
}

type Manager struct {
	mu       sync.RWMutex
	backends map[string]Deployer
	targets  map[string]registeredTarget
	mode     string
	rrNext   int
}

func NewManager() *Manager {
	return &Manager{
		backends: make(map[string]Deployer),
		targets:  make(map[string]registeredTarget),
		mode:     "ordered",
	}
}

func (m *Manager) Register(d Deployer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backends[d.Type()] = d
}

func (m *Manager) UpsertTarget(cfg TargetConfig, d Deployer) {
	if cfg.ID == "" {
		cfg.ID = cfg.Type
	}
	if cfg.Weight <= 0 {
		cfg.Weight = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targets[cfg.ID] = registeredTarget{cfg: cfg, deployer: d}
}

func (m *Manager) ReplaceTargets(mode string, targets []TargetConfig, factory func(TargetConfig) (Deployer, error)) error {
	if mode == "" {
		mode = "ordered"
	}
	newTargets := make(map[string]registeredTarget, len(targets))
	for _, cfg := range targets {
		if cfg.ID == "" || cfg.Type == "" || !cfg.Enabled {
			continue
		}
		if cfg.Weight <= 0 {
			cfg.Weight = 1
		}
		d, err := factory(cfg)
		if err != nil {
			return fmt.Errorf("target %s: %w", cfg.ID, err)
		}
		newTargets[cfg.ID] = registeredTarget{cfg: cfg, deployer: d}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targets = newTargets
	m.mode = mode
	m.rrNext = 0
	return nil
}

func (m *Manager) SchedulerConfig() SchedulerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := SchedulerConfig{
		Mode:    m.mode,
		Targets: make([]TargetConfig, 0, len(m.targets)),
	}
	for _, target := range m.targets {
		cfg.Targets = append(cfg.Targets, target.cfg)
	}
	sort.Slice(cfg.Targets, func(i, j int) bool {
		if cfg.Targets[i].Order == cfg.Targets[j].Order {
			return cfg.Targets[i].ID < cfg.Targets[j].ID
		}
		return cfg.Targets[i].Order < cfg.Targets[j].Order
	})
	return cfg
}

func (m *Manager) Select(selector string, usage map[string]TargetUsage, cpu float64, memoryMB int) (Selection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if target, ok := m.targets[selector]; ok {
		if !targetHasCapacity(target.cfg, usage[selector], cpu, memoryMB) {
			return Selection{}, fmt.Errorf("deploy target %s is at capacity", selector)
		}
		return Selection{ID: selector, Type: target.cfg.Type, Deployer: target.deployer}, nil
	}

	candidates := make([]registeredTarget, 0, len(m.targets))
	for _, target := range m.targets {
		if !target.cfg.Enabled {
			continue
		}
		if selector != "" && selector != "auto" && target.cfg.Type != selector {
			continue
		}
		if !targetHasCapacity(target.cfg, usage[target.cfg.ID], cpu, memoryMB) {
			continue
		}
		candidates = append(candidates, target)
	}
	if len(candidates) == 0 {
		return Selection{}, fmt.Errorf("no deploy targets available for selector %q", selector)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].cfg.Order == candidates[j].cfg.Order {
			return candidates[i].cfg.ID < candidates[j].cfg.ID
		}
		return candidates[i].cfg.Order < candidates[j].cfg.Order
	})

	if m.mode == "balanced" {
		idx := m.rrNext % len(candidates)
		m.rrNext++
		target := candidates[idx]
		return Selection{ID: target.cfg.ID, Type: target.cfg.Type, Deployer: target.deployer}, nil
	}

	target := candidates[0]
	return Selection{ID: target.cfg.ID, Type: target.cfg.Type, Deployer: target.deployer}, nil
}

func targetHasCapacity(cfg TargetConfig, usage TargetUsage, cpu float64, memoryMB int) bool {
	if cfg.MaxInstances > 0 && usage.Instances >= cfg.MaxInstances {
		return false
	}
	if cfg.MaxCPU > 0 && cpu > 0 && usage.CPU+cpu > cfg.MaxCPU {
		return false
	}
	if cfg.MaxMemoryMB > 0 && memoryMB > 0 && usage.MemoryMB+memoryMB > cfg.MaxMemoryMB {
		return false
	}
	return true
}

func (m *Manager) Get(name string) (Deployer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if target, ok := m.targets[name]; ok {
		return target.deployer, nil
	}
	if d, ok := m.backends[name]; ok {
		return d, nil
	}
	for _, target := range m.targets {
		if target.cfg.Type == name {
			return target.deployer, nil
		}
	}
	return nil, fmt.Errorf("unknown deployer backend: %s", name)
}

func NormalizeSchedulerMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "balanced":
		return "balanced"
	default:
		return "ordered"
	}
}
