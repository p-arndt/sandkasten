//go:build linux

package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const cgroupRoot = "/sys/fs/cgroup/sandkasten"

type CgroupConfig struct {
	CPULimit   float64 // CPUs (e.g., 1.0 = 1 CPU)
	MemLimitMB int     // Memory in MB
	PidsLimit  int     // Max processes
}

func CgroupPath(sessionID string) string {
	return filepath.Join(cgroupRoot, sessionID)
}

func CreateCgroup(sessionID string, cfg CgroupConfig) (string, error) {
	cgPath := CgroupPath(sessionID)
	if err := os.MkdirAll(cgPath, 0755); err != nil {
		return "", fmt.Errorf("create cgroup %s: %w", cgPath, err)
	}

	if cfg.MemLimitMB > 0 {
		memBytes := int64(cfg.MemLimitMB) * 1024 * 1024
		if err := os.WriteFile(filepath.Join(cgPath, "memory.max"), []byte(strconv.FormatInt(memBytes, 10)), 0644); err != nil {
			return "", fmt.Errorf("set memory.max: %w", err)
		}
	}

	if cfg.PidsLimit > 0 {
		if err := os.WriteFile(filepath.Join(cgPath, "pids.max"), []byte(strconv.Itoa(cfg.PidsLimit)), 0644); err != nil {
			return "", fmt.Errorf("set pids.max: %w", err)
		}
	}

	if cfg.CPULimit > 0 {
		quota := int64(cfg.CPULimit * 100000)
		cpuMax := fmt.Sprintf("%d 100000", quota)
		if err := os.WriteFile(filepath.Join(cgPath, "cpu.max"), []byte(cpuMax), 0644); err != nil {
			return "", fmt.Errorf("set cpu.max: %w", err)
		}
	}

	return cgPath, nil
}

func AttachToCgroup(cgPath string, pid int) error {
	procsPath := filepath.Join(cgPath, "cgroup.procs")
	if err := os.WriteFile(procsPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("attach pid %d to cgroup: %w", pid, err)
	}
	return nil
}

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

func RemoveCgroup(sessionID string) error {
	cgPath := CgroupPath(sessionID)
	if err := os.RemoveAll(cgPath); err != nil {
		return fmt.Errorf("remove cgroup %s: %w", cgPath, err)
	}
	return nil
}

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
