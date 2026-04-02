package autopull

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type imageMeta struct {
	Name      string    `json:"name"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	Layers    []string  `json:"layers,omitempty"`
}

func marshalJSON(meta imageMeta) ([]byte, error) {
	meta.CreatedAt = time.Now().UTC()
	return json.Marshal(meta)
}

func extractLayer(rootfsDir string, layerReader io.Reader) error {
	tarReader := tar.NewReader(layerReader)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, rel, err := secureTargetPath(rootfsDir, header.Name)
		if err != nil {
			return err
		}

		baseName := filepath.Base(rel)
		dirName := filepath.Dir(target)
		if baseName == ".wh..wh..opq" {
			_ = unix.Setxattr(dirName, "trusted.overlay.opaque", []byte("y"), 0)
			continue
		}
		if strings.HasPrefix(baseName, ".wh.") {
			whiteoutTarget := filepath.Join(dirName, strings.TrimPrefix(baseName, ".wh."))
			_ = unix.Mknod(whiteoutTarget, unix.S_IFCHR|0, 0)
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(dirName, 0755); err != nil {
				return err
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(dirName, 0755); err != nil {
				return err
			}
			_ = os.RemoveAll(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.MkdirAll(dirName, 0755); err != nil {
				return err
			}
			_ = os.RemoveAll(target)
			linkTarget, _, err := secureTargetPath(rootfsDir, header.Linkname)
			if err != nil {
				return err
			}
			if err := os.Link(linkTarget, target); err != nil {
				return err
			}
		}
	}
}

func secureTargetPath(rootfsDir, archivePath string) (string, string, error) {
	target, err := evalSymlinksInScope(rootfsDir, archivePath, 0)
	if err != nil {
		return "", "", err
	}
	if !strings.HasPrefix(target, rootfsDir+string(os.PathSeparator)) && target != rootfsDir {
		return "", "", fmt.Errorf("archive path escapes rootfs: %q", archivePath)
	}
	rel := strings.TrimPrefix(target, rootfsDir)
	rel = strings.TrimPrefix(rel, string(os.PathSeparator))
	return target, rel, nil
}

func evalSymlinksInScope(root, archivePath string, depth int) (string, error) {
	if depth > 255 {
		return "", fmt.Errorf("too many symlinks")
	}
	root = filepath.Clean(root)
	archivePath = filepath.Clean("/" + filepath.ToSlash(archivePath))
	parts := strings.Split(archivePath, "/")

	current := root
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if current != root {
				current = filepath.Dir(current)
			}
			continue
		}

		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if err != nil {
			if os.IsNotExist(err) {
				current = next
				continue
			}
			return "", err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(next)
			if err != nil {
				return "", err
			}
			var nextPath string
			if filepath.IsAbs(target) {
				nextPath = target
			} else {
				relToRoot := strings.TrimPrefix(filepath.Dir(next), root)
				if relToRoot == "" {
					relToRoot = "/"
				}
				nextPath = filepath.Join(relToRoot, target)
			}
			resolved, err := evalSymlinksInScope(root, nextPath, depth+1)
			if err != nil {
				return "", err
			}
			current = resolved
		} else {
			current = next
		}
	}
	return current, nil
}

func injectRunner(dataDir string) error {
	runnerDst := filepath.Join(dataDir, "layers", "runner", "rootfs", "usr", "local", "bin", "runner")
	if _, err := os.Stat(runnerDst); err == nil {
		return nil // already injected
	}

	if err := os.MkdirAll(filepath.Dir(runnerDst), 0755); err != nil {
		return fmt.Errorf("create runner dir: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	binDir := filepath.Dir(exePath)
	runnerSrc := filepath.Join(binDir, "runner")

	if _, err := os.Stat(runnerSrc); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("runner binary not found at %s - run 'task build' first", runnerSrc)
	}

	srcFile, err := os.Open(runnerSrc)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(runnerDst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return os.Chmod(runnerDst, 0755)
}
