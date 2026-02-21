//go:build linux

package linux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DetectOverlayFS verifies overlayfs works by performing a minimal overlay mount
// (create temp lower/upper/work/mnt, mount overlay, unmount, cleanup).
func DetectOverlayFS() error {
	tmpDir, err := os.MkdirTemp("", "sandkasten-overlay-probe-")
	if err != nil {
		return fmt.Errorf("create overlay probe temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	lower := filepath.Join(tmpDir, "lower")
	upper := filepath.Join(tmpDir, "upper")
	work := filepath.Join(tmpDir, "work")
	mnt := filepath.Join(tmpDir, "mnt")
	for _, d := range []string{lower, upper, work, mnt} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create overlay probe dir %s: %w", d, err)
		}
	}
	probeFile := filepath.Join(lower, "probe")
	if err := os.WriteFile(probeFile, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("create overlay probe file: %w", err)
	}

	if err := MountOverlay(lower, upper, work, mnt); err != nil {
		return fmt.Errorf("overlayfs mount probe failed: %w", err)
	}
	defer UmountDetach(mnt)

	if _, err := os.Stat(filepath.Join(mnt, "probe")); err != nil {
		return fmt.Errorf("overlayfs probe file not visible after mount: %w", err)
	}
	return nil
}

// DetectMountPropagation verifies that mount propagation can be set to private,
// by running unshare -m and mount --make-rprivate / in a child process.
// This does not change the daemon's mount namespace.
func DetectMountPropagation() error {
	cmd := exec.Command("unshare", "-m", "--", "mount", "--make-rprivate", "/")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mount propagation check failed (unshare -m mount --make-rprivate /): %w", err)
	}
	return nil
}
