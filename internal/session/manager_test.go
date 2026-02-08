package session

import (
	"testing"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/stretchr/testify/assert"
)

func testConfig() *config.Config {
	return &config.Config{
		DefaultImage:      "sandbox-runtime:base",
		AllowedImages:     []string{"sandbox-runtime:base", "sandbox-runtime:python"},
		SessionTTLSeconds: 300,
		Defaults: config.Defaults{
			MaxExecTimeoutMs: 120000,
		},
	}
}

func newTestManager() (*Manager, *MockDockerClient, *MockSessionStore, *MockContainerPool, *MockWorkspaceManager) {
	dc := &MockDockerClient{}
	st := &MockSessionStore{}
	pool := &MockContainerPool{}
	ws := &MockWorkspaceManager{}
	cfg := testConfig()
	mgr := NewManager(cfg, st, dc, pool, ws)
	return mgr, dc, st, pool, ws
}

func TestIsImageAllowed(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	assert.True(t, mgr.isImageAllowed("sandbox-runtime:base"))
	assert.True(t, mgr.isImageAllowed("sandbox-runtime:python"))
	assert.False(t, mgr.isImageAllowed("sandbox-runtime:node"))
	assert.False(t, mgr.isImageAllowed("evil-image"))
}

func TestIsImageAllowedNoRestrictions(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()
	mgr.cfg.AllowedImages = nil

	assert.True(t, mgr.isImageAllowed("anything"))
	assert.True(t, mgr.isImageAllowed("sandbox-runtime:base"))
}

func TestResolveImage(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	assert.Equal(t, "sandbox-runtime:base", mgr.resolveImage(""))
	assert.Equal(t, "sandbox-runtime:python", mgr.resolveImage("sandbox-runtime:python"))
}

func TestResolveTTL(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	assert.Equal(t, 300, mgr.resolveTTL(0))
	assert.Equal(t, 300, mgr.resolveTTL(-1))
	assert.Equal(t, 600, mgr.resolveTTL(600))
}

func TestEnforceMaxTimeout(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	assert.Equal(t, 120000, mgr.enforceMaxTimeout(0))
	assert.Equal(t, 120000, mgr.enforceMaxTimeout(-1))
	assert.Equal(t, 120000, mgr.enforceMaxTimeout(200000))
	assert.Equal(t, 5000, mgr.enforceMaxTimeout(5000))
}

func TestSessionLock(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	mu1 := mgr.sessionLock("sess-1")
	mu2 := mgr.sessionLock("sess-1")
	mu3 := mgr.sessionLock("sess-2")

	// Same session returns same mutex
	assert.Same(t, mu1, mu2)
	// Different session returns different mutex
	assert.NotSame(t, mu1, mu3)
}

func TestCleanupSessionLock(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	_ = mgr.sessionLock("sess-1")
	assert.Len(t, mgr.locks, 1)

	mgr.CleanupSessionLock("sess-1")
	assert.Len(t, mgr.locks, 0)

	// Cleaning up non-existent lock should not panic
	mgr.CleanupSessionLock("nonexistent")
}

func TestResolveCwd(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	assert.Equal(t, "/home", mgr.resolveCwd("/home", "/workspace"))
	assert.Equal(t, "/workspace", mgr.resolveCwd("", "/workspace"))
}
