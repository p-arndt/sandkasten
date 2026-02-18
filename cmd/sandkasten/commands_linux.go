//go:build linux

package main

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

const (
	defaultDataDir = "/var/lib/sandkasten"
	defaultListen  = "127.0.0.1:8080"
)

var imageNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type ImageMeta struct {
	Name      string    `json:"name"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

type initConfigDefaults struct {
	CPULimit         float64 `yaml:"cpu_limit"`
	MemLimitMB       int     `yaml:"mem_limit_mb"`
	PidsLimit        int     `yaml:"pids_limit"`
	MaxExecTimeoutMs int     `yaml:"max_exec_timeout_ms"`
	NetworkMode      string  `yaml:"network_mode"`
	ReadonlyRootfs   bool    `yaml:"readonly_rootfs"`
}

type initConfig struct {
	Listen       string             `yaml:"listen"`
	APIKey       string             `yaml:"api_key"`
	DataDir      string             `yaml:"data_dir"`
	DefaultImage string             `yaml:"default_image"`
	AllowedImage []string           `yaml:"allowed_images"`
	DBPath       string             `yaml:"db_path"`
	Defaults     initConfigDefaults `yaml:"defaults"`
}

type doctorCheck struct {
	Name    string
	Status  string
	Details string
}

func runImage(args []string) int {
	if len(args) == 0 {
		printImageUsage()
		return 1
	}

	switch args[0] {
	case "pull":
		return runImagePull(args[1:])
	case "list":
		return runImageList(args[1:])
	case "validate":
		return runImageValidate(args[1:])
	case "delete":
		return runImageDelete(args[1:])
	default:
		printImageUsage()
		return 1
	}
}

func printMainUsage() {
	fmt.Fprint(os.Stderr, `Usage:
  sandkasten [--config <path>] [--log-level <level>]      Run daemon
  sandkasten doctor [--data-dir <dir>]                    Run environment checks
  sandkasten init [options]                               Bootstrap config and data dir
  sandkasten image <command> [options]                    Manage images

Image commands:
  sandkasten image pull <ref> [--name <image>] [--data-dir <dir>]
  sandkasten image list [--data-dir <dir>]
  sandkasten image validate <image> [--data-dir <dir>]
  sandkasten image delete <image> [--data-dir <dir>]

Init defaults:
  --config sandkasten.yaml
  --data-dir /var/lib/sandkasten
  --default-image base
  --pull alpine:latest
`)
}

func runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dataDir := fs.String("data-dir", envOrDefault("SANDKASTEN_DATA_DIR", defaultDataDir), "sandkasten data directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	checks := make([]doctorCheck, 0, 8)
	failures := 0

	if runtime.GOOS != "linux" {
		checks = append(checks, doctorCheck{Name: "Linux runtime", Status: "FAIL", Details: "sandkasten requires Linux or WSL2"})
		failures++
	} else {
		checks = append(checks, doctorCheck{Name: "Linux runtime", Status: "OK", Details: runtime.GOOS})
	}

	if ok, details := checkKernelVersion(); ok {
		checks = append(checks, doctorCheck{Name: "Kernel >= 5.11", Status: "OK", Details: details})
	} else {
		checks = append(checks, doctorCheck{Name: "Kernel >= 5.11", Status: "FAIL", Details: details})
		failures++
	}

	if ok, details := checkCgroupV2(); ok {
		checks = append(checks, doctorCheck{Name: "cgroups v2", Status: "OK", Details: details})
	} else {
		checks = append(checks, doctorCheck{Name: "cgroups v2", Status: "FAIL", Details: details})
		failures++
	}

	if ok, details := checkOverlayFS(); ok {
		checks = append(checks, doctorCheck{Name: "overlayfs", Status: "OK", Details: details})
	} else {
		checks = append(checks, doctorCheck{Name: "overlayfs", Status: "FAIL", Details: details})
		failures++
	}

	if os.Geteuid() == 0 {
		checks = append(checks, doctorCheck{Name: "Privileges", Status: "OK", Details: "running as root"})
	} else {
		checks = append(checks, doctorCheck{Name: "Privileges", Status: "WARN", Details: "daemon needs root or CAP_SYS_ADMIN"})
	}

	if ok, status, details := checkDataDir(*dataDir); ok {
		checks = append(checks, doctorCheck{Name: "Data directory", Status: status, Details: details})
	} else {
		checks = append(checks, doctorCheck{Name: "Data directory", Status: status, Details: details})
		if status == "FAIL" {
			failures++
		}
	}

	if ok, details := checkRunnerBinary(); ok {
		checks = append(checks, doctorCheck{Name: "Runner binary", Status: "OK", Details: details})
	} else {
		checks = append(checks, doctorCheck{Name: "Runner binary", Status: "WARN", Details: details})
	}

	fmt.Println("Sandkasten doctor")
	for _, check := range checks {
		fmt.Printf("[%s] %-16s %s\n", check.Status, check.Name, check.Details)
	}

	if failures > 0 {
		fmt.Printf("\nDoctor found %d blocking issue(s).\n", failures)
		return 1
	}

	fmt.Println("\nDoctor checks passed.")
	return 0
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "sandkasten.yaml", "path for generated config")
	dataDir := fs.String("data-dir", envOrDefault("SANDKASTEN_DATA_DIR", defaultDataDir), "sandkasten data directory")
	listen := fs.String("listen", defaultListen, "daemon listen address")
	apiKey := fs.String("api-key", "", "API key to write to config (auto-generated if empty)")
	defaultImage := fs.String("default-image", "base", "default image name")
	pullRef := fs.String("pull", "alpine:latest", "OCI image reference to pull")
	skipPull := fs.Bool("skip-pull", false, "skip pulling default image")
	force := fs.Bool("force", false, "overwrite existing config")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if !imageNamePattern.MatchString(*defaultImage) {
		fmt.Fprintf(os.Stderr, "Error: invalid --default-image %q\n", *defaultImage)
		return 1
	}

	if *apiKey == "" {
		generated, err := generateAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating API key: %v\n", err)
			return 1
		}
		*apiKey = generated
	}

	if err := os.MkdirAll(filepath.Join(*dataDir, "images"), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating images dir: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Join(*dataDir, "sessions"), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating sessions dir: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Join(*dataDir, "workspaces"), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating workspaces dir: %v\n", err)
		return 1
	}

	if err := writeInitialConfig(*configPath, *listen, *apiKey, *dataDir, *defaultImage, *force); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	if !*skipPull {
		if exists := imageExists(*dataDir, *defaultImage); exists {
			fmt.Printf("Image %q already exists, skipping pull.\n", *defaultImage)
		} else {
			fmt.Printf("Pulling %s as image %q...\n", *pullRef, *defaultImage)
			if err := pullImage(*dataDir, *defaultImage, *pullRef); err != nil {
				fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
				return 1
			}
		}
	}

	fmt.Println("Sandkasten initialized.")
	fmt.Printf("- Config: %s\n", *configPath)
	fmt.Printf("- Data dir: %s\n", *dataDir)
	fmt.Printf("- Default image: %s\n", *defaultImage)
	fmt.Printf("- Start daemon: sudo ./bin/sandkasten --config %s\n", *configPath)
	return 0
}

func runImagePull(args []string) int {
	fs := flag.NewFlagSet("image pull", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dataDir := fs.String("data-dir", envOrDefault("SANDKASTEN_DATA_DIR", defaultDataDir), "sandkasten data directory")
	imageName := fs.String("name", "", "sandkasten image name (defaults to repository name)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: sandkasten image pull [--name <image>] [--data-dir <dir>] <oci-reference>")
		return 1
	}

	ref := fs.Arg(0)
	if *imageName == "" {
		parsed, err := name.ParseReference(ref, name.WeakValidation)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid reference %q: %v\n", ref, err)
			return 1
		}
		*imageName = path.Base(parsed.Context().RepositoryStr())
	}

	if !imageNamePattern.MatchString(*imageName) {
		fmt.Fprintf(os.Stderr, "Error: invalid image name %q\n", *imageName)
		return 1
	}

	if err := pullImage(*dataDir, *imageName, ref); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Pulled image: %s (%s)\n", *imageName, ref)
	return 0
}

func runImageList(args []string) int {
	fs := flag.NewFlagSet("image list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dataDir := fs.String("data-dir", envOrDefault("SANDKASTEN_DATA_DIR", defaultDataDir), "sandkasten data directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := listImages(*dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func runImageValidate(args []string) int {
	fs := flag.NewFlagSet("image validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dataDir := fs.String("data-dir", envOrDefault("SANDKASTEN_DATA_DIR", defaultDataDir), "sandkasten data directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: sandkasten image validate [--data-dir <dir>] <image>")
		return 1
	}

	if err := validateImage(*dataDir, fs.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Image %s is valid\n", fs.Arg(0))
	return 0
}

func runImageDelete(args []string) int {
	fs := flag.NewFlagSet("image delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dataDir := fs.String("data-dir", envOrDefault("SANDKASTEN_DATA_DIR", defaultDataDir), "sandkasten data directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: sandkasten image delete [--data-dir <dir>] <image>")
		return 1
	}

	if err := deleteImage(*dataDir, fs.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Deleted image: %s\n", fs.Arg(0))
	return 0
}

func pullImage(dataDir, nameValue, ref string) (err error) {
	if !imageNamePattern.MatchString(nameValue) {
		return fmt.Errorf("invalid image name %q", nameValue)
	}

	imageDir := filepath.Join(dataDir, "images", nameValue)
	rootfsDir := filepath.Join(imageDir, "rootfs")

	if _, statErr := os.Stat(rootfsDir); statErr == nil {
		return fmt.Errorf("image %s already exists", nameValue)
	}

	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return fmt.Errorf("create rootfs dir: %w", err)
	}

	defer func() {
		if err != nil {
			_ = os.RemoveAll(imageDir)
		}
	}()

	parsedRef, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	img, err := remote.Image(parsedRef, remote.WithContext(context.Background()))
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("resolve layers: %w", err)
	}

	for _, layer := range layers {
		reader, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("open layer: %w", err)
		}

		if err := extractLayer(rootfsDir, reader); err != nil {
			reader.Close()
			return fmt.Errorf("extract layer: %w", err)
		}
		if err := reader.Close(); err != nil {
			return fmt.Errorf("close layer: %w", err)
		}
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("compute digest: %w", err)
	}

	meta := ImageMeta{
		Name:      nameValue,
		Hash:      digest.String(),
		CreatedAt: time.Now().UTC(),
	}
	if err := writeMeta(filepath.Join(imageDir, "meta.json"), meta); err != nil {
		return err
	}

	if err := injectRunner(rootfsDir); err != nil {
		return err
	}

	return nil
}

func listImages(dataDir string) error {
	imageDir := filepath.Join(dataDir, "images")
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
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

func validateImage(dataDir, nameValue string) error {
	imageDir := filepath.Join(dataDir, "images", nameValue)
	rootfsDir := filepath.Join(imageDir, "rootfs")

	if _, err := os.Stat(rootfsDir); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("image rootfs not found")
	}

	required := []string{
		"bin/sh",
		"usr/local/bin/runner",
	}

	for _, rel := range required {
		fullPath := filepath.Join(rootfsDir, rel)
		if _, err := os.Stat(fullPath); errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("required file missing: /%s", rel)
		}
	}

	runnerPath := filepath.Join(rootfsDir, "usr", "local", "bin", "runner")
	info, err := os.Stat(runnerPath)
	if err != nil {
		return fmt.Errorf("stat runner: %w", err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("runner is not executable")
	}

	return nil
}

func deleteImage(dataDir, nameValue string) error {
	imageDir := filepath.Join(dataDir, "images", nameValue)
	if err := os.RemoveAll(imageDir); err != nil {
		return fmt.Errorf("remove image: %w", err)
	}
	return nil
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
			entries, err := os.ReadDir(dirName)
			if err == nil {
				for _, entry := range entries {
					if removeErr := os.RemoveAll(filepath.Join(dirName, entry.Name())); removeErr != nil {
						return removeErr
					}
				}
			}
			continue
		}
		if strings.HasPrefix(baseName, ".wh.") {
			whiteoutTarget := filepath.Join(dirName, strings.TrimPrefix(baseName, ".wh."))
			if err := os.RemoveAll(whiteoutTarget); err != nil {
				return err
			}
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
			if err := os.RemoveAll(target); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}

		case tar.TypeLink:
			if err := os.MkdirAll(dirName, 0755); err != nil {
				return err
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
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

func writeMeta(metaPath string, meta ImageMeta) error {
	metaFile, err := os.Create(metaPath)
	if err != nil {
		return fmt.Errorf("create meta file: %w", err)
	}
	defer metaFile.Close()

	if err := json.NewEncoder(metaFile).Encode(meta); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}

	return nil
}

func injectRunner(rootfsDir string) error {
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

	if _, err := os.Stat(runnerSrc); errors.Is(err, fs.ErrNotExist) {
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

func secureTargetPath(rootfsDir, archivePath string) (string, string, error) {
	clean := filepath.Clean("/" + filepath.FromSlash(archivePath))
	rel := strings.TrimPrefix(clean, "/")
	if rel == "" || rel == "." {
		return "", "", fmt.Errorf("invalid archive path %q", archivePath)
	}
	target := filepath.Join(rootfsDir, rel)
	if !strings.HasPrefix(target, rootfsDir+string(os.PathSeparator)) && target != rootfsDir {
		return "", "", fmt.Errorf("archive path escapes rootfs: %q", archivePath)
	}
	return target, rel, nil
}

func printImageUsage() {
	fmt.Fprint(os.Stderr, `Usage: sandkasten image <command> [options]

Commands:
  pull <ref> [--name <image>] [--data-dir <dir>]   Pull OCI image from registry
  list [--data-dir <dir>]                           List available images
  validate <image> [--data-dir <dir>]               Validate an image
  delete <image> [--data-dir <dir>]                 Delete an image

Environment:
  SANDKASTEN_DATA_DIR   Data directory (default: /var/lib/sandkasten)
`)
}

func checkKernelVersion() (bool, string) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return false, fmt.Sprintf("uname failed: %v", err)
	}
	release := charsToString(uts.Release[:])
	major, minor := parseKernelVersion(release)
	if major > 5 || (major == 5 && minor >= 11) {
		return true, release
	}
	return false, fmt.Sprintf("%s (need >= 5.11)", release)
}

func checkCgroupV2() (bool, string) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/sys/fs/cgroup", &stat); err != nil {
		return false, fmt.Sprintf("statfs failed: %v", err)
	}
	if stat.Type != unix.CGROUP2_SUPER_MAGIC {
		return false, fmt.Sprintf("unexpected filesystem type: 0x%x", stat.Type)
	}
	return true, "/sys/fs/cgroup is cgroup2"
}

func checkOverlayFS() (bool, string) {
	data, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		return false, fmt.Sprintf("read /proc/filesystems: %v", err)
	}
	if strings.Contains(string(data), "overlay") {
		return true, "overlay filesystem available"
	}
	return false, "overlay filesystem not listed"
}

func checkDataDir(dataDir string) (bool, string, string) {
	isWSL := detectWSL()
	if isWSL && strings.HasPrefix(filepath.Clean(dataDir), "/mnt/") {
		return false, "FAIL", "data dir is on /mnt (NTFS); use ext4 path like /var/lib/sandkasten"
	}

	checkPath := nearestExistingDir(dataDir)
	var stat unix.Statfs_t
	if err := unix.Statfs(checkPath, &stat); err != nil {
		return false, "WARN", fmt.Sprintf("cannot statfs %s: %v", checkPath, err)
	}

	if stat.Type == 0x5346544e {
		return false, "FAIL", fmt.Sprintf("%s is NTFS; overlayfs does not work reliably", checkPath)
	}

	if _, err := os.Stat(dataDir); errors.Is(err, fs.ErrNotExist) {
		return true, "WARN", fmt.Sprintf("%s does not exist yet (will be created by init)", dataDir)
	}

	return true, "OK", fmt.Sprintf("%s looks usable", dataDir)
}

func checkRunnerBinary() (bool, string) {
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Sprintf("cannot determine executable path: %v", err)
	}
	runnerPath := filepath.Join(filepath.Dir(exePath), "runner")
	if _, err := os.Stat(runnerPath); err != nil {
		return false, fmt.Sprintf("runner missing at %s", runnerPath)
	}
	return true, runnerPath
}

func charsToString(chars []byte) string {
	var out strings.Builder
	for _, c := range chars {
		if c == 0 {
			break
		}
		out.WriteByte(byte(c))
	}
	return out.String()
}

func parseKernelVersion(release string) (int, int) {
	parts := strings.SplitN(release, "-", 2)
	core := parts[0]
	bits := strings.Split(core, ".")
	if len(bits) < 2 {
		return 0, 0
	}
	major, _ := strconv.Atoi(bits[0])
	minor, _ := strconv.Atoi(bits[1])
	return major, minor
}

func detectWSL() bool {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	release := strings.ToLower(string(data))
	return strings.Contains(release, "microsoft")
}

func nearestExistingDir(pathValue string) string {
	current := filepath.Clean(pathValue)
	for {
		if info, err := os.Stat(current); err == nil && info.IsDir() {
			return current
		}
		next := filepath.Dir(current)
		if next == current {
			return "/"
		}
		current = next
	}
}

func writeInitialConfig(configPath, listen, apiKey, dataDir, defaultImage string, force bool) error {
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config %s already exists (use --force to overwrite)", configPath)
		}
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	cfg := initConfig{
		Listen:       listen,
		APIKey:       apiKey,
		DataDir:      dataDir,
		DefaultImage: defaultImage,
		AllowedImage: []string{defaultImage},
		DBPath:       filepath.Join(dataDir, "sandkasten.db"),
		Defaults: initConfigDefaults{
			CPULimit:         1.0,
			MemLimitMB:       512,
			PidsLimit:        256,
			MaxExecTimeoutMs: 120000,
			NetworkMode:      "none",
			ReadonlyRootfs:   true,
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func generateAPIKey() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(raw), nil
}

func imageExists(dataDir, image string) bool {
	_, err := os.Stat(filepath.Join(dataDir, "images", image, "rootfs"))
	return err == nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
