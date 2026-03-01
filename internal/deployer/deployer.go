package deployer

import (
	"context"
	"fmt"

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

type Manager struct {
	backends map[string]Deployer
}

func NewManager() *Manager {
	return &Manager{backends: make(map[string]Deployer)}
}

func (m *Manager) Register(d Deployer) {
	m.backends[d.Type()] = d
}

func (m *Manager) Get(backendType string) (Deployer, error) {
	d, ok := m.backends[backendType]
	if !ok {
		return nil, fmt.Errorf("unknown deployer backend: %s", backendType)
	}
	return d, nil
}
