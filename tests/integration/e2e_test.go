//go:build integration && linux

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
	"github.com/p-arndt/sandkasten/internal/reaper"
	linuxrt "github.com/p-arndt/sandkasten/internal/runtime/linux"
	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "sk-integration-test"

func startTestServer(t *testing.T) (string, func()) {
	t.Helper()

	cfg := &config.Config{
		Listen:            "127.0.0.1:0",
		APIKey:            testAPIKey,
		DataDir:           "/tmp/sandkasten-test",
		DefaultImage:      "base",
		AllowedImages:     []string{"base", "python", "node"},
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
		Workspace: config.WorkspaceConfig{
			Enabled: true,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	st, err := store.New(cfg.DBPath, 0)
	require.NoError(t, err)

	rt, err := linuxrt.NewDriver(cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	mgr := session.NewManager(cfg, st, rt, nil, nil)

	rpr := reaper.New(st, rt, 5*time.Second, logger)
	rpr.SetSessionManager(mgr)
	go rpr.Run(ctx)

	srv := api.NewServer(cfg, mgr, st, "", logger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	httpServer := &http.Server{Handler: srv.Handler()}
	go httpServer.Serve(listener)

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())

	cleanup := func() {
		cancel()
		httpServer.Close()
		rt.Close()
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

	noAuth := newTestClient(baseURL, "")
	resp := noAuth.doRequest(t, "GET", "/v1/sessions", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	wrongKey := newTestClient(baseURL, "wrong-key")
	resp = wrongKey.doRequest(t, "GET", "/v1/sessions", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	validClient := newTestClient(baseURL, testAPIKey)
	resp = validClient.doRequest(t, "GET", "/v1/sessions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
