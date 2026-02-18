//go:build linux

package linux

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	EnvNsinit = "SANDKASTEN_NSINIT"
	EnvConfig = "SANDKASTEN_NSINIT_CONFIG"
)

type NsinitConfig struct {
	SessionID   string `json:"session_id"`
	Mnt         string `json:"mnt"`
	CgroupPath  string `json:"cgroup_path"`
	RunnerPath  string `json:"runner_path"`
	UID         int    `json:"uid"`
	GID         int    `json:"gid"`
	NoNewPrivs  bool   `json:"no_new_privs"`
	NetworkNone bool   `json:"network_none"`
}

func IsNsinit() bool {
	return os.Getenv(EnvNsinit) == "1"
}

func RunNsinit() error {
	cfgJSON := os.Getenv(EnvConfig)
	if cfgJSON == "" {
		return fmt.Errorf("missing %s", EnvConfig)
	}

	var cfg NsinitConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return fmt.Errorf("parse nsinit config: %w", err)
	}

	return nsinitMain(cfg)
}

func nsinitMain(cfg NsinitConfig) error {
	if err := unix.Sethostname([]byte("sk-" + cfg.SessionID[:8])); err != nil {
		return fmt.Errorf("sethostname: %w", err)
	}

	if err := MakePrivate("/"); err != nil {
		return fmt.Errorf("make private: %w", err)
	}

	oldRoot := filepath.Join(cfg.Mnt, ".oldroot")
	if err := os.MkdirAll(oldRoot, 0700); err != nil {
		return fmt.Errorf("mkdir .oldroot: %w", err)
	}

	if err := PivotRoot(cfg.Mnt, oldRoot); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}

	_ = UmountDetach("/.oldroot")
	_ = os.RemoveAll("/.oldroot")

	if err := MountProc("/proc"); err != nil {
		return fmt.Errorf("mount proc: %w", err)
	}

	devPts := "/dev/pts"
	if err := os.MkdirAll(devPts, 0755); err != nil {
		return fmt.Errorf("mkdir /dev/pts: %w", err)
	}
	if err := unix.Mount("devpts", devPts, "devpts", 0, "newinstance,ptmxmode=0666,mode=620,gid=5"); err != nil {
		return fmt.Errorf("mount devpts: %w", err)
	}

	if err := os.MkdirAll("/run/sandkasten", 0755); err != nil {
		return fmt.Errorf("mkdir /run/sandkasten: %w", err)
	}
	if err := unix.Chmod("/run", 0755); err != nil {
		return fmt.Errorf("chmod /run: %w", err)
	}
	if cfg.UID > 0 || cfg.GID > 0 {
		if err := unix.Chown("/run/sandkasten", cfg.UID, cfg.GID); err != nil {
			return fmt.Errorf("chown /run/sandkasten: %w", err)
		}
	}

	if cfg.NoNewPrivs {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("prctl NO_NEW_PRIVS: %w", err)
		}
	}

	if err := dropCapabilities(); err != nil {
		return fmt.Errorf("drop capabilities: %w", err)
	}

	if cfg.GID > 0 {
		if err := unix.Setgroups([]int{cfg.GID}); err != nil {
			return fmt.Errorf("setgroups: %w", err)
		}
		if err := unix.Setgid(cfg.GID); err != nil {
			return fmt.Errorf("setgid: %w", err)
		}
	}

	if cfg.UID > 0 {
		if err := unix.Setuid(cfg.UID); err != nil {
			return fmt.Errorf("setuid: %w", err)
		}
	}

	argv := []string{cfg.RunnerPath}
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/home/sandbox",
		"TERM=xterm",
		"LANG=C.UTF-8",
	}

	return unix.Exec(cfg.RunnerPath, argv, env)
}

func dropCapabilities() error {
	caps := []uintptr{
		unix.CAP_NET_RAW,
		unix.CAP_NET_BIND_SERVICE,
		unix.CAP_SYS_ADMIN,
		unix.CAP_SYS_PTRACE,
		unix.CAP_SYS_MODULE,
		unix.CAP_SYS_RAWIO,
		unix.CAP_SYS_TIME,
		unix.CAP_SYSLOG,
		unix.CAP_SYS_CHROOT,
		unix.CAP_SYS_BOOT,
		unix.CAP_KILL,
		unix.CAP_DAC_OVERRIDE,
		unix.CAP_DAC_READ_SEARCH,
		unix.CAP_FOWNER,
		unix.CAP_FSETID,
		unix.CAP_SETGID,
		unix.CAP_SETUID,
		unix.CAP_SETPCAP,
		unix.CAP_LINUX_IMMUTABLE,
		unix.CAP_NET_BROADCAST,
		unix.CAP_IPC_LOCK,
		unix.CAP_IPC_OWNER,
		unix.CAP_SYS_PTRACE,
		unix.CAP_SYS_PACCT,
		unix.CAP_MKNOD,
	}

	for _, cap := range caps {
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, cap, 0, 0, 0); err != nil {
		}
	}
	return nil
}

func LaunchNsinit(cfg NsinitConfig) (*exec.Cmd, *os.File, error) {
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal nsinit config: %w", err)
	}

	selfPath, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("get executable path: %w", err)
	}

	logFile, err := os.CreateTemp("", "sandkasten-nsinit-*.log")
	if err != nil {
		return nil, nil, fmt.Errorf("create log file: %w", err)
	}

	cmd := exec.Command(selfPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("%s=1", EnvNsinit),
		fmt.Sprintf("%s=%s", EnvConfig, string(cfgJSON)),
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC,
	}

	if cfg.NetworkNone {
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	cmd.SysProcAttr.Setsid = true

	return cmd, logFile, nil
}

func WaitProcess(cmd *exec.Cmd) error {
	err := cmd.Wait()
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				sig := status.Signal()
				if sig == syscall.SIGTERM || sig == syscall.SIGKILL {
					return nil
				}
			}
		}
	}
	return err
}

func KillProcess(pid int) error {
	if err := unix.Kill(pid, unix.SIGTERM); err != nil {
		if err == unix.ESRCH {
			return nil
		}
		return err
	}
	return nil
}

func KillProcessForce(pid int) error {
	if err := unix.Kill(pid, unix.SIGKILL); err != nil {
		if err == unix.ESRCH {
			return nil
		}
		return err
	}
	return nil
}

func SetupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		os.Exit(0)
	}()
}
