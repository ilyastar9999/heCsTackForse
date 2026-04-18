package deployer

import (
	"context"
	"fmt"
	"sync"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

// SingleInstanceDeployer wraps another deployer and ensures only one shared instance per challenge.
type SingleInstanceDeployer struct {
	mu        sync.RWMutex
	backend   Deployer
	instances map[int64]*models.Instance
}

func NewSingleInstanceDeployer(backend Deployer) *SingleInstanceDeployer {
	return &SingleInstanceDeployer{
		backend:   backend,
		instances: make(map[int64]*models.Instance),
	}
}

func (s *SingleInstanceDeployer) Type() string { return "single_instance" }

func (s *SingleInstanceDeployer) Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error) {
	s.mu.RLock()
	inst, ok := s.instances[req.ChallengeID]
	s.mu.RUnlock()
	if ok {
		return inst, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after acquiring write lock.
	if inst, ok = s.instances[req.ChallengeID]; ok {
		return inst, nil
	}
	inst, err := s.backend.Deploy(ctx, req)
	if err != nil {
		return nil, err
	}
	s.instances[req.ChallengeID] = inst
	return inst, nil
}

func (s *SingleInstanceDeployer) Destroy(ctx context.Context, instanceID string) error {
	s.mu.Lock()
	for k, v := range s.instances {
		if v.InstanceID == instanceID {
			delete(s.instances, k)
			break
		}
	}
	s.mu.Unlock()
	return s.backend.Destroy(ctx, instanceID)
}

func (s *SingleInstanceDeployer) Status(ctx context.Context, instanceID string) (InstanceStatus, error) {
	return s.backend.Status(ctx, instanceID)
}

// NoDeployDeployer is the deployer for challenges that don't need deployment.
type NoDeployDeployer struct{}

func (n *NoDeployDeployer) Type() string { return "no_deploy" }

func (n *NoDeployDeployer) Deploy(_ context.Context, _ DeployRequest) (*models.Instance, error) {
	return nil, fmt.Errorf("this challenge requires no deployment")
}

func (n *NoDeployDeployer) Destroy(_ context.Context, _ string) error {
	return nil
}

func (n *NoDeployDeployer) Status(_ context.Context, _ string) (InstanceStatus, error) {
	return StatusStopped, nil
}
