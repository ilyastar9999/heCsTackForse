package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
)

const deploySchedulerSettingsKey = "deploy_scheduler"

func (s *Server) bootstrapDeployScheduler() error {
	cfg, found, err := s.loadPersistedSchedulerConfig()
	if err != nil {
		return err
	}
	if !found {
		cfg = s.defaultSchedulerConfig()
	}
	return s.applyDeploySchedulerConfig(cfg)
}

func (s *Server) loadPersistedSchedulerConfig() (deployer.SchedulerConfig, bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM ctf_settings WHERE key=?`, deploySchedulerSettingsKey).Scan(&raw)
	if err != nil || strings.TrimSpace(raw) == "" {
		return deployer.SchedulerConfig{}, false, nil
	}
	var cfg deployer.SchedulerConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return deployer.SchedulerConfig{}, false, fmt.Errorf("invalid deploy scheduler config: %w", err)
	}
	return normalizeSchedulerConfig(cfg), true, nil
}

func (s *Server) defaultSchedulerConfig() deployer.SchedulerConfig {
	targets := make([]deployer.TargetConfig, 0, 2)
	order := 10
	if s.cfg.Deployer.Backends.Docker.Enabled {
		targets = append(targets, deployer.TargetConfig{
			ID:             "docker-default",
			Type:           "docker",
			Enabled:        true,
			Order:          order,
			Weight:         1,
			Host:           s.cfg.Deployer.Backends.Docker.Host,
			TLSVerify:      s.cfg.Deployer.Backends.Docker.TLSVerify,
			CertPath:       s.cfg.Deployer.Backends.Docker.CertPath,
			Network:        s.cfg.Deployer.Backends.Docker.Network,
			Registry:       s.cfg.Deployer.Backends.Docker.Registry,
			ChallengesDir:  s.cfg.Deployer.Backends.Docker.ChallengesDir,
			LocalTagPrefix: s.cfg.Deployer.Backends.Docker.LocalTagPrefix,
		})
		order += 10
	}
	if s.cfg.Deployer.Backends.Kubernetes.Enabled {
		targets = append(targets, deployer.TargetConfig{
			ID:         "kubernetes-default",
			Type:       "kubernetes",
			Enabled:    true,
			Order:      order,
			Weight:     1,
			Kubeconfig: s.cfg.Deployer.Backends.Kubernetes.Kubeconfig,
			Namespace:  s.cfg.Deployer.Backends.Kubernetes.Namespace,
		})
		order += 10
	}
	if s.cfg.Deployer.Backends.PVE.Enabled {
		targets = append(targets, deployer.TargetConfig{
			ID:      "pve-default",
			Type:    "pve",
			Enabled: true,
			Order:   order,
			Weight:  1,
		})
	}
	return deployer.SchedulerConfig{
		Mode:    "ordered",
		Targets: targets,
	}
}

func normalizeSchedulerConfig(cfg deployer.SchedulerConfig) deployer.SchedulerConfig {
	cfg.Mode = deployer.NormalizeSchedulerMode(cfg.Mode)
	for i := range cfg.Targets {
		target := &cfg.Targets[i]
		target.ID = strings.TrimSpace(target.ID)
		target.Type = strings.ToLower(strings.TrimSpace(target.Type))
		if target.Weight <= 0 {
			target.Weight = 1
		}
		if target.Order < 0 {
			target.Order = 0
		}
	}
	return cfg
}

func (s *Server) applyDeploySchedulerConfig(cfg deployer.SchedulerConfig) error {
	cfg = normalizeSchedulerConfig(cfg)
	return s.deployer.ReplaceTargets(cfg.Mode, cfg.Targets, s.deployTargetFactory)
}

func (s *Server) deployTargetFactory(cfg deployer.TargetConfig) (deployer.Deployer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "docker":
		return &deployer.DockerDeployer{
			Registry:       fallbackString(cfg.Registry, s.cfg.Deployer.Backends.Docker.Registry),
			Host:           fallbackString(cfg.Host, s.cfg.Deployer.Backends.Docker.Host),
			TLSVerify:      cfg.TLSVerify || s.cfg.Deployer.Backends.Docker.TLSVerify,
			CertPath:       fallbackString(cfg.CertPath, s.cfg.Deployer.Backends.Docker.CertPath),
			Network:        fallbackString(cfg.Network, s.cfg.Deployer.Backends.Docker.Network),
			ChallengesDir:  fallbackString(cfg.ChallengesDir, s.cfg.Deployer.Backends.Docker.ChallengesDir),
			LocalTagPrefix: fallbackString(cfg.LocalTagPrefix, s.cfg.Deployer.Backends.Docker.LocalTagPrefix),
		}, nil
	case "kubernetes":
		return &deployer.K8sDeployer{
			Kubeconfig: fallbackString(cfg.Kubeconfig, s.cfg.Deployer.Backends.Kubernetes.Kubeconfig),
			Namespace:  fallbackString(cfg.Namespace, s.cfg.Deployer.Backends.Kubernetes.Namespace),
		}, nil
	case "pve":
		return s.deployer.Get("pve")
	default:
		return nil, fmt.Errorf("unsupported deploy target type %q", cfg.Type)
	}
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func (s *Server) handleAdminGetDeployConfig(c echo.Context) error {
	current := s.deployer.SchedulerConfig()
	if stored, found, err := s.loadPersistedSchedulerConfig(); err == nil && found {
		current = stored
	}
	response := map[string]any{
		"mode":    current.Mode,
		"targets": current.Targets,
		"defaults": map[string]any{
			"docker": map[string]any{
				"enabled":          s.cfg.Deployer.Backends.Docker.Enabled,
				"host":             s.cfg.Deployer.Backends.Docker.Host,
				"tls_verify":       s.cfg.Deployer.Backends.Docker.TLSVerify,
				"cert_path":        s.cfg.Deployer.Backends.Docker.CertPath,
				"network":          s.cfg.Deployer.Backends.Docker.Network,
				"registry":         s.cfg.Deployer.Backends.Docker.Registry,
				"challenges_dir":   s.cfg.Deployer.Backends.Docker.ChallengesDir,
				"local_tag_prefix": s.cfg.Deployer.Backends.Docker.LocalTagPrefix,
			},
			"kubernetes": map[string]any{
				"enabled":    s.cfg.Deployer.Backends.Kubernetes.Enabled,
				"kubeconfig": s.cfg.Deployer.Backends.Kubernetes.Kubeconfig,
				"namespace":  s.cfg.Deployer.Backends.Kubernetes.Namespace,
			},
			"pve": map[string]any{
				"enabled": s.cfg.Deployer.Backends.PVE.Enabled,
			},
		},
	}
	return c.JSON(http.StatusOK, response)
}

func (s *Server) handleAdminSetDeployConfig(c echo.Context) error {
	var cfg deployer.SchedulerConfig
	if err := c.Bind(&cfg); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	cfg = normalizeSchedulerConfig(cfg)
	for _, target := range cfg.Targets {
		if target.ID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "every deploy target requires an id"})
		}
		switch target.Type {
		case "docker", "kubernetes", "pve":
		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported deploy target type: " + target.Type})
		}
		if err := s.validateDeployTargetConfig(target); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	}
	if err := s.applyDeploySchedulerConfig(cfg); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "marshal error"})
	}
	if _, err := s.db.Exec(
		`INSERT INTO ctf_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		deploySchedulerSettingsKey, string(raw),
	); err != nil {
		if _, err2 := s.db.Exec(`UPDATE ctf_settings SET value=? WHERE key=?`, string(raw), deploySchedulerSettingsKey); err2 != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"message": "deploy scheduler updated",
		"mode":    cfg.Mode,
		"targets": cfg.Targets,
	})
}

func (s *Server) validateDeployTargetConfig(target deployer.TargetConfig) error {
	switch target.Type {
	case "docker":
		host := strings.TrimSpace(fallbackString(target.Host, s.cfg.Deployer.Backends.Docker.Host))
		certPath := strings.TrimSpace(fallbackString(target.CertPath, s.cfg.Deployer.Backends.Docker.CertPath))
		tlsVerify := target.TLSVerify || s.cfg.Deployer.Backends.Docker.TLSVerify
		if host != "" && !strings.HasPrefix(host, "unix://") && !strings.HasPrefix(host, "npipe://") && !strings.HasPrefix(host, "tcp://") {
			return fmt.Errorf("docker target %s: host must use tcp://, unix://, or npipe://", target.ID)
		}
		if strings.HasPrefix(host, "tcp://") && tlsVerify && certPath == "" {
			return fmt.Errorf("docker target %s: cert_path is required when TLS verify is enabled", target.ID)
		}
	case "kubernetes":
		kubeconfig := strings.TrimSpace(fallbackString(target.Kubeconfig, s.cfg.Deployer.Backends.Kubernetes.Kubeconfig))
		if kubeconfig == "" {
			return fmt.Errorf("kubernetes target %s: kubeconfig is required", target.ID)
		}
	case "pve":
		// Proxmox targets are still placeholders and are accepted for config staging.
	}
	return nil
}
