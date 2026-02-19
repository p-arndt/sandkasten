//go:build linux

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const defaultDataDir = "/var/lib/sandkasten"

type ImageMeta struct {
	Name      string    `json:"name"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	Layers    []string  `json:"layers,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	dataDir := os.Getenv("SANDKASTEN_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	switch os.Args[1] {
	case "import":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: imgbuilder import --name <image> --tar <file>\n")
			os.Exit(1)
		}
		name := ""
		tarPath := ""
		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "--name" && i+1 < len(os.Args) {
				name = os.Args[i+1]
				i++
			} else if os.Args[i] == "--tar" && i+1 < len(os.Args) {
				tarPath = os.Args[i+1]
				i++
			}
		}
		if name == "" || tarPath == "" {
			fmt.Fprintf(os.Stderr, "Error: --name and --tar are required\n")
			os.Exit(1)
		}
		if err := importImage(dataDir, name, tarPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Imported image: %s\n", name)

	case "list":
		if err := listImages(dataDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "validate":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: imgbuilder validate <image>\n")
			os.Exit(1)
		}
		if err := validateImage(dataDir, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Image %s is valid\n", os.Args[2])

	case "delete":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: imgbuilder delete <image>\n")
			os.Exit(1)
		}
		if err := deleteImage(dataDir, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Deleted image: %s\n", os.Args[2])

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: imgbuilder <command> [options]

Commands:
  import   --name <image> --tar <file>   Import a rootfs tarball
  list                                    List available images
  validate <image>                        Validate an image
  delete <image>                          Delete an image

Environment:
  SANDKASTEN_DATA_DIR   Data directory (default: /var/lib/sandkasten)
`)
}

func importImage(dataDir, name, tarPath string) error {
	imageDir := filepath.Join(dataDir, "images", name)
	rootfsDir := filepath.Join(imageDir, "rootfs")

	if _, err := os.Stat(rootfsDir); err == nil {
		return fmt.Errorf("image %s already exists", name)
	}

	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return fmt.Errorf("create rootfs dir: %w", err)
	}

	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar file: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file
	if filepath.Ext(tarPath) == ".gz" || filepath.Ext(tarPath) == ".tgz" {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		target := filepath.Join(rootfsDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", filepath.Dir(target), err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", filepath.Dir(target), err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
		}
	}

	meta := ImageMeta{
		Name:      name,
		Hash:      "sha256:" + fmt.Sprintf("%x", time.Now().UnixNano()),
		CreatedAt: time.Now().UTC(),
	}
	metaPath := filepath.Join(imageDir, "meta.json")
	metaFile, err := os.Create(metaPath)
	if err != nil {
		return fmt.Errorf("create meta file: %w", err)
	}
	defer metaFile.Close()

	if err := json.NewEncoder(metaFile).Encode(meta); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}

	runnerDst := filepath.Join(rootfsDir, "usr", "local", "bin", "runner")
	if err := os.MkdirAll(filepath.Dir(runnerDst), 0755); err != nil {
		return fmt.Errorf("create runner dir: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	binDir := filepath.Dir(exePath)
	runnerSrc := filepath.Join(binDir, "runner")

	if _, err := os.Stat(runnerSrc); os.IsNotExist(err) {
		return fmt.Errorf("runner binary not found at %s - run 'task build' first", runnerSrc)
	}

	if err := copyFile(runnerSrc, runnerDst); err != nil {
		return fmt.Errorf("copy runner: %w", err)
	}
	if err := os.Chmod(runnerDst, 0755); err != nil {
		return fmt.Errorf("chmod runner: %w", err)
	}

	return nil
}

func listImages(dataDir string) error {
	imageDir := filepath.Join(dataDir, "images")
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No images found")
			return nil
		}
		return fmt.Errorf("read images dir: %w", err)
	}

	fmt.Println("Images:")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(imageDir, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			fmt.Printf("  - %s (metadata missing)\n", entry.Name())
			continue
		}
		var meta ImageMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			fmt.Printf("  - %s (invalid metadata)\n", entry.Name())
			continue
		}
		fmt.Printf("  - %s (created: %s)\n", meta.Name, meta.CreatedAt.Format(time.RFC3339))
	}

	return nil
}

func validateImage(dataDir, name string) error {
	imageDir := filepath.Join(dataDir, "images", name)
	metaPath := filepath.Join(imageDir, "meta.json")

	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("read meta: %w", err)
	}
	var meta ImageMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return fmt.Errorf("parse meta: %w", err)
	}

	if len(meta.Layers) > 0 {
		for _, layer := range meta.Layers {
			if _, err := os.Stat(filepath.Join(dataDir, "layers", layer, "rootfs")); err != nil {
				return fmt.Errorf("missing layer: %s", layer)
			}
		}
		if _, err := os.Stat(filepath.Join(dataDir, "layers", "runner", "rootfs", "usr", "local", "bin", "runner")); err != nil {
			return fmt.Errorf("missing runner layer")
		}
		return nil
	}

	rootfsDir := filepath.Join(imageDir, "rootfs")
	if _, err := os.Stat(rootfsDir); os.IsNotExist(err) {
		return fmt.Errorf("image rootfs not found")
	}

	required := []string{
		"/bin/sh",
		"/usr/local/bin/runner",
	}

	for _, path := range required {
		fullPath := filepath.Join(rootfsDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("required file missing: %s", path)
		}
	}

	runnerPath := filepath.Join(rootfsDir, "usr", "local", "bin", "runner")
	if err := unix.Access(runnerPath, unix.X_OK); err != nil {
		return fmt.Errorf("runner is not executable")
	}

	return nil
}

func deleteImage(dataDir, name string) error {
	imageDir := filepath.Join(dataDir, "images", name)
	if err := os.RemoveAll(imageDir); err != nil {
		return fmt.Errorf("remove image: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
