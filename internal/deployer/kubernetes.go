package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
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
	if req.Image == "" {
		return nil, fmt.Errorf("kubernetes deployer: image is required")
	}
	ns := k.Namespace
	if ns == "" {
		ns = "default"
	}
	name := fmt.Sprintf("ctf-%d-%d", req.ChallengeID, time.Now().UnixNano())
	env := []map[string]string{}
	for key, value := range stringMap(req.DeployConfig["env"]) {
		env = append(env, map[string]string{"name": key, "value": value})
	}

	labels := map[string]string{
		"app":                   "hecstack-challenge",
		"hecstack.instance":     name,
		"hecstack.challenge_id": fmt.Sprint(req.ChallengeID),
	}
	if req.UserID != nil {
		labels["hecstack.user_id"] = fmt.Sprint(*req.UserID)
	}
	if req.TeamID != nil {
		labels["hecstack.team_id"] = fmt.Sprint(*req.TeamID)
	}

	container := map[string]any{
		"name":  "challenge",
		"image": req.Image,
		"env":   env,
	}
	if command, ok := req.DeployConfig["command"].([]any); ok && len(command) > 0 {
		values := make([]string, 0, len(command))
		for _, part := range command {
			values = append(values, fmt.Sprint(part))
		}
		container["command"] = values
	}
	if workdir, ok := req.DeployConfig["workdir"].(string); ok && strings.TrimSpace(workdir) != "" {
		container["workingDir"] = workdir
	}
	if resources := k8sResourceRequirements(req.DeployConfig); len(resources) > 0 {
		container["resources"] = resources
	}

	portMap := stringMap(req.DeployConfig["ports"])
	containerPorts := []map[string]any{}
	servicePorts := []map[string]any{}
	serviceType := strings.TrimSpace(fmt.Sprint(req.DeployConfig["service_type"]))
	if serviceType == "" {
		serviceType = "ClusterIP"
	}
	for containerPort, hostPort := range portMap {
		portNum := parseContainerPort(containerPort)
		if portNum == 0 {
			continue
		}
		containerPorts = append(containerPorts, map[string]any{
			"containerPort": portNum,
			"protocol":      "TCP",
		})
		servicePort := map[string]any{
			"name":       fmt.Sprintf("tcp-%d", portNum),
			"port":       portNum,
			"targetPort": portNum,
			"protocol":   "TCP",
		}
		if strings.EqualFold(serviceType, "NodePort") && strings.TrimSpace(hostPort) != "" {
			if nodePort, err := strconv.Atoi(hostPort); err == nil && nodePort > 0 {
				servicePort["nodePort"] = nodePort
			}
		}
		servicePorts = append(servicePorts, servicePort)
	}
	if len(containerPorts) > 0 {
		container["ports"] = containerPorts
	}

	pod := map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      name,
			"namespace": ns,
			"labels":    labels,
		},
		"spec": map[string]any{
			"restartPolicy": "Never",
			"containers":    []map[string]any{container},
		},
	}
	if pullSecrets := k8sImagePullSecrets(req.DeployConfig); len(pullSecrets) > 0 {
		pod["spec"].(map[string]any)["imagePullSecrets"] = pullSecrets
	}

	items := []any{pod}
	if len(servicePorts) > 0 {
		service := map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      name,
				"namespace": ns,
				"labels":    labels,
			},
			"spec": map[string]any{
				"type":     serviceType,
				"selector": map[string]string{"hecstack.instance": name},
				"ports":    servicePorts,
			},
		}
		items = append(items, service)
	}

	manifestBytes, _ := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "List",
		"items":      items,
	})

	args := []string{"apply", "-f", "-"}
	if k.Kubeconfig != "" {
		args = append([]string{"--kubeconfig", k.Kubeconfig}, args...)
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = strings.NewReader(string(manifestBytes))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("kubectl apply failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	conn := map[string]any{
		"pod":       name,
		"namespace": ns,
	}
	if len(servicePorts) > 0 {
		conn["service"] = name
		conn["service_type"] = serviceType
		conn["ports"] = portMap
		if serviceConn := k.serviceConnectionInfo(ctx, ns, name); len(serviceConn) > 0 {
			for key, value := range serviceConn {
				conn[key] = value
			}
		}
	}
	connInfo, _ := json.Marshal(conn)

	inst := &models.Instance{
		ChallengeID:    req.ChallengeID,
		UserID:         req.UserID,
		TeamID:         req.TeamID,
		InstanceType:   req.DeployType,
		Backend:        "kubernetes",
		InstanceID:     name,
		ConnectionInfo: string(connInfo),
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
	_, _ = k.kubectl(ctx, "delete", "service", instanceID, "-n", ns, "--ignore-not-found")
	out, err := k.kubectl(ctx, "delete", "pod", instanceID, "-n", ns, "--ignore-not-found")
	if err != nil {
		return fmt.Errorf("kubectl delete failed: %w: %s", err, strings.TrimSpace(string(out)))
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

func k8sResourceRequirements(deployConfig map[string]any) map[string]any {
	cpus := strings.TrimSpace(fmt.Sprint(deployConfig["cpus"]))
	memory := strings.TrimSpace(fmt.Sprint(deployConfig["memory"]))
	if cpus == "" || cpus == "<nil>" {
		cpus = ""
	}
	if memory == "" || memory == "<nil>" {
		memory = ""
	}
	if cpus == "" && memory == "" {
		if resources, ok := deployConfig["resources"].(map[string]any); ok {
			if cpus == "" {
				cpus = strings.TrimSpace(fmt.Sprint(resources["cpus"]))
			}
			if memory == "" {
				memory = strings.TrimSpace(fmt.Sprint(resources["memory"]))
			}
		}
	}
	if cpus == "" && memory == "" {
		return nil
	}
	limits := map[string]string{}
	if cpus != "" && cpus != "<nil>" {
		limits["cpu"] = cpus
	}
	if memory != "" && memory != "<nil>" {
		limits["memory"] = memory
	}
	return map[string]any{
		"requests": limits,
		"limits":   limits,
	}
}

func parseContainerPort(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func k8sImagePullSecrets(deployConfig map[string]any) []map[string]string {
	seen := map[string]bool{}
	var names []string
	if value := strings.TrimSpace(fmt.Sprint(deployConfig["image_pull_secret"])); value != "" && value != "<nil>" {
		names = append(names, value)
	}
	for _, value := range stringSlice(deployConfig["image_pull_secrets"]) {
		names = append(names, value)
	}
	out := make([]map[string]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, map[string]string{"name": name})
	}
	return out
}

func (k *K8sDeployer) serviceConnectionInfo(ctx context.Context, namespace, name string) map[string]any {
	out, err := k.kubectl(ctx, "get", "service", name, "-n", namespace, "-o", "json")
	if err != nil {
		return nil
	}
	var payload struct {
		Spec struct {
			ClusterIP string `json:"clusterIP"`
			Ports     []struct {
				Port     int `json:"port"`
				NodePort int `json:"nodePort"`
			} `json:"ports"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil
	}
	info := map[string]any{}
	if payload.Spec.ClusterIP != "" && payload.Spec.ClusterIP != "None" {
		info["ip"] = payload.Spec.ClusterIP
	}
	if len(payload.Spec.Ports) > 0 {
		port := payload.Spec.Ports[0].Port
		if payload.Spec.Ports[0].NodePort > 0 {
			port = payload.Spec.Ports[0].NodePort
		}
		if port > 0 {
			info["port"] = port
		}
	}
	return info
}
