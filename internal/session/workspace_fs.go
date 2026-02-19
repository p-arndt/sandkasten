package session

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/p-arndt/sandkasten/protocol"
)

type WorkspaceFileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

func (m *Manager) ListWorkspaceFiles(ctx context.Context, workspaceID, dirPath string) ([]WorkspaceFileEntry, error) {
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

	entries, err := os.ReadDir(fullPath)
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
	rel, err := filepath.Rel(workspacePath, realPath)
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
