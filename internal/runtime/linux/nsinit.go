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
	"unsafe"

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
	Readonly    bool   `json:"readonly"`
	Seccomp     string `json:"seccomp"`
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

	// pivot_root requires the new root to be a mount point
	if err := unix.Mount(cfg.Mnt, cfg.Mnt, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount new root: %w", err)
	}
	// Needs to be private for pivot_root
	unix.Mount("", cfg.Mnt, "", unix.MS_PRIVATE|unix.MS_REC, "")

	oldRoot := filepath.Join(cfg.Mnt, ".oldroot")
	if info, err := os.Stat(oldRoot); err != nil {
		return fmt.Errorf("stat .oldroot: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf(".oldroot is not a directory")
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
	if err := unix.Mount("devpts", devPts, "devpts", 0, "newinstance,ptmxmode=0666,mode=620,gid=5"); err != nil {
		return fmt.Errorf("mount devpts: %w", err)
	}

	if cfg.NoNewPrivs {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("prctl NO_NEW_PRIVS: %w", err)
		}
	}

	if err := applySeccomp(cfg.Seccomp); err != nil {
		return fmt.Errorf("apply seccomp: %w", err)
	}

	if err := dropCapabilities(); err != nil {
		return fmt.Errorf("drop capabilities: %w", err)
	}

	if cfg.GID > 0 {
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
			return fmt.Errorf("drop capability %d: %w", cap, err)
		}
	}
	return nil
}

func ensureSandboxHome(uid, gid int) error {
	home := "/home/sandbox"
	if err := os.MkdirAll(home, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", home, err)
	}
	if err := unix.Chown(home, uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", home, err)
	}
	if err := unix.Chmod(home, 0755); err != nil {
		return fmt.Errorf("chmod %s: %w", home, err)
	}
	return nil
}

func applySeccomp(profile string) error {
	switch profile {
	case "", "off":
		return nil
	case "mvp":
		return installSeccompFilter(false)
	case "strict":
		return installSeccompFilter(true)
	default:
		return fmt.Errorf("unknown profile %q", profile)
	}
}

func installSeccompFilter(strict bool) error {
	deny := []uint32{
		uint32(unix.SYS_BPF),
		uint32(unix.SYS_USERFAULTFD),
		uint32(unix.SYS_PERF_EVENT_OPEN),
		uint32(unix.SYS_PTRACE),
		uint32(unix.SYS_KEXEC_LOAD),
		uint32(unix.SYS_OPEN_BY_HANDLE_AT),
		uint32(unix.SYS_KEYCTL),
		uint32(unix.SYS_ADD_KEY),
		uint32(unix.SYS_REQUEST_KEY),
		uint32(unix.SYS_INIT_MODULE),
		uint32(unix.SYS_FINIT_MODULE),
		uint32(unix.SYS_DELETE_MODULE),
		uint32(unix.SYS_MOUNT),
		uint32(unix.SYS_UMOUNT2),
		uint32(unix.SYS_PIVOT_ROOT),
	}

	if strict {
		deny = append(deny,
			uint32(unix.SYS_SETNS),
			uint32(unix.SYS_UNSHARE),
		)
	}

	filters := make([]unix.SockFilter, 0, len(deny)*2+2)
	filters = append(filters, unix.SockFilter{
		Code: uint16(unix.BPF_LD | unix.BPF_W | unix.BPF_ABS),
		K:    0,
	})

	for _, nr := range deny {
		filters = append(filters,
			unix.SockFilter{
				Code: uint16(unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K),
				Jt:   0,
				Jf:   1,
				K:    nr,
			},
			unix.SockFilter{
				Code: uint16(unix.BPF_RET | unix.BPF_K),
				K:    unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM),
			},
		)
	}

	filters = append(filters, unix.SockFilter{
		Code: uint16(unix.BPF_RET | unix.BPF_K),
		K:    unix.SECCOMP_RET_ALLOW,
	})

	prog := unix.SockFprog{Len: uint16(len(filters)), Filter: &filters[0]}
	if err := unix.Prctl(unix.PR_SET_SECCOMP, uintptr(unix.SECCOMP_MODE_FILTER), uintptr(unsafe.Pointer(&prog)), 0, 0); err != nil {
		return err
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
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: 0, Size: 65536},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: 0, Size: 65536},
		},
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
