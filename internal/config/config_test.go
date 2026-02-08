package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:8080", cfg.Listen)
	assert.Equal(t, "sandbox-runtime:base", cfg.DefaultImage)
	assert.Equal(t, "./sandkasten.db", cfg.DBPath)
	assert.Equal(t, 1800, cfg.SessionTTLSeconds)
	assert.Equal(t, 1.0, cfg.Defaults.CPULimit)
	assert.Equal(t, 512, cfg.Defaults.MemLimitMB)
	assert.Equal(t, 256, cfg.Defaults.PidsLimit)
	assert.Equal(t, 120000, cfg.Defaults.MaxExecTimeoutMs)
	assert.Equal(t, "none", cfg.Defaults.NetworkMode)
	assert.True(t, cfg.Defaults.ReadonlyRootfs)
	assert.False(t, cfg.Pool.Enabled)
	assert.False(t, cfg.Workspace.Enabled)
}

func TestLoadYAML(t *testing.T) {
	yamlContent := `
listen: "0.0.0.0:9090"
api_key: "sk-test"
default_image: "sandbox-runtime:python"
session_ttl_seconds: 3600
defaults:
  cpu_limit: 2.0
  mem_limit_mb: 1024
workspace:
  enabled: true
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlContent), 0644))

	cfg, err := Load(yamlPath)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0:9090", cfg.Listen)
	assert.Equal(t, "sk-test", cfg.APIKey)
	assert.Equal(t, "sandbox-runtime:python", cfg.DefaultImage)
	assert.Equal(t, 3600, cfg.SessionTTLSeconds)
	assert.Equal(t, 2.0, cfg.Defaults.CPULimit)
	assert.Equal(t, 1024, cfg.Defaults.MemLimitMB)
	assert.True(t, cfg.Workspace.Enabled)
}

func TestLoadYAMLMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	// Non-existent file is not an error (silently uses defaults)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8080", cfg.Listen)
}

func TestLoadYAMLInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "bad.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("{{{{invalid yaml"), 0644))

	_, err := Load(yamlPath)
	assert.Error(t, err)
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("SANDKASTEN_LISTEN", "0.0.0.0:7777")
	t.Setenv("SANDKASTEN_API_KEY", "env-key")
	t.Setenv("SANDKASTEN_DEFAULT_IMAGE", "sandbox-runtime:node")
	t.Setenv("SANDKASTEN_ALLOWED_IMAGES", "img1,img2,img3")
	t.Setenv("SANDKASTEN_DB_PATH", "/tmp/test.db")
	t.Setenv("SANDKASTEN_SESSION_TTL_SECONDS", "600")
	t.Setenv("SANDKASTEN_CPU_LIMIT", "0.5")
	t.Setenv("SANDKASTEN_MEM_LIMIT_MB", "256")
	t.Setenv("SANDKASTEN_PIDS_LIMIT", "128")
	t.Setenv("SANDKASTEN_MAX_EXEC_TIMEOUT_MS", "30000")
	t.Setenv("SANDKASTEN_NETWORK_MODE", "bridge")
	t.Setenv("SANDKASTEN_READONLY_ROOTFS", "false")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0:7777", cfg.Listen)
	assert.Equal(t, "env-key", cfg.APIKey)
	assert.Equal(t, "sandbox-runtime:node", cfg.DefaultImage)
	assert.Equal(t, []string{"img1", "img2", "img3"}, cfg.AllowedImages)
	assert.Equal(t, "/tmp/test.db", cfg.DBPath)
	assert.Equal(t, 600, cfg.SessionTTLSeconds)
	assert.Equal(t, 0.5, cfg.Defaults.CPULimit)
	assert.Equal(t, 256, cfg.Defaults.MemLimitMB)
	assert.Equal(t, 128, cfg.Defaults.PidsLimit)
	assert.Equal(t, 30000, cfg.Defaults.MaxExecTimeoutMs)
	assert.Equal(t, "bridge", cfg.Defaults.NetworkMode)
	assert.False(t, cfg.Defaults.ReadonlyRootfs)
}

func TestEnvOverridesYAML(t *testing.T) {
	yamlContent := `
listen: "127.0.0.1:8080"
api_key: "yaml-key"
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlContent), 0644))

	t.Setenv("SANDKASTEN_API_KEY", "env-key")

	cfg, err := Load(yamlPath)
	require.NoError(t, err)

	// Env should override YAML
	assert.Equal(t, "env-key", cfg.APIKey)
	// YAML value should be preserved for non-overridden fields
	assert.Equal(t, "127.0.0.1:8080", cfg.Listen)
}

func TestEnvOverrideInvalidValues(t *testing.T) {
	t.Setenv("SANDKASTEN_SESSION_TTL_SECONDS", "not-a-number")
	t.Setenv("SANDKASTEN_CPU_LIMIT", "not-a-float")

	cfg, err := Load("")
	require.NoError(t, err)

	// Invalid values should be silently ignored, keeping defaults
	assert.Equal(t, 1800, cfg.SessionTTLSeconds)
	assert.Equal(t, 1.0, cfg.Defaults.CPULimit)
}
