//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/p-arndt/sandkasten/internal/api"
	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/pool"
	"github.com/p-arndt/sandkasten/internal/reaper"
	runtimepkg "github.com/p-arndt/sandkasten/internal/runtime"
	"github.com/p-arndt/sandkasten/internal/runtime/linux"
	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/internal/store"
)

func main() {
	if isNsinit() {
		if err := runNsinit(); err != nil {
			fmt.Fprintf(os.Stderr, "nsinit error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help", "-h", "--help":
			printMainUsage()
			os.Exit(0)
		case "doctor":
			os.Exit(runDoctor(os.Args[2:]))
		case "security":
			os.Exit(runSecurity(os.Args[2:]))
		case "init":
			os.Exit(runInit(os.Args[2:]))
		case "image":
			os.Exit(runImage(os.Args[2:]))
		case "ps":
			os.Exit(runPs(os.Args[2:]))
		case "rm":
			os.Exit(runRm(os.Args[2:]))
		case "stop":
			os.Exit(runStop(os.Args[2:]))
		case "logs":
			os.Exit(runLogs(os.Args[2:]))
		case "daemon":
			os.Exit(runDaemon(os.Args[2:]))
		}
	}

	os.Exit(runDaemon(os.Args[1:]))
}

func runDaemon(args []string) int {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "Error: sandkasten daemon requires Linux (or WSL2)\n")
		return 1
	}

	fs := flag.NewFlagSet("sandkasten", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := fs.String("config", "", "path to sandkasten.yaml")
	logLevelStr := fs.String("log-level", "", "log level: debug, info, warn, error (default from SANDKASTEN_LOG or info)")
	detach := fs.Bool("detach", false, "run daemon in background (like docker)")
	detachShort := fs.Bool("d", false, "short for --detach")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	daemonDetach := *detach || *detachShort

	logLevel := slog.LevelInfo
	if v := *logLevelStr; v != "" {
		// Flag takes precedence (works with sudo when env is stripped)
		switch v {
		case "debug":
			logLevel = slog.LevelDebug
		case "info":
			logLevel = slog.LevelInfo
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		}
	} else if v := os.Getenv("SANDKASTEN_LOG"); v != "" {
		switch v {
		case "debug":
			logLevel = slog.LevelDebug
		case "info":
			logLevel = slog.LevelInfo
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	path := *cfgPath
	if path == "" {
		for _, p := range []string{"sandkasten.yaml", "/etc/sandkasten/sandkasten.yaml"} {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		logger.Error("load config", "error", err)
		return 1
	}
	logger.Debug("config loaded", "config_path", path, "data_dir", cfg.DataDir, "db_path", cfg.DBPath, "listen", cfg.Listen, "network_mode", cfg.Defaults.NetworkMode)

	if daemonDetach {
		if err := daemonize(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "daemonize: %v\n", err)
			return 1
		}
		// After daemonize, stdout/stderr are /dev/null; use them for the logger
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	}

	if err := writePidFileIfDetached(cfg); err != nil {
		logger.Error("write pid file", "error", err)
		return 1
	}

	if cfg.APIKey == "" {
		if isListenNonLoopback(cfg.Listen) {
			logger.Error("refusing to start: API key is empty and listen address is not loopback; set api_key in config or use listen 127.0.0.1 for dev only")
			return 1
		}
		logger.Warn("no API key configured — running in open access mode (dev only; do not use in production)")
	}

	st, err := store.New(cfg.DBPath, cfg.DBMaxOpenConns)
	if err != nil {
		logger.Error("open store", "error", err)
		return 1
	}
	defer st.Close()
	logger.Debug("store opened", "db_path", cfg.DBPath)

	rt, err := linux.NewDriver(cfg, logger)
	if err != nil {
		logger.Error("runtime driver", "error", err)
		return 1
	}
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rt.Ping(ctx); err != nil {
		logger.Error("runtime ping failed", "error", err)
		return 1
	}
	logger.Info("runtime driver OK")
	logger.Debug("reaper and API server starting")

	var pl session.ContainerPool
	if cfg.Pool.Enabled {
		poolCfg := pool.PoolConfig{
			Store:      st,
			Logger:     logger,
			SessionTTL: cfg.SessionTTLSeconds,
			PoolExpiry: 10 * 365 * 24 * time.Hour, // 10 years for pool_idle sessions
			CreateFunc: func(ctx context.Context, sessionID string, image string) (*pool.CreateResult, error) {
				info, err := rt.Create(ctx, runtimepkg.CreateOpts{
					SessionID:   sessionID,
					Image:       image,
					WorkspaceID: "",
				})
				if err != nil {
					return nil, err
				}
				return &pool.CreateResult{InitPID: info.InitPID, CgroupPath: info.CgroupPath}, nil
			},
		}
		if p := pool.New(cfg, poolCfg); p != nil {
			pl = p
			go p.RefillAll(ctx)
		}
	}

	mgr := session.NewManager(cfg, st, rt, nil, pl)

	rpr := reaper.New(st, rt, 30*time.Second, logger)
	rpr.SetSessionManager(mgr)
	go rpr.Run(ctx)

	srv := api.NewServer(cfg, mgr, st, path, logger)

	httpServer := &http.Server{
		Addr:         cfg.Listen,
		Handler:      srv.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		logger.Info("shutting down...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("listening", "addr", cfg.Listen)
	fmt.Fprintf(os.Stderr, "\n  sandkasten daemon ready\n")
	fmt.Fprintf(os.Stderr, "  API:       http://%s/v1\n\n", cfg.Listen)

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		return 1
	}

	return 0
}

// isListenNonLoopback returns true if the listen address binds to a non-loopback interface.
func isListenNonLoopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return true // unknown format; treat as non-loopback to be safe
	}
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}
