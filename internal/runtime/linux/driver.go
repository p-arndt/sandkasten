//go:build linux

package linux

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
}

func NewDriver(cfg *config.Config) (*Driver, error) {
	if err := DetectCgroupV2(); err != nil {
		return nil, fmt.Errorf("cgroup v2 check failed: %w", err)
	}

	d := &Driver{
		cfg:      cfg,
		dataDir:  cfg.DataDir,
		imageDir: filepath.Join(cfg.DataDir, "images"),
	}

	dirs := []string{
		d.dataDir,
		filepath.Join(d.dataDir, "sessions"),
		filepath.Join(d.dataDir, "workspaces"),
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
	lower := filepath.Join(d.imageDir, opts.Image, "rootfs")
	if _, err := os.Stat(lower); os.IsNotExist(err) {
		return nil, fmt.Errorf("image %s not found at %s", opts.Image, lower)
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
	}

	if err := SetupFilesystem(lower, upper, work, mnt, workspaceSrc, runHostDir); err != nil {
		d.cleanupSessionDir(sessionDir)
		return nil, fmt.Errorf("setup filesystem: %w", err)
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
		UID:         1000,
		GID:         1000,
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

	return &runtime.SessionInfo{
		SessionID:  opts.SessionID,
		InitPID:    initPid,
		CgroupPath: cgPath,
		Mnt:        mnt,
		RunnerSock: runnerSock,
	}, nil
}

func (d *Driver) Exec(ctx context.Context, sessionID string, req protocol.Request) (*protocol.Response, error) {
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
