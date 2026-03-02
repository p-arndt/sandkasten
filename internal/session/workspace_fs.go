package session

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/p-arndt/sandkasten/protocol"
	"golang.org/x/sys/unix"
)

type WorkspaceFileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

func (m *Manager) ListWorkspaceFiles(ctx context.Context, workspaceID, dirPath string) ([]WorkspaceFileEntry, error) {
	if !m.cfg.Workspace.Enabled {
		return nil, fmt.Errorf("workspaces not enabled")
	}

	shortID := m.normalizeWorkspaceID(workspaceID)
	if shortID == "" {
		return nil, fmt.Errorf("invalid workspace id")
	}

	workspacePath := filepath.Join(m.cfg.DataDir, "workspaces", shortID)
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace not found: %s", shortID)
	}

	safePath := m.safeWorkspacePath(dirPath)
	fullPath := filepath.Join(workspacePath, safePath)

	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	realWorkspacePath, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	rel, err := filepath.Rel(realWorkspacePath, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("path escapes workspace")
	}

	entries, err := os.ReadDir(realPath)
	if err != nil {
		return nil, fmt.Errorf("read workspace dir: %w", err)
	}

	var result []WorkspaceFileEntry
	for _, entry := range entries {
		result = append(result, WorkspaceFileEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}

	return result, nil
}

func (m *Manager) ReadWorkspaceFile(ctx context.Context, workspaceID, filePath string, maxBytes int) (contentBase64 string, truncated bool, err error) {
	if !m.cfg.Workspace.Enabled {
		return "", false, fmt.Errorf("workspaces not enabled")
	}

	shortID := m.normalizeWorkspaceID(workspaceID)
	if shortID == "" {
		return "", false, fmt.Errorf("invalid workspace id")
	}

	safePath := m.safeWorkspacePath(filePath)
	if safePath == "" {
		return "", false, fmt.Errorf("invalid file path")
	}

	workspacePath := filepath.Join(m.cfg.DataDir, "workspaces", shortID)
	fullPath := filepath.Join(workspacePath, safePath)

	// Resolve symlinks and ensure the resolved path stays inside the workspace (prevents symlink escape).
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return "", false, fmt.Errorf("resolve path: %w", err)
	}

	realWorkspacePath, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return "", false, fmt.Errorf("resolve workspace path: %w", err)
	}

	rel, err := filepath.Rel(realWorkspacePath, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false, fmt.Errorf("path escapes workspace")
	}

	info, err := os.Stat(realPath)
	if err != nil {
		return "", false, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("path is a directory")
	}

	if maxBytes <= 0 {
		maxBytes = protocol.DefaultMaxReadBytes
	}

	file, err := os.Open(realPath)
	if err != nil {
		return "", false, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	limitedReader := io.LimitReader(file, int64(maxBytes)+1)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", false, fmt.Errorf("read file: %w", err)
	}

	truncated = len(content) > maxBytes
	if truncated {
		content = content[:maxBytes]
	}

	return base64.StdEncoding.EncodeToString(content), truncated, nil
}

func (m *Manager) WriteWorkspaceFile(ctx context.Context, workspaceID, filePath string, content []byte, isBase64 bool) error {
	if !m.cfg.Workspace.Enabled {
		return fmt.Errorf("workspaces not enabled")
	}

	shortID := m.normalizeWorkspaceID(workspaceID)
	if shortID == "" {
		return fmt.Errorf("invalid workspace id")
	}

	safePath := m.safeWorkspacePath(filePath)
	if safePath == "" {
		return fmt.Errorf("invalid file path")
	}

	workspacePath := filepath.Join(m.cfg.DataDir, "workspaces", shortID)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}

	fullPath := filepath.Join(workspacePath, safePath)
	fullPath = filepath.Clean(fullPath)
	rel, relErr := filepath.Rel(workspacePath, fullPath)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes workspace")
	}

	realWorkspacePath, err := filepath.EvalSymlinks(workspacePath)
	if err != nil {
		return fmt.Errorf("resolve workspace path: %w", err)
	}

	var data []byte
	if isBase64 {
		var decodeErr error
		data, decodeErr = base64.StdEncoding.DecodeString(string(content))
		if decodeErr != nil {
			return fmt.Errorf("invalid base64 content: %w", decodeErr)
		}
	} else {
		data = content
	}

	if err := writeWorkspaceFileNoSymlinkTraversal(realWorkspacePath, safePath, data); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func writeWorkspaceFileNoSymlinkTraversal(rootPath, relPath string, data []byte) error {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) == 0 {
		return fmt.Errorf("invalid file path")
	}

	fileName := parts[len(parts)-1]
	if fileName == "" || fileName == "." || fileName == ".." {
		return fmt.Errorf("invalid file path")
	}

	rootFD, err := unix.Open(rootPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}
	defer unix.Close(rootFD)

	currentFD := rootFD
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid file path")
		}

		nextFD, openErr := unix.Openat(currentFD, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			if errors.Is(openErr, unix.ENOENT) {
				if mkErr := unix.Mkdirat(currentFD, part, 0755); mkErr != nil && !errors.Is(mkErr, unix.EEXIST) {
					return fmt.Errorf("create directory %q: %w", part, mkErr)
				}
				nextFD, openErr = unix.Openat(currentFD, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			}
			if openErr != nil {
				if errors.Is(openErr, unix.ELOOP) {
					return fmt.Errorf("path escapes workspace")
				}
				return fmt.Errorf("open directory %q: %w", part, openErr)
			}
		}

		if currentFD != rootFD {
			_ = unix.Close(currentFD)
		}
		currentFD = nextFD
	}

	defer func() {
		if currentFD != rootFD {
			_ = unix.Close(currentFD)
		}
	}()

	fileFD, err := unix.Openat(currentFD, fileName, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0644)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return fmt.Errorf("path escapes workspace")
		}
		return err
	}
	defer unix.Close(fileFD)

	for written := 0; written < len(data); {
		n, writeErr := unix.Write(fileFD, data[written:])
		if writeErr != nil {
			return writeErr
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		written += n
	}

	return nil
}

func (m *Manager) normalizeWorkspaceID(workspaceID string) string {
	short := strings.TrimPrefix(workspaceID, protocol.WorkspaceVolumePrefix)
	if short != "" {
		return short
	}
	return workspaceID
}

func (m *Manager) safeWorkspacePath(raw string) string {
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if strings.Contains(cleaned, "..") {
		return ""
	}
	return cleaned
}
