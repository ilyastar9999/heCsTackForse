package deployer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type K8sDeployer struct {
	Kubeconfig string
	Namespace  string
}

func (k *K8sDeployer) Type() string { return "kubernetes" }

func (k *K8sDeployer) kubectl(ctx context.Context, args ...string) ([]byte, error) {
	cmdArgs := args
	if k.Kubeconfig != "" {
		cmdArgs = append([]string{"--kubeconfig", k.Kubeconfig}, args...)
	}
	return exec.CommandContext(ctx, "kubectl", cmdArgs...).CombinedOutput()
}

func (k *K8sDeployer) Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error) {
	ns := k.Namespace
	if ns == "" {
		ns = "default"
	}
	name := fmt.Sprintf("ctf-%d-%d", req.ChallengeID, time.Now().UnixNano())
	manifest := fmt.Sprintf(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"%s","namespace":"%s"},"spec":{"containers":[{"name":"challenge","image":"%s"}]}}`, name, ns, req.Image)
	// Pass the manifest via stdin with kubectl apply -f -
	args := []string{"apply", "-f", "-"}
	if k.Kubeconfig != "" {
		args = append([]string{"--kubeconfig", k.Kubeconfig}, args...)
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = strings.NewReader(manifest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("kubectl apply failed: %w: %s", err, out)
	}
	inst := &models.Instance{
		ChallengeID:    req.ChallengeID,
		UserID:         req.UserID,
		TeamID:         req.TeamID,
		InstanceType:   req.DeployType,
		Backend:        "kubernetes",
		InstanceID:     name,
		ConnectionInfo: fmt.Sprintf(`{"pod":"%s","namespace":"%s"}`, name, ns),
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	return inst, nil
}

func (k *K8sDeployer) Destroy(ctx context.Context, instanceID string) error {
	ns := k.Namespace
	if ns == "" {
		ns = "default"
	}
	out, err := k.kubectl(ctx, "delete", "pod", instanceID, "-n", ns, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("kubectl delete failed: %w: %s", err, out)
	}
	return nil
}

func (k *K8sDeployer) Status(ctx context.Context, instanceID string) (InstanceStatus, error) {
	ns := k.Namespace
	if ns == "" {
		ns = "default"
	}
	out, err := k.kubectl(ctx, "get", "pod", instanceID, "-n", ns, "-o", "jsonpath={.status.phase}")
	if err != nil {
		return StatusError, nil
	}
	switch strings.TrimSpace(string(out)) {
	case "Running":
		return StatusRunning, nil
	case "Succeeded", "Failed":
		return StatusStopped, nil
	default:
		return StatusPending, nil
	}
}
