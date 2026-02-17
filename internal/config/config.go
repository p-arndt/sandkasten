package config

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Defaults struct {
	CPULimit         float64 `yaml:"cpu_limit"`
	MemLimitMB       int     `yaml:"mem_limit_mb"`
	PidsLimit        int     `yaml:"pids_limit"`
	MaxExecTimeoutMs int     `yaml:"max_exec_timeout_ms"`
	NetworkMode      string  `yaml:"network_mode"`
	ReadonlyRootfs   bool    `yaml:"readonly_rootfs"`
}

type PoolConfig struct {
	Enabled bool               `yaml:"enabled"`
	Images  map[string]int     `yaml:"images"` // image -> pool size
}

type WorkspaceConfig struct {
	Enabled          bool `yaml:"enabled"`
	PersistByDefault bool `yaml:"persist_by_default"`
}

type Config struct {
	Listen                 string          `yaml:"listen"`
	APIKey                 string          `yaml:"api_key"`
	DefaultImage           string          `yaml:"default_image"`
	AllowedImages          []string        `yaml:"allowed_images"`
	DBPath                 string          `yaml:"db_path"`
	SessionTTLSeconds      int             `yaml:"session_ttl_seconds"`
	PlaygroundConfigPath   string          `yaml:"playground_config_path"`
	Defaults               Defaults        `yaml:"defaults"`
	Pool                   PoolConfig      `yaml:"pool"`
	Workspace              WorkspaceConfig `yaml:"workspace"`
}

func Load(yamlPath string) (*Config, error) {
	cfg := &Config{
		Listen:            "127.0.0.1:8080",
		DefaultImage:      "sandbox-runtime:base",
		DBPath:            "./sandkasten.db",
		SessionTTLSeconds: 1800,
		Defaults: Defaults{
			CPULimit:         1.0,
			MemLimitMB:       512,
			PidsLimit:        256,
			MaxExecTimeoutMs: 120000,
			NetworkMode:      "none",
			ReadonlyRootfs:   true,
		},
		Pool: PoolConfig{
			Enabled: false,
			Images:  make(map[string]int),
		},
		Workspace: WorkspaceConfig{
			Enabled:          false,
			PersistByDefault: false,
		},
	}

	if yamlPath != "" {
		data, err := os.ReadFile(yamlPath)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SANDKASTEN_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("SANDKASTEN_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("SANDKASTEN_DEFAULT_IMAGE"); v != "" {
		cfg.DefaultImage = v
	}
	if v := os.Getenv("SANDKASTEN_ALLOWED_IMAGES"); v != "" {
		cfg.AllowedImages = strings.Split(v, ",")
	}
	if v := os.Getenv("SANDKASTEN_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("SANDKASTEN_SESSION_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SessionTTLSeconds = n
		}
	}
	if v := os.Getenv("SANDKASTEN_CPU_LIMIT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Defaults.CPULimit = f
		}
	}
	if v := os.Getenv("SANDKASTEN_MEM_LIMIT_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Defaults.MemLimitMB = n
		}
	}
	if v := os.Getenv("SANDKASTEN_PIDS_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Defaults.PidsLimit = n
		}
	}
	if v := os.Getenv("SANDKASTEN_MAX_EXEC_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Defaults.MaxExecTimeoutMs = n
		}
	}
	if v := os.Getenv("SANDKASTEN_NETWORK_MODE"); v != "" {
		cfg.Defaults.NetworkMode = v
	}
	if v := os.Getenv("SANDKASTEN_READONLY_ROOTFS"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Defaults.ReadonlyRootfs = b
		}
	}
	if v := os.Getenv("SANDKASTEN_PLAYGROUND_CONFIG_PATH"); v != "" {
		cfg.PlaygroundConfigPath = v
	}
}
