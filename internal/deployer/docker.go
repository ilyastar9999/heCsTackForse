package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type DockerDeployer struct {
	Registry       string
	Host           string
	TLSVerify      bool
	CertPath       string
	Network        string
	ChallengesDir  string
	LocalTagPrefix string
}

func (d *DockerDeployer) Type() string { return "docker" }

func (d *DockerDeployer) Deploy(ctx context.Context, req DeployRequest) (*models.Instance, error) {
	image, err := d.resolveImage(ctx, req)
	if err != nil {
		return nil, err
	}

	containerName := fmt.Sprintf("ctf-%d-%d", req.ChallengeID, time.Now().UnixNano())
	args := []string{"run", "-d", "--name", containerName, "--label", fmt.Sprintf("hecstack.challenge_id=%d", req.ChallengeID)}
	if d.Network != "" {
		args = append(args, "--network", d.Network)
	}
	if req.UserID != nil {
		args = append(args, "--label", fmt.Sprintf("hecstack.user_id=%d", *req.UserID))
	}
	if req.TeamID != nil {
		args = append(args, "--label", fmt.Sprintf("hecstack.team_id=%d", *req.TeamID))
	}
	for k, v := range stringMap(req.DeployConfig["env"]) {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	for containerPort, hostPort := range stringMap(req.DeployConfig["ports"]) {
		if strings.TrimSpace(hostPort) == "" {
			args = append(args, "-p", containerPort)
			continue
		}
		args = append(args, "-p", fmt.Sprintf("%s:%s", hostPort, containerPort))
	}
	for _, volume := range stringSlice(req.DeployConfig["volumes"]) {
		args = append(args, "-v", volume)
	}
	if workdir, ok := req.DeployConfig["workdir"].(string); ok && workdir != "" {
		args = append(args, "-w", workdir)
	}
	if memory, ok := req.DeployConfig["memory"].(string); ok && memory != "" {
		args = append(args, "--memory", memory)
	}
	if cpus, ok := req.DeployConfig["cpus"].(string); ok && cpus != "" {
		args = append(args, "--cpus", cpus)
	}
	if restart, ok := req.DeployConfig["restart"].(string); ok && restart != "" {
		args = append(args, "--restart", restart)
	}
	args = append(args, image)
	if command, ok := req.DeployConfig["command"].([]any); ok {
		for _, part := range command {
			args = append(args, fmt.Sprint(part))
		}
	}

	out, err := d.dockerCommandContext(ctx, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker run failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	containerID := strings.TrimSpace(string(out))

	conn := map[string]any{
		"container": containerName,
		"image":     image,
	}
	if ports := stringMap(req.DeployConfig["ports"]); len(ports) > 0 {
		conn["ports"] = d.publishedPorts(ctx, containerID, ports)
	}
	connInfo, _ := json.Marshal(conn)
	inst := &models.Instance{
		ChallengeID:    req.ChallengeID,
		UserID:         req.UserID,
		TeamID:         req.TeamID,
		InstanceType:   req.DeployType,
		Backend:        "docker",
		InstanceID:     containerID,
		ConnectionInfo: string(connInfo),
		Status:         "running",
		CreatedAt:      time.Now(),
	}
	return inst, nil
}

func (d *DockerDeployer) resolveImage(ctx context.Context, req DeployRequest) (string, error) {
	image := strings.TrimSpace(req.Image)
	buildContext, _ := req.DeployConfig["build_context"].(string)
	if strings.TrimSpace(buildContext) == "" {
		if image == "" {
			return "", fmt.Errorf("docker deployer: image is required when build_context is not set")
		}
		return image, nil
	}

	resolvedContext, err := d.resolveBuildContext(buildContext)
	if err != nil {
		return "", err
	}

	dockerfile := "Dockerfile"
	if customDockerfile, ok := req.DeployConfig["dockerfile"].(string); ok && strings.TrimSpace(customDockerfile) != "" {
		dockerfile = strings.TrimSpace(customDockerfile)
	}
	dockerfilePath := dockerfile
	if !filepath.IsAbs(dockerfilePath) {
		dockerfilePath = filepath.Join(resolvedContext, dockerfilePath)
	}
	if _, err := os.Stat(dockerfilePath); err != nil {
		return "", fmt.Errorf("docker deployer: dockerfile not found: %s", dockerfilePath)
	}

	if image == "" {
		image = d.defaultLocalImageTag(req.ChallengeID, resolvedContext)
	}

	buildArgs := []string{"build", "-t", image, "-f", dockerfilePath}
	for key, value := range stringMap(req.DeployConfig["build_args"]) {
		buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}
	if noCache, ok := req.DeployConfig["build_no_cache"].(bool); ok && noCache {
		buildArgs = append(buildArgs, "--no-cache")
	}
	buildArgs = append(buildArgs, resolvedContext)

	out, err := d.dockerCommandContext(ctx, buildArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker build failed for %s: %w: %s", image, err, strings.TrimSpace(string(out)))
	}
	return image, nil
}

func (d *DockerDeployer) resolveBuildContext(buildContext string) (string, error) {
	root := d.ChallengesDir
	if strings.TrimSpace(root) == "" {
		root = "./challenges"
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("docker deployer: failed to resolve challenges_dir: %w", err)
	}

	candidate := buildContext
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootAbs, filepath.Clean(buildContext))
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("docker deployer: failed to resolve build_context: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return "", fmt.Errorf("docker deployer: failed to validate build_context: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("docker deployer: build_context must stay inside challenges_dir (%s)", rootAbs)
	}
	info, err := os.Stat(candidateAbs)
	if err != nil {
		return "", fmt.Errorf("docker deployer: build_context not found: %s", candidateAbs)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("docker deployer: build_context must be a directory: %s", candidateAbs)
	}
	return candidateAbs, nil
}

func (d *DockerDeployer) defaultLocalImageTag(challengeID int64, resolvedContext string) string {
	prefix := strings.TrimSpace(d.LocalTagPrefix)
	if prefix == "" {
		prefix = "hecstack/"
	}
	if !strings.HasSuffix(prefix, "/") && !strings.HasSuffix(prefix, ":") {
		prefix += "/"
	}
	base := sanitizeDockerName(filepath.Base(resolvedContext))
	if base == "" {
		base = fmt.Sprintf("challenge-%d", challengeID)
	}
	return fmt.Sprintf("%s%s:latest", prefix, base)
}

func stringMap(value any) map[string]string {
	out := map[string]string{}
	raw, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range raw {
		if k != "" {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

var dockerNameSanitizer = regexp.MustCompile(`[^a-z0-9._/-]+`)

func sanitizeDockerName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = dockerNameSanitizer.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-./")
	return name
}

func (d *DockerDeployer) Destroy(ctx context.Context, instanceID string) error {
	if out, err := d.dockerCommandContext(ctx, "rm", "-f", instanceID).CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm failed: %w: %s", err, out)
	}
	return nil
}

func (d *DockerDeployer) Status(ctx context.Context, instanceID string) (InstanceStatus, error) {
	out, err := d.dockerCommandContext(ctx, "inspect", "-f", "{{.State.Status}}", instanceID).Output()
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

func (d *DockerDeployer) dockerCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = os.Environ()
	if strings.TrimSpace(d.Host) != "" {
		cmd.Env = append(cmd.Env, "DOCKER_HOST="+strings.TrimSpace(d.Host))
	}
	if d.TLSVerify {
		cmd.Env = append(cmd.Env, "DOCKER_TLS_VERIFY=1")
	}
	if strings.TrimSpace(d.CertPath) != "" {
		cmd.Env = append(cmd.Env, "DOCKER_CERT_PATH="+strings.TrimSpace(d.CertPath))
	}
	return cmd
}

func (d *DockerDeployer) publishedPorts(ctx context.Context, instanceID string, requested map[string]string) map[string]any {
	out, err := d.dockerCommandContext(ctx, "port", instanceID).CombinedOutput()
	if err != nil {
		return map[string]any{"requested": requested}
	}

	published := map[string]any{}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " -> ", 2)
		if len(parts) != 2 {
			continue
		}
		containerPort := strings.TrimSpace(parts[0])
		bind := strings.TrimSpace(parts[1])
		host := bind
		port := bind
		if idx := strings.LastIndex(bind, ":"); idx >= 0 && idx < len(bind)-1 {
			host = strings.Trim(bind[:idx], "[]")
			port = bind[idx+1:]
		}
		entry := map[string]any{"host": host, "port": port}
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			entry["port"] = p
		}
		published[containerPort] = entry
	}
	if len(published) == 0 {
		return map[string]any{"requested": requested}
	}
	return published
}
