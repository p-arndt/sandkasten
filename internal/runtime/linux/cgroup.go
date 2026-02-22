//go:build linux

// Cgroup v2 integration for resource limits. Each session gets its own cgroup under
// /sys/fs/cgroup/sandkasten/<sessionID> with CPU, memory, and PIDs controllers.
//
// Example: With CPULimit=2.0, MemLimitMB=512, PidsLimit=100:
//   - cpu.max: "200000 100000" (2 cores worth of quota per 100ms period)
//   - memory.max: 536870912
//   - memory.swap.max: 0 (no swap, prevents bypassing mem limit)
//   - pids.max: 100
package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// CgroupConfig holds resource limits applied to a session's cgroup.
type CgroupConfig struct {
	CPULimit   float64 // CPU cores (e.g. 2.0 = 2 cores); 0 = unlimited
	MemLimitMB int     // Memory limit in MiB; 0 = unlimited
	PidsLimit  int     // Max processes; 0 = unlimited
}

// getCgroupPath returns the cgroup v2 root for this process (e.g. /sys/fs/cgroup or
// /sys/fs/cgroup/user.slice/user-1000.slice if under user delegation).
func getCgroupPath() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 && parts[2] != "" {
			path := strings.TrimPrefix(parts[2], "/")
			if path != "" {
				return filepath.Join("/sys/fs/cgroup", path)
			}
		}
	}
	return "/sys/fs/cgroup"
}

// CgroupPath returns the full path to a session's cgroup, e.g. /sys/fs/cgroup/sandkasten/<sessionID>.
func CgroupPath(sessionID string) string {
	base := getCgroupPath()
	return filepath.Join(base, "sandkasten", sessionID)
}

// enableControllers propagates cpu, memory, pids controllers down the hierarchy to the session
// cgroup. In cgroup v2, controllers must be explicitly enabled in cgroup.subtree_control at
// each level before child cgroups can use them.
func enableControllers(cgPath string) {
	parts := strings.Split(strings.TrimPrefix(cgPath, "/sys/fs/cgroup"), "/")
	current := "/sys/fs/cgroup"

	for _, part := range parts {
		if part != "" {
			current = filepath.Join(current, part)
		}
		data, err := os.ReadFile(filepath.Join(current, "cgroup.controllers"))
		if err != nil {
			continue
		}
		var enable []string
		for _, c := range strings.Split(strings.TrimSpace(string(data)), " ") {
			if c == "cpu" || c == "memory" || c == "pids" {
				enable = append(enable, "+"+c)
			}
		}
		if len(enable) > 0 {
			_ = os.WriteFile(filepath.Join(current, "cgroup.subtree_control"), []byte(strings.Join(enable, " ")), 0644)
		}
	}
}

// CreateCgroup creates a new cgroup for the session under sandkasten/<sessionID>, enables
// controllers, and applies limits. Returns the full cgroup path. The cgroup is empty until
// AttachToCgroup is called.
func CreateCgroup(sessionID string, cfg CgroupConfig) (string, error) {
	basePath := getCgroupPath()
	parentPath := filepath.Join(basePath, "sandkasten")
	if err := os.MkdirAll(parentPath, 0755); err != nil {
		return "", fmt.Errorf("create parent cgroup %s: %w", parentPath, err)
	}

	enableControllers(parentPath)

	cgPath := CgroupPath(sessionID)

	if err := os.MkdirAll(cgPath, 0755); err != nil {
		return "", fmt.Errorf("create cgroup %s: %w", cgPath, err)
	}

	if cfg.MemLimitMB > 0 {
		memBytes := int64(cfg.MemLimitMB) * 1024 * 1024
		memPath := filepath.Join(cgPath, "memory.max")
		if err := os.WriteFile(memPath, []byte(strconv.FormatInt(memBytes, 10)), 0644); err != nil {
			if os.IsPermission(err) {
				// Cgroup may not be delegated (e.g. under systemd); warn but continue
				fmt.Fprintf(os.Stderr, "warning: cannot set memory limit (cgroup not delegated): %v\n", err)
			} else {
				return "", fmt.Errorf("set memory.max: %w", err)
			}
		}

		// memory.swap.max=0 prevents using swap; otherwise tasks could bypass memory limits.
		swapPath := filepath.Join(cgPath, "memory.swap.max")
		if err := os.WriteFile(swapPath, []byte("0"), 0644); err != nil {
			// Ignore if swap controller is not enabled or available
		}
	}

	if cfg.PidsLimit > 0 {
		pidsPath := filepath.Join(cgPath, "pids.max")
		if err := os.WriteFile(pidsPath, []byte(strconv.Itoa(cfg.PidsLimit)), 0644); err != nil {
			if os.IsPermission(err) {
				fmt.Fprintf(os.Stderr, "warning: cannot set pids limit (cgroup not delegated): %v\n", err)
			} else {
				return "", fmt.Errorf("set pids.max: %w", err)
			}
		}
	}

	if cfg.CPULimit > 0 {
		// cpu.max format: "quota period" in microseconds. 100000 us = 100ms period.
		// quota = CPULimit * 100000 means "CPULimit cores worth of CPU per 100ms".
		quota := int64(cfg.CPULimit * 100000)
		cpuMax := fmt.Sprintf("%d 100000", quota)
		cpuPath := filepath.Join(cgPath, "cpu.max")
		if err := os.WriteFile(cpuPath, []byte(cpuMax), 0644); err != nil {
			if os.IsPermission(err) {
				fmt.Fprintf(os.Stderr, "warning: cannot set cpu limit (cgroup not delegated): %v\n", err)
			} else {
				return "", fmt.Errorf("set cpu.max: %w", err)
			}
		}
	}

	return cgPath, nil
}

// AttachToCgroup moves the given PID into the cgroup by writing it to cgroup.procs.
// All descendants of this process inherit the cgroup.
func AttachToCgroup(cgPath string, pid int) error {
	procsPath := filepath.Join(cgPath, "cgroup.procs")
	if err := os.WriteFile(procsPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("attach pid %d to cgroup: %w", pid, err)
	}
	return nil
}

// KillCgroupProcesses sends SIGKILL to all processes in the cgroup (read from cgroup.procs).
// Used during Destroy to ensure no processes remain before removing the cgroup.
func KillCgroupProcesses(cgPath string) error {
	procsPath := filepath.Join(cgPath, "cgroup.procs")
	data, err := os.ReadFile(procsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read cgroup.procs: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		_ = unix.Kill(pid, unix.SIGKILL)
	}
	return nil
}

// RemoveCgroup removes the session's cgroup directory. Processes must be killed first.
func RemoveCgroup(sessionID string) error {
	cgPath := CgroupPath(sessionID)
	if err := os.RemoveAll(cgPath); err != nil {
		return fmt.Errorf("remove cgroup %s: %w", cgPath, err)
	}
	return nil
}

// DetectCgroupV2 verifies that cgroup v2 is mounted at /sys/fs/cgroup (statfs CGROUP2_SUPER_MAGIC).
// Required at driver init; returns error if v1 or hybrid setup is detected.
func DetectCgroupV2() error {
	var stat unix.Statfs_t
	if err := unix.Statfs("/sys/fs/cgroup", &stat); err != nil {
		return fmt.Errorf("stat /sys/fs/cgroup: %w", err)
	}
	if stat.Type != unix.CGROUP2_SUPER_MAGIC {
		return fmt.Errorf("cgroup v2 not mounted at /sys/fs/cgroup")
	}
	return nil
}
