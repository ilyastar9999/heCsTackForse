package deployer

import (
	"context"
	"fmt"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

// PerUserDeployer deploys a separate instance for each user/team.
type PerUserDeployer struct {
	backend     Deployer
	InstanceTTL time.Duration
}

func NewPerUserDeployer(backend Deployer, ttl time.Duration) *PerUserDeployer {
	return &PerUserDeployer{backend: backend, InstanceTTL: ttl}
}

func (p *PerUserDeployer) Type() string { return "per_user" }

func (p *PerUserDeployer) Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error) {
	inst, err := p.backend.Deploy(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("per_user deploy: %w", err)
	}
	if p.InstanceTTL > 0 {
		exp := time.Now().Add(p.InstanceTTL)
		inst.ExpiresAt = &exp
	}
	return inst, nil
}

func (p *PerUserDeployer) Destroy(ctx context.Context, instanceID string) error {
	return p.backend.Destroy(ctx, instanceID)
}

func (p *PerUserDeployer) Status(ctx context.Context, instanceID string) (InstanceStatus, error) {
	return p.backend.Status(ctx, instanceID)
}
