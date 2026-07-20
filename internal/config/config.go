package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig       `yaml:"server"`
	Database DatabaseConfig     `yaml:"database"`
	CTF      CTFConfig          `yaml:"ctf"`
	Deployer DeployerConfig     `yaml:"deployer"`
	AD       ADConfig           `yaml:"ad"`
	Plugins  PluginBridgeConfig `yaml:"plugins"`
	Cache    CacheConfig        `yaml:"cache"`
}

type PluginBridgeConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Python     string   `yaml:"python"`
	Script     string   `yaml:"script"`
	PluginDirs []string `yaml:"plugin_dirs"`
	Modules    []string `yaml:"modules"`
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
	SeedChallenges   bool   `yaml:"seed_challenges"`
	Language         string `yaml:"language"` // default UI language: "en", "ru", …
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
	Enabled        bool   `yaml:"enabled"`
	Registry       string `yaml:"registry"`
	Host           string `yaml:"host"`
	TLSVerify      bool   `yaml:"tls_verify"`
	CertPath       string `yaml:"cert_path"`
	Network        string `yaml:"network"`
	ChallengesDir  string `yaml:"challenges_dir"`
	LocalTagPrefix string `yaml:"local_tag_prefix"`
}

type KubernetesConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Kubeconfig string `yaml:"kubeconfig"`
	Namespace  string `yaml:"namespace"`
}

type PVEConfig struct {
	Enabled            bool   `yaml:"enabled"`
	URL                string `yaml:"url"`
	TokenID            string `yaml:"token_id"`
	TokenSecret        string `yaml:"token_secret"`
	Node               string `yaml:"node"`
	VMTemplate         int    `yaml:"vm_template"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // set true only if PVE uses a self-signed cert
}

type ADConfig struct {
	RoundDuration  string    `yaml:"round_duration"`
	FlagLifetime   int       `yaml:"flag_lifetime"`
	Teams          []string  `yaml:"teams"`
	FlagSubmitURL  string    `yaml:"flag_submit_url"` // e.g. http://10.10.10.10/flags
	FlagSubmitKey  string    `yaml:"flag_submit_key"` // auth key/token for the central flag submitter
	CheckerTimeout string    `yaml:"checker_timeout"` // e.g. 30s
	SploitTimeout  string    `yaml:"sploit_timeout"`  // e.g. 60s
	SploitDir      string    `yaml:"sploit_dir"`      // writable dir for temp sploit files
	VPN            VPNConfig `yaml:"vpn"`
}

// VPNConfig holds the WireGuard server-side parameters needed to generate
// per-team client configuration files.
type VPNConfig struct {
	Enabled         bool     `yaml:"enabled"`
	ServerPublicKey string   `yaml:"server_public_key"` // WireGuard public key of the server peer
	ServerEndpoint  string   `yaml:"server_endpoint"`   // host:port, e.g. vpn.example.com:51820
	ServerIP        string   `yaml:"server_ip"`         // server's WireGuard IP, e.g. 10.8.0.1
	TeamSubnetBase  string   `yaml:"team_subnet_base"`  // e.g. "10.8." - team N gets 10.8.N.0/24
	GameNetCIDR     string   `yaml:"game_net_cidr"`     // allowed-IPs route for the game network, e.g. 10.10.0.0/16
	DNS             string   `yaml:"dns"`               // optional DNS pushed to clients
	HookCommand     string   `yaml:"hook_command"`      // optional peer sync command, receives env vars describing the peer
	HookArgs        []string `yaml:"hook_args"`         // optional args for hook_command
	HookTimeout     string   `yaml:"hook_timeout"`      // e.g. 15s
}

type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`      // e.g. "valkey://localhost:6379"
	Prefix  string `yaml:"prefix"`   // key prefix, default "hectackforse:"
	TTLScoreboard string `yaml:"ttl_scoreboard"` // e.g. "30s"
	TTLStatistics string `yaml:"ttl_statistics"`  // e.g. "60s"
	TTLConfig     string `yaml:"ttl_config"`      // e.g. "60s"
	TTLAD         string `yaml:"ttl_ad"`          // e.g. "5m"
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
	cfg.CTF.SeedChallenges = true
	cfg.CTF.Language = "en"
	cfg.Deployer.InstanceTTL = "4h"
	cfg.Deployer.Backends.Docker.ChallengesDir = "./challenges"
	cfg.Deployer.Backends.Docker.LocalTagPrefix = "hecstack/"
	cfg.AD.RoundDuration = "5m"
	cfg.AD.FlagLifetime = 2
	cfg.AD.CheckerTimeout = "30s"
	cfg.AD.SploitTimeout = "60s"
	cfg.AD.SploitDir = "/tmp/sploits"
	cfg.AD.VPN.HookTimeout = "15s"
	cfg.Cache.URL = ""
	cfg.Cache.Prefix = "hectackforse:"
	cfg.Cache.TTLScoreboard = "30s"
	cfg.Cache.TTLStatistics = "60s"
	cfg.Cache.TTLConfig = "60s"
	cfg.Cache.TTLAD = "5m"

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	expanded := os.ExpandEnv(string(data))
	return cfg, yaml.Unmarshal([]byte(expanded), cfg)
}
