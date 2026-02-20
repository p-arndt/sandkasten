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

type CgroupConfig struct {
	CPULimit   float64
	MemLimitMB int
	PidsLimit  int
}

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

func CgroupPath(sessionID string) string {
	base := getCgroupPath()
	return filepath.Join(base, "sandkasten", sessionID)
}

func enableControllers(cgPath string) {
	data, err := os.ReadFile(filepath.Join(cgPath, "cgroup.controllers"))
	if err != nil {
		return
	}
	var enable []string
	for _, c := range strings.Split(strings.TrimSpace(string(data)), " ") {
		if c == "cpu" || c == "memory" || c == "pids" {
			enable = append(enable, "+"+c)
		}
	}
	if len(enable) > 0 {
		_ = os.WriteFile(filepath.Join(cgPath, "cgroup.subtree_control"), []byte(strings.Join(enable, " ")), 0644)
	}
}

func CreateCgroup(sessionID string, cfg CgroupConfig) (string, error) {
	basePath := getCgroupPath()
	parentPath := filepath.Join(basePath, "sandkasten")
	if err := os.MkdirAll(parentPath, 0755); err != nil {
		return "", fmt.Errorf("create parent cgroup %s: %w", parentPath, err)
	}

	enableControllers(basePath)
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
				fmt.Fprintf(os.Stderr, "warning: cannot set memory limit (cgroup not delegated): %v\n", err)
			} else {
				return "", fmt.Errorf("set memory.max: %w", err)
			}
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
