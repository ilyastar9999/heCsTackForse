package deployer

import (
	"context"
	"fmt"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

// ADDeployer handles attack & defense challenge deployments.
type ADDeployer struct {
	backend       Deployer
	RoundDuration time.Duration
	FlagLifetime  int
	stopCh        chan struct{}
}

func NewADDeployer(backend Deployer, roundDuration time.Duration, flagLifetime int) *ADDeployer {
	return &ADDeployer{
		backend:       backend,
		RoundDuration: roundDuration,
		FlagLifetime:  flagLifetime,
		stopCh:        make(chan struct{}),
	}
}

func (a *ADDeployer) Type() string { return "attack_defence" }

func (a *ADDeployer) Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error) {
	inst, err := a.backend.Deploy(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ad deploy: %w", err)
	}
	return inst, nil
}

func (a *ADDeployer) Destroy(ctx context.Context, instanceID string) error {
	return a.backend.Destroy(ctx, instanceID)
}

func (a *ADDeployer) Status(ctx context.Context, instanceID string) (InstanceStatus, error) {
	return a.backend.Status(ctx, instanceID)
}

// StartFlagRotation starts a goroutine that rotates flags every round.
func (a *ADDeployer) StartFlagRotation(onRotate func(round int)) {
	go func() {
		round := 0
		ticker := time.NewTicker(a.RoundDuration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				round++
				if onRotate != nil {
					onRotate(round)
				}
			case <-a.stopCh:
				return
			}
		}
	}()
}

// Stop stops the flag rotation goroutine.
func (a *ADDeployer) Stop() {
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}
}

// GenerateADFlag generates a flag for A&D challenges.
func GenerateADFlag(prefix string, challengeID, teamID int64, round int) string {
	return fmt.Sprintf("%s_AD_%d_%d_%d_%d}", prefix, challengeID, teamID, round, time.Now().UnixNano()%100000)
}
