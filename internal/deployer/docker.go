package deployer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type DockerDeployer struct {
	Registry string
	Network  string
}

func (d *DockerDeployer) Type() string { return "docker" }

func (d *DockerDeployer) Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error) {
	image := req.Image
	if image == "" {
		return nil, fmt.Errorf("docker deployer: image is required")
	}
	containerName := fmt.Sprintf("ctf-%d-%d", req.ChallengeID, time.Now().UnixNano())
	args := []string{"run", "-d", "--name", containerName}
	if d.Network != "" {
		args = append(args, "--network", d.Network)
	}
	args = append(args, image)
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker run failed: %w", err)
	}
	containerID := strings.TrimSpace(string(out))
	inst := &models.Instance{
		ChallengeID:    req.ChallengeID,
		UserID:         req.UserID,
		TeamID:         req.TeamID,
		InstanceType:   req.DeployType,
		Backend:        "docker",
		InstanceID:     containerID,
		ConnectionInfo: fmt.Sprintf(`{"container":"%s"}`, containerName),
		Status:         "running",
		CreatedAt:      time.Now(),
	}
	return inst, nil
}

func (d *DockerDeployer) Destroy(ctx context.Context, instanceID string) error {
	if out, err := exec.CommandContext(ctx, "docker", "rm", "-f", instanceID).CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm failed: %w: %s", err, out)
	}
	return nil
}

func (d *DockerDeployer) Status(ctx context.Context, instanceID string) (InstanceStatus, error) {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}", instanceID).Output()
	if err != nil {
		return StatusError, nil
	}
	switch strings.TrimSpace(string(out)) {
	case "running":
		return StatusRunning, nil
	case "exited", "dead":
		return StatusStopped, nil
	default:
		return StatusPending, nil
	}
}
