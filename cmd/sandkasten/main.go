//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/p-arndt/sandkasten/internal/api"
	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/reaper"
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
		case "init":
			os.Exit(runInit(os.Args[2:]))
		case "image":
			os.Exit(runImage(os.Args[2:]))
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
	if err := fs.Parse(args); err != nil {
		return 1
	}

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

	if cfg.APIKey == "" {
		logger.Warn("no API key configured — running in open access mode")
	}

	st, err := store.New(cfg.DBPath)
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

	mgr := session.NewManager(cfg, st, rt, nil)

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
	fmt.Fprintf(os.Stderr, "  Dashboard: http://%s\n", cfg.Listen)
	fmt.Fprintf(os.Stderr, "  API:       http://%s/v1\n\n", cfg.Listen)

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		return 1
	}

	return 0
}
