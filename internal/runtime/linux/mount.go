//go:build linux

package linux

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func MkdirAll(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return nil
}

func MountOverlay(lower, upper, work, mnt string) error {
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := unix.Mount("overlay", mnt, "overlay", 0, opts); err != nil {
		return fmt.Errorf("mount overlay %s: %w", mnt, err)
	}
	return nil
}

func BindMount(src, dst string, recursive bool) error {
	flags := unix.MS_BIND
	if recursive {
		flags |= unix.MS_REC
	}
	if err := unix.Mount(src, dst, "", uintptr(flags), ""); err != nil {
		return fmt.Errorf("bind mount %s -> %s: %w", src, dst, err)
	}
	return nil
}

func MountTmpfs(target string, sizeBytes int64) error {
	opts := fmt.Sprintf("size=%d", sizeBytes)
	if err := unix.Mount("tmpfs", target, "tmpfs", 0, opts); err != nil {
		return fmt.Errorf("mount tmpfs %s: %w", target, err)
	}
	return nil
}

func MountProc(target string) error {
	if err := unix.Mount("proc", target, "proc", 0, ""); err != nil {
		return fmt.Errorf("mount proc %s: %w", target, err)
	}
	return nil
}

func MakePrivate(mountPoint string) error {
	if err := unix.Mount("", mountPoint, "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make private %s: %w", mountPoint, err)
	}
	return nil
}

func PivotRoot(newRoot, putOld string) error {
	if err := unix.Chdir(newRoot); err != nil {
		return fmt.Errorf("chdir %s: %w", newRoot, err)
	}
	if err := unix.PivotRoot(newRoot, putOld); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	return nil
}

func UmountDetach(target string) error {
	if err := unix.Unmount(target, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("umount %s: %w", target, err)
	}
	return nil
}

func SetupMinimalDev(mnt string) error {
	devDir := filepath.Join(mnt, "dev")
	if err := MkdirAll(devDir); err != nil {
		return err
	}
	if err := MountTmpfs(devDir, 16*1024*1024); err != nil {
		return err
	}

	devices := []struct {
		path  string
		mode  uint32
		major uint32
		minor uint32
	}{
		{filepath.Join(devDir, "null"), unix.S_IFCHR | 0666, 1, 3},
		{filepath.Join(devDir, "zero"), unix.S_IFCHR | 0666, 1, 5},
		{filepath.Join(devDir, "random"), unix.S_IFCHR | 0666, 1, 8},
		{filepath.Join(devDir, "urandom"), unix.S_IFCHR | 0666, 1, 9},
		{filepath.Join(devDir, "tty"), unix.S_IFCHR | 0666, 5, 0},
	}

	for _, d := range devices {
		dev := int(d.major<<8 | d.minor)
		if err := unix.Mknod(d.path, d.mode, dev); err != nil {
			if !os.IsExist(err) {
				return fmt.Errorf("mknod %s: %w", d.path, err)
			}
		}
	}

	for _, link := range []struct {
		src, dst string
	}{
		{"pts/ptmx", filepath.Join(devDir, "ptmx")},
		{"/proc/self/fd", filepath.Join(devDir, "fd")},
		{"/proc/self/fd/0", filepath.Join(devDir, "stdin")},
		{"/proc/self/fd/1", filepath.Join(devDir, "stdout")},
		{"/proc/self/fd/2", filepath.Join(devDir, "stderr")},
	} {
		if err := os.Symlink(link.src, link.dst); err != nil {
			if !os.IsExist(err) {
				return fmt.Errorf("symlink %s: %w", link.dst, err)
			}
		}
	}

	return nil
}

func BindHostFile(mnt, hostPath, relPath string) error {
	if _, err := os.Stat(hostPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat host file %s: %w", hostPath, err)
	}

	dst := filepath.Join(mnt, relPath)
	if err := MkdirAll(filepath.Dir(dst)); err != nil {
		return err
	}

	if fi, err := os.Lstat(dst); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(dst); err != nil {
				return fmt.Errorf("remove symlink %s: %w", dst, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("lstat %s: %w", dst, err)
	}

	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.WriteFile(dst, []byte{}, 0644); err != nil {
			return fmt.Errorf("create file %s: %w", dst, err)
		}
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", dst, err)
	}

	if err := BindMount(hostPath, dst, false); err != nil {
		return fmt.Errorf("bind file %s -> %s: %w", hostPath, dst, err)
	}

	return nil
}

func SetupFilesystem(lower, upper, work, mnt, workspaceSrc, runHostDir string) error {
	if err := MkdirAll(upper); err != nil {
		return err
	}
	if err := MkdirAll(work); err != nil {
		return err
	}
	if err := MkdirAll(mnt); err != nil {
		return err
	}
	if err := MkdirAll(runHostDir); err != nil {
		return err
	}

	if err := MountOverlay(lower, upper, work, mnt); err != nil {
		return err
	}

	if err := BindHostFile(mnt, "/etc/resolv.conf", "etc/resolv.conf"); err != nil {
		return err
	}
	if err := BindHostFile(mnt, "/etc/hosts", "etc/hosts"); err != nil {
		return err
	}

	workspaceDst := filepath.Join(mnt, "workspace")
	if err := MkdirAll(workspaceDst); err != nil {
		return err
	}
	if workspaceSrc != "" {
		if err := BindMount(workspaceSrc, workspaceDst, true); err != nil {
			return err
		}
	}

	runDst := filepath.Join(mnt, "run", "sandkasten")
	if err := MkdirAll(runDst); err != nil {
		return err
	}
	if err := BindMount(runHostDir, runDst, false); err != nil {
		return err
	}

	tmpDst := filepath.Join(mnt, "tmp")
	if err := MkdirAll(tmpDst); err != nil {
		return err
	}
	if err := MountTmpfs(tmpDst, 512*1024*1024); err != nil {
		return err
	}

	if err := SetupMinimalDev(mnt); err != nil {
		return err
	}

	return nil
}

func CleanupMounts(mnt string) {
	_ = unix.Unmount(mnt, unix.MNT_DETACH)
}
