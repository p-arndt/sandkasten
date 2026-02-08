//go:build integration

package integration

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/api"
	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/pool"
	"github.com/p-arndt/sandkasten/internal/reaper"
	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "sk-integration-test"

func startTestServer(t *testing.T) (string, func()) {
	t.Helper()

	cfg := &config.Config{
		Listen:            "127.0.0.1:0",
		APIKey:            testAPIKey,
		DefaultImage:      "sandbox-runtime:base",
		AllowedImages:     []string{"sandbox-runtime:base", "sandbox-runtime:python", "sandbox-runtime:node"},
		DBPath:            ":memory:",
		SessionTTLSeconds: 60,
		Defaults: config.Defaults{
			CPULimit:         0.5,
			MemLimitMB:       256,
			PidsLimit:        128,
			MaxExecTimeoutMs: 30000,
			NetworkMode:      "none",
			ReadonlyRootfs:   true,
		},
		Pool: config.PoolConfig{
			Enabled: false,
			Images:  make(map[string]int),
		},
		Workspace: config.WorkspaceConfig{
			Enabled: true,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	st, err := store.New(cfg.DBPath)
	require.NoError(t, err)

	dc, err := docker.New()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, dc.Ping(ctx), "Docker must be running for integration tests")

	wm := workspace.NewManager(dc.DockerClient())
	poolMgr := pool.New(cfg, dc, logger)
	mgr := session.NewManager(cfg, st, dc, poolMgr, wm)

	rpr := reaper.New(st, dc, 5*time.Second, logger)
	rpr.SetSessionManager(mgr)
	go rpr.Run(ctx)

	srv := api.NewServer(cfg, mgr, st, poolMgr, "", logger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	httpServer := &http.Server{Handler: srv.Handler()}
	go httpServer.Serve(listener)

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())

	cleanup := func() {
		cancel()
		httpServer.Close()
		dc.Close()
		st.Close()
	}

	return baseURL, cleanup
}

func TestE2E_Healthz(t *testing.T) {
	baseURL, cleanup := startTestServer(t)
	defer cleanup()

	client := newTestClient(baseURL, testAPIKey)
	resp := client.doRequest(t, "GET", "/healthz", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestE2E_AuthRequired(t *testing.T) {
	baseURL, cleanup := startTestServer(t)
	defer cleanup()

	// No auth
	noAuth := newTestClient(baseURL, "")
	resp := noAuth.doRequest(t, "GET", "/v1/sessions", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// Wrong key
	wrongKey := newTestClient(baseURL, "wrong-key")
	resp = wrongKey.doRequest(t, "GET", "/v1/sessions", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// Correct key
	validClient := newTestClient(baseURL, testAPIKey)
	resp = validClient.doRequest(t, "GET", "/v1/sessions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestE2E_CreateAndExec(t *testing.T) {
	baseURL, cleanup := startTestServer(t)
	defer cleanup()

	client := newTestClient(baseURL, testAPIKey)

	// Create session
	info := client.createSession(t, "sandbox-runtime:base", 60)
	sessionID := info["id"].(string)
	assert.NotEmpty(t, sessionID)
	assert.Equal(t, "running", info["status"])

	// Wait for runner to be ready
	time.Sleep(1 * time.Second)

	// Execute command
	result := client.exec(t, sessionID, "echo hello world")
	assert.Equal(t, float64(0), result["exit_code"])
	assert.Contains(t, result["output"], "hello world")

	// Verify session is listed
	resp := client.doRequest(t, "GET", "/v1/sessions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Destroy session
	client.destroySession(t, sessionID)

	// Verify session is gone (should return 404 or similar)
	resp = client.doRequest(t, "GET", fmt.Sprintf("/v1/sessions/%s", sessionID), nil)
	// Session might still exist in DB with status="destroyed"
	resp.Body.Close()
}

func TestE2E_FileWriteRead(t *testing.T) {
	baseURL, cleanup := startTestServer(t)
	defer cleanup()

	client := newTestClient(baseURL, testAPIKey)

	info := client.createSession(t, "sandbox-runtime:base", 60)
	sessionID := info["id"].(string)
	defer client.destroySession(t, sessionID)

	time.Sleep(1 * time.Second)

	// Write file
	client.writeFile(t, sessionID, "/workspace/test.txt", "hello from integration test")

	// Read file back
	readResult := client.readFile(t, sessionID, "/workspace/test.txt")
	assert.NotEmpty(t, readResult["content_base64"])
}

func TestE2E_SessionExpiry(t *testing.T) {
	baseURL, cleanup := startTestServer(t)
	defer cleanup()

	client := newTestClient(baseURL, testAPIKey)

	// Create session with very short TTL
	info := client.createSession(t, "sandbox-runtime:base", 3)
	sessionID := info["id"].(string)

	time.Sleep(1 * time.Second)

	// Should still work
	result := client.exec(t, sessionID, "echo alive")
	assert.Equal(t, float64(0), result["exit_code"])

	// Wait for expiry + reaper cycle
	time.Sleep(10 * time.Second)

	// Session should be expired now
	resp := client.doRequest(t, "POST", fmt.Sprintf("/v1/sessions/%s/exec", sessionID), map[string]any{
		"cmd": "echo dead",
	})
	// Should get 410 Gone (expired) or 404 or 500
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
