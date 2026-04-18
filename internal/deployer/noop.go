package deployer

import (
	"context"
	"fmt"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type NoopDeployer struct{}

func (n *NoopDeployer) Type() string { return "noop" }

func (n *NoopDeployer) Deploy(_ context.Context, _ DeployRequest) (*models.Instance, error) {
	return nil, fmt.Errorf("no deployment needed for this challenge")
}

func (n *NoopDeployer) Destroy(_ context.Context, _ string) error {
	return fmt.Errorf("no deployment to destroy")
}

func (n *NoopDeployer) Status(_ context.Context, _ string) (InstanceStatus, error) {
	return StatusStopped, nil
}
