package session

import (
	"testing"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/stretchr/testify/assert"
)

func testConfig() *config.Config {
	return &config.Config{
		DataDir:           "/tmp/sandkasten-test",
		DefaultImage:      "base",
		AllowedImages:     []string{"base", "python"},
		SessionTTLSeconds: 300,
		Defaults: config.Defaults{
			MaxExecTimeoutMs: 120000,
		},
	}
}

func newTestManager() (*Manager, *MockRuntimeDriver, *MockSessionStore) {
	rt := &MockRuntimeDriver{}
	st := &MockSessionStore{}
	cfg := testConfig()
	mgr := NewManager(cfg, st, rt, nil, nil)
	return mgr, rt, st
}

func TestIsImageAllowed(t *testing.T) {
	mgr, _, _ := newTestManager()

	assert.True(t, mgr.isImageAllowed("base"))
	assert.True(t, mgr.isImageAllowed("python"))
	assert.False(t, mgr.isImageAllowed("node"))
	assert.False(t, mgr.isImageAllowed("evil-image"))
}

func TestIsImageAllowedNoRestrictions(t *testing.T) {
	mgr, _, _ := newTestManager()
	mgr.cfg.AllowedImages = nil

	assert.True(t, mgr.isImageAllowed("anything"))
	assert.True(t, mgr.isImageAllowed("base"))
}

func TestResolveImage(t *testing.T) {
	mgr, _, _ := newTestManager()

	assert.Equal(t, "base", mgr.resolveImage(""))
	assert.Equal(t, "python", mgr.resolveImage("python"))
}

func TestResolveTTL(t *testing.T) {
	mgr, _, _ := newTestManager()

	assert.Equal(t, 300, mgr.resolveTTL(0))
	assert.Equal(t, 300, mgr.resolveTTL(-1))
	assert.Equal(t, 600, mgr.resolveTTL(600))
}

func TestEnforceMaxTimeout(t *testing.T) {
	mgr, _, _ := newTestManager()

	assert.Equal(t, 120000, mgr.enforceMaxTimeout(0))
	assert.Equal(t, 120000, mgr.enforceMaxTimeout(-1))
	assert.Equal(t, 120000, mgr.enforceMaxTimeout(200000))
	assert.Equal(t, 5000, mgr.enforceMaxTimeout(5000))
}

func TestSessionLock(t *testing.T) {
	mgr, _, _ := newTestManager()

	mu1 := mgr.sessionLock("sess-1")
	mu2 := mgr.sessionLock("sess-1")
	mu3 := mgr.sessionLock("sess-2")

	assert.Same(t, mu1, mu2)
	assert.NotSame(t, mu1, mu3)
}

func TestCleanupSessionLock(t *testing.T) {
	mgr, _, _ := newTestManager()

	_ = mgr.sessionLock("sess-1")
	assert.Len(t, mgr.locks, 1)

	mgr.CleanupSessionLock("sess-1")
	assert.Len(t, mgr.locks, 0)

	mgr.CleanupSessionLock("nonexistent")
}

func TestResolveCwd(t *testing.T) {
	mgr, _, _ := newTestManager()

	assert.Equal(t, "/home", mgr.resolveCwd("/home", "/workspace"))
	assert.Equal(t, "/workspace", mgr.resolveCwd("", "/workspace"))
}
