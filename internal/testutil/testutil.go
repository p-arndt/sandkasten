package testutil

import (
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/store"
)

// TestConfig returns a Config with sensible test defaults.
func TestConfig() *config.Config {
	return &config.Config{
		Listen:            "127.0.0.1:0",
		APIKey:            "test-api-key",
		DataDir:           "/tmp/sandkasten-test",
		DefaultImage:      "base",
		AllowedImages:     []string{"base", "python", "node"},
		DBPath:            ":memory:",
		SessionTTLSeconds: 300,
		Defaults: config.Defaults{
			CPULimit:         1.0,
			MemLimitMB:       512,
			PidsLimit:        256,
			MaxExecTimeoutMs: 120000,
			NetworkMode:      "none",
			ReadonlyRootfs:   true,
		},
		Pool: config.PoolConfig{
			Enabled: false,
			Images:  make(map[string]int),
		},
		Workspace: config.WorkspaceConfig{
			Enabled:          false,
			PersistByDefault: false,
		},
	}
}

func TestSession(id string) *store.Session {
	now := time.Now().UTC()
	return &store.Session{
		ID:           id,
		Image:        "base",
		InitPID:      12345,
		CgroupPath:   "/sys/fs/cgroup/sandkasten/" + id,
		Status:       "running",
		Cwd:          "/workspace",
		CreatedAt:    now,
		ExpiresAt:    now.Add(5 * time.Minute),
		LastActivity: now,
	}
}

// NewTestStore creates an in-memory SQLite store for testing.
func NewTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}
