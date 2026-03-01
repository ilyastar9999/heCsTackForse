package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	CTF      CTFConfig      `yaml:"ctf"`
	Deployer DeployerConfig `yaml:"deployer"`
	AD       ADConfig       `yaml:"ad"`
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	SecretKey string `yaml:"secret_key"`
	StaticDir string `yaml:"static_dir"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type CTFConfig struct {
	Name             string `yaml:"name"`
	Mode             string `yaml:"mode"`
	TeamMode         bool   `yaml:"team_mode"`
	RegistrationOpen bool   `yaml:"registration_open"`
	Scoring          string `yaml:"scoring"`
	FlagPrefix       string `yaml:"flag_prefix"`
	FlagSuffix       string `yaml:"flag_suffix"`
}

type DeployerConfig struct {
	Backends    BackendsConfig `yaml:"backends"`
	InstanceTTL string         `yaml:"instance_ttl"`
}

type BackendsConfig struct {
	Docker     DockerConfig     `yaml:"docker"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	PVE        PVEConfig        `yaml:"pve"`
}

type DockerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Registry string `yaml:"registry"`
	Network  string `yaml:"network"`
}

type KubernetesConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Kubeconfig string `yaml:"kubeconfig"`
	Namespace  string `yaml:"namespace"`
}

type PVEConfig struct {
	Enabled     bool   `yaml:"enabled"`
	URL         string `yaml:"url"`
	TokenID     string `yaml:"token_id"`
	TokenSecret string `yaml:"token_secret"`
	Node        string `yaml:"node"`
	VMTemplate  int    `yaml:"vm_template"`
}

type ADConfig struct {
	RoundDuration string   `yaml:"round_duration"`
	FlagLifetime  int      `yaml:"flag_lifetime"`
	Teams         []string `yaml:"teams"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080
	cfg.Server.SecretKey = "change-me-in-production"
	cfg.Server.StaticDir = "./frontend"
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = "./ctf.db"
	cfg.CTF.Name = "My CTF"
	cfg.CTF.Mode = "ctf"
	cfg.CTF.RegistrationOpen = true
	cfg.CTF.Scoring = "static"
	cfg.CTF.FlagPrefix = "FLAG{"
	cfg.CTF.FlagSuffix = "}"
	cfg.Deployer.InstanceTTL = "4h"
	cfg.AD.RoundDuration = "5m"
	cfg.AD.FlagLifetime = 2

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}
