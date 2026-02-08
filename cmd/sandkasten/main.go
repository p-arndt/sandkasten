package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/p-arndt/sandkasten/internal/api"
	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/pool"
	"github.com/p-arndt/sandkasten/internal/reaper"
	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/internal/workspace"
)

func main() {
	cfgPath := flag.String("config", "", "path to sandkasten.yaml")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	if cfg.APIKey == "" {
		logger.Warn("no API key configured — running in open access mode")
	}

	st, err := store.New(cfg.DBPath)
	if err != nil {
		logger.Error("open store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	dc, err := docker.New()
	if err != nil {
		logger.Error("docker client", "error", err)
		os.Exit(1)
	}
	defer dc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Ping(ctx); err != nil {
		logger.Error("docker ping failed — is Docker running?", "error", err)
		os.Exit(1)
	}
	logger.Info("docker connection OK")

	// Initialize workspace manager
	wm := workspace.NewManager(dc.DockerClient())

	// Initialize pool manager
	poolMgr := pool.New(cfg, dc, st, logger)

	// Start pool if enabled
	if cfg.Pool.Enabled && len(cfg.Pool.Images) > 0 {
		poolConfigs := make([]pool.PoolConfig, 0, len(cfg.Pool.Images))
		for image, size := range cfg.Pool.Images {
			poolConfigs = append(poolConfigs, pool.PoolConfig{
				Image: image,
				Size:  size,
			})
		}
		poolMgr.Start(ctx, poolConfigs)
		logger.Info("container pool started", "images", cfg.Pool.Images)
	}

	mgr := session.NewManager(cfg, st, dc, poolMgr, wm)

	rpr := reaper.New(st, dc, 30*time.Second, logger)
	rpr.SetSessionManager(mgr)
	go rpr.Run(ctx)

	srv := api.NewServer(cfg, mgr, st, poolMgr, *cfgPath, logger)

	httpServer := &http.Server{
		Addr:         cfg.Listen,
		Handler:      srv.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // exec can be long
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		logger.Info("shutting down...")
		cancel()

		// Stop pool
		if cfg.Pool.Enabled {
			poolMgr.Stop(ctx)
		}

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("listening", "addr", cfg.Listen)
	fmt.Fprintf(os.Stderr, "\n  🏖️  sandkasten daemon ready\n")
	fmt.Fprintf(os.Stderr, "  📊 Dashboard: http://%s\n", cfg.Listen)
	fmt.Fprintf(os.Stderr, "  🔧 API:       http://%s/v1\n\n", cfg.Listen)

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
