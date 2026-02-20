//go:build linux

package linux

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/runtime"
	"github.com/p-arndt/sandkasten/protocol"
)

type Driver struct {
	cfg      *config.Config
	dataDir  string
	imageDir string
	logger   *slog.Logger
}

func NewDriver(cfg *config.Config, logger *slog.Logger) (*Driver, error) {
	if err := DetectCgroupV2(); err != nil {
		return nil, fmt.Errorf("cgroup v2 check failed: %w", err)
	}

	d := &Driver{
		cfg:      cfg,
		dataDir:  cfg.DataDir,
		imageDir: filepath.Join(cfg.DataDir, "images"),
		logger:   logger,
	}

	dirs := []string{
		d.dataDir,
		filepath.Join(d.dataDir, "sessions"),
		filepath.Join(d.dataDir, "workspaces"),
		filepath.Join(d.dataDir, "layers"),
		d.imageDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	return d, nil
}

func (d *Driver) Close() error {
	return nil
}

func (d *Driver) Ping(ctx context.Context) error {
	return DetectCgroupV2()
}

func (d *Driver) Create(ctx context.Context, opts runtime.CreateOpts) (*runtime.SessionInfo, error) {
	if d.logger != nil {
		d.logger.Debug("runtime create session", "session_id", opts.SessionID, "image", opts.Image, "workspace_id", opts.WorkspaceID)
	}
	runnerUID := 1000
	runnerGID := 1000

	var lower string
	metaPath := filepath.Join(d.imageDir, opts.Image, "meta.json")
	if metaData, err := os.ReadFile(metaPath); err == nil {
		var meta struct {
			Layers []string `json:"layers"`
		}
		if err := json.Unmarshal(metaData, &meta); err == nil && len(meta.Layers) > 0 {
			var lowerDirs []string
			lowerDirs = append(lowerDirs, filepath.Join(d.dataDir, "layers", "runner", "rootfs"))
			for i := len(meta.Layers) - 1; i >= 0; i-- {
				lowerDirs = append(lowerDirs, filepath.Join(d.dataDir, "layers", meta.Layers[i], "rootfs"))
			}
			lower = strings.Join(lowerDirs, ":")
		}
	}

	if lower == "" {
		lower = filepath.Join(d.imageDir, opts.Image, "rootfs")
		if _, err := os.Stat(lower); os.IsNotExist(err) {
			return nil, fmt.Errorf("image %s not found at %s", opts.Image, lower)
		}
	}

	sessionDir := filepath.Join(d.dataDir, "sessions", opts.SessionID)
	upper := filepath.Join(sessionDir, "upper")
	work := filepath.Join(sessionDir, "work")
	mnt := filepath.Join(sessionDir, "mnt")
	runHostDir := filepath.Join(sessionDir, protocol.RunDirName)

	var workspaceSrc string
	if opts.WorkspaceID != "" {
		workspaceSrc = filepath.Join(d.dataDir, "workspaces", opts.WorkspaceID)
		if err := os.MkdirAll(workspaceSrc, 0755); err != nil {
			return nil, fmt.Errorf("mkdir workspace %s: %w", workspaceSrc, err)
		}
		if err := os.Chown(workspaceSrc, runnerUID, runnerGID); err != nil {
			return nil, fmt.Errorf("chown workspace %s: %w", workspaceSrc, err)
		}
	}

	if err := SetupFilesystem(lower, upper, work, mnt, workspaceSrc, runHostDir); err != nil {
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("setup filesystem: %w", err)
	}
	if d.cfg.Defaults.NetworkMode != "none" {
		if err := EnsureResolvConf(mnt); err != nil {
			CleanupMounts(mnt)
			d.cleanupSessionDir(sessionDir)
			return nil, fmt.Errorf("ensure resolv.conf: %w", err)
		}
	}

	if err := os.Chown(runHostDir, runnerUID, runnerGID); err != nil {
		CleanupMounts(mnt)
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("chown run dir %s: %w", runHostDir, err)
	}

	cgConfig := CgroupConfig{
		CPULimit:   d.cfg.Defaults.CPULimit,
		MemLimitMB: d.cfg.Defaults.MemLimitMB,
		PidsLimit:  d.cfg.Defaults.PidsLimit,
	}
	cgPath, err := CreateCgroup(opts.SessionID, cgConfig)
	if err != nil {
		CleanupMounts(mnt)
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("create cgroup: %w", err)
	}

	nsConfig := NsinitConfig{
		SessionID:   opts.SessionID,
		Mnt:         mnt,
		CgroupPath:  cgPath,
		RunnerPath:  "/usr/local/bin/runner",
		UID:         runnerUID,
		GID:         runnerGID,
		NoNewPrivs:  true,
		NetworkNone: d.cfg.Defaults.NetworkMode == "none",
	}

	cmd, nsinitLog, err := LaunchNsinit(nsConfig)
	if err != nil {
		_ = RemoveCgroup(opts.SessionID)
		CleanupMounts(mnt)
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("launch nsinit: %w", err)
	}

	if err := cmd.Start(); err != nil {
		logContent, _ := os.ReadFile(nsinitLog.Name())
		_ = nsinitLog.Close()
		_ = os.Remove(nsinitLog.Name())
		_ = RemoveCgroup(opts.SessionID)
		CleanupMounts(mnt)
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("start nsinit: %w (log: %s)", err, string(logContent))
	}

	initPid := cmd.Process.Pid
	if err := AttachToCgroup(cgPath, initPid); err != nil {
		logContent, _ := os.ReadFile(nsinitLog.Name())
		_ = nsinitLog.Close()
		_ = os.Remove(nsinitLog.Name())
		_ = KillProcessForce(initPid)
		_ = RemoveCgroup(opts.SessionID)
		CleanupMounts(mnt)
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("attach to cgroup: %w (log: %s)", err, string(logContent))
	}

	runnerSock := filepath.Join(runHostDir, protocol.RunnerSocketName)
	if err := d.waitForSocket(ctx, runnerSock, 10*time.Second); err != nil {
		logContent, _ := os.ReadFile(nsinitLog.Name())
		_ = nsinitLog.Close()
		_ = os.Remove(nsinitLog.Name())
		_ = KillProcessForce(initPid)
		_ = RemoveCgroup(opts.SessionID)
		CleanupMounts(mnt)
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("wait for runner socket: %w (nsinit log: %s)", err, string(logContent))
	}

	_ = nsinitLog.Close()
	_ = os.Remove(nsinitLog.Name())

	state := protocol.SessionState{
		SessionID:  opts.SessionID,
		InitPID:    initPid,
		CgroupPath: cgPath,
		Mnt:        mnt,
		RunnerSock: runnerSock,
	}
	statePath := filepath.Join(sessionDir, "state.json")
	if err := d.writeState(statePath, state); err != nil {
		_ = KillProcessForce(initPid)
		_ = RemoveCgroup(opts.SessionID)
		CleanupMounts(mnt)
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("write state: %w", err)
	}

	if d.logger != nil {
		d.logger.Debug("runtime session created", "session_id", opts.SessionID, "init_pid", initPid)
	}
	return &runtime.SessionInfo{
		SessionID:  opts.SessionID,
		InitPID:    initPid,
		CgroupPath: cgPath,
		Mnt:        mnt,
		RunnerSock: runnerSock,
	}, nil
}

func (d *Driver) Exec(ctx context.Context, sessionID string, req protocol.Request) (*protocol.Response, error) {
	if d.logger != nil {
		d.logger.Debug("runtime exec", "session_id", sessionID, "request_id", req.ID)
	}
	runnerSock := filepath.Join(d.dataDir, "sessions", sessionID, protocol.RunDirName, protocol.RunnerSocketName)
	return d.execViaSocket(runnerSock, req)
}

func (d *Driver) execViaSocket(sockPath string, req protocol.Request) (*protocol.Response, error) {
	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to runner: %w", err)
	}
	defer conn.Close()

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := fmt.Fprintf(conn, "%s\n", reqJSON); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, protocol.MaxOutputBytes+4096), protocol.MaxOutputBytes+4096)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		return nil, fmt.Errorf("no response from runner")
	}

	var resp protocol.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

func (d *Driver) Destroy(ctx context.Context, sessionID string) error {
	if d.logger != nil {
		d.logger.Debug("runtime destroy session", "session_id", sessionID)
	}
	sessionDir := filepath.Join(d.dataDir, "sessions", sessionID)
	statePath := filepath.Join(sessionDir, "state.json")

	state, err := d.readState(statePath)
	if err != nil {
		_ = os.RemoveAll(sessionDir)
		return nil
	}

	if state.InitPID > 0 {
		_ = KillProcess(state.InitPID)
		time.Sleep(500 * time.Millisecond)

		if running, _ := d.isProcessRunning(state.InitPID); running {
			_ = KillProcessForce(state.InitPID)
		}
	}

	if state.CgroupPath != "" {
		_ = KillCgroupProcesses(state.CgroupPath)
		_ = RemoveCgroup(sessionID)
	}

	if state.Mnt != "" {
		CleanupMounts(state.Mnt)
	}

	_ = os.RemoveAll(sessionDir)

	return nil
}

func (d *Driver) IsRunning(ctx context.Context, sessionID string) (bool, error) {
	statePath := filepath.Join(d.dataDir, "sessions", sessionID, "state.json")
	state, err := d.readState(statePath)
	if err != nil {
		return false, nil
	}
	return d.isProcessRunning(state.InitPID)
}

func (d *Driver) Stats(ctx context.Context, sessionID string) (*protocol.SessionStats, error) {
	statePath := filepath.Join(d.dataDir, "sessions", sessionID, "state.json")
	state, err := d.readState(statePath)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	if state.CgroupPath == "" {
		return nil, fmt.Errorf("no cgroup path for session")
	}

	stats := &protocol.SessionStats{}

	// Read memory current
	if data, err := os.ReadFile(filepath.Join(state.CgroupPath, "memory.current")); err == nil {
		fmt.Sscanf(string(data), "%d", &stats.MemoryBytes)
	}

	// Read memory limit
	if data, err := os.ReadFile(filepath.Join(state.CgroupPath, "memory.max")); err == nil {
		val := strings.TrimSpace(string(data))
		if val != "max" && val != "" {
			fmt.Sscanf(val, "%d", &stats.MemoryLimit)
		}
	}

	// Read cpu usage
	if data, err := os.ReadFile(filepath.Join(state.CgroupPath, "cpu.stat")); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "usage_usec ") {
				fmt.Sscanf(line, "usage_usec %d", &stats.CPUUsageUsec)
				break
			}
		}
	}

	return stats, nil
}

func (d *Driver) isProcessRunning(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err == os.ErrProcessDone || exec.Command("kill", "-0", fmt.Sprintf("%d", pid)).Run() != nil {
		return false, nil
	}
	return true, nil
}

func (d *Driver) waitForSocket(ctx context.Context, sockPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, err := os.Stat(sockPath); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for socket %s", sockPath)
}

func (d *Driver) writeState(path string, state protocol.SessionState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (d *Driver) readState(path string) (*protocol.SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state protocol.SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (d *Driver) cleanupSessionDir(dir string) {
	_ = os.RemoveAll(dir)
}
