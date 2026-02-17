package session

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/protocol"
)

// WorkspaceFileEntry describes a single file or directory in a workspace.
type WorkspaceFileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

// ListWorkspaceFiles lists entries in a workspace directory.
func (m *Manager) ListWorkspaceFiles(ctx context.Context, workspaceID, dirPath string) ([]WorkspaceFileEntry, error) {
	shortID := m.normalizeWorkspaceID(workspaceID)
	if shortID == "" {
		return nil, fmt.Errorf("invalid workspace id")
	}
	safePath := m.safeWorkspacePath(dirPath)
	listPath := "/workspace"
	if safePath != "" {
		listPath = "/workspace/" + safePath
	}

	containerID, err := m.createWorkspaceBrowseContainer(ctx, shortID)
	if err != nil {
		return nil, err
	}
	defer m.docker.RemoveContainer(ctx, containerID, "_wsfs_"+shortID)

	// Give the runner server inside the container time to start (shell + socket).
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	req := protocol.Request{
		ID:   uuid.New().String()[:8],
		Type: protocol.RequestExec,
		Cmd:  "ls -la " + listPath,
	}
	resp, err := m.docker.ExecRunner(ctx, containerID, req)
	if err != nil {
		return nil, fmt.Errorf("list workspace files: %w", err)
	}
	if resp.Type == protocol.ResponseError {
		return nil, fmt.Errorf("list workspace files: %s", resp.Error)
	}
	return parseLSOutput(resp.Output), nil
}

// ReadWorkspaceFile reads file content from a workspace.
func (m *Manager) ReadWorkspaceFile(ctx context.Context, workspaceID, filePath string, maxBytes int) (contentBase64 string, truncated bool, err error) {
	shortID := m.normalizeWorkspaceID(workspaceID)
	if shortID == "" {
		return "", false, fmt.Errorf("invalid workspace id")
	}
	safePath := m.safeWorkspacePath(filePath)
	if safePath == "" {
		return "", false, fmt.Errorf("invalid file path")
	}
	readPath := "/workspace/" + safePath

	containerID, err := m.createWorkspaceBrowseContainer(ctx, shortID)
	if err != nil {
		return "", false, err
	}
	defer m.docker.RemoveContainer(ctx, containerID, "_wsfs_"+shortID)

	// Give the runner server inside the container time to start (shell + socket).
	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	req := protocol.Request{
		ID:       uuid.New().String()[:8],
		Type:     protocol.RequestRead,
		Path:     readPath,
		MaxBytes: maxBytes,
	}
	if maxBytes <= 0 {
		req.MaxBytes = protocol.DefaultMaxReadBytes
	}
	resp, err := m.docker.ExecRunner(ctx, containerID, req)
	if err != nil {
		return "", false, fmt.Errorf("read workspace file: %w", err)
	}
	if resp.Type == protocol.ResponseError {
		return "", false, fmt.Errorf("read workspace file: %s", resp.Error)
	}
	return resp.ContentBase64, resp.Truncated, nil
}

func (m *Manager) normalizeWorkspaceID(workspaceID string) string {
	short := strings.TrimPrefix(workspaceID, protocol.WorkspaceVolumePrefix)
	if short != "" {
		return short
	}
	return workspaceID
}

// safeWorkspacePath returns a path safe to join with /workspace/ (no .., no leading /).
func (m *Manager) safeWorkspacePath(raw string) string {
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	if strings.HasPrefix(cleaned, "/") {
		cleaned = strings.TrimPrefix(cleaned, "/")
	}
	if strings.Contains(cleaned, "..") {
		return ""
	}
	return cleaned
}

func (m *Manager) createWorkspaceBrowseContainer(ctx context.Context, shortWorkspaceID string) (string, error) {
	image := m.resolveImage("")
	defaults := m.cfg.Defaults
	opts := docker.CreateOpts{
		SessionID:   "_wsfs_" + shortWorkspaceID + "-" + uuid.New().String()[:8],
		Image:       image,
		WorkspaceID: shortWorkspaceID,
		Defaults:    defaults,
	}
	containerID, err := m.docker.CreateContainer(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("create workspace browse container: %w", err)
	}
	return containerID, nil
}

// parseLSOutput parses "ls -la" output into file entries. Skips ".", "..", and "total".
func parseLSOutput(output string) []WorkspaceFileEntry {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var entries []WorkspaceFileEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 9 {
			continue
		}
		perms := parts[0]
		if len(perms) == 0 {
			continue
		}
		isDir := perms[0] == 'd'
		name := strings.Join(parts[8:], " ")
		if name == "." || name == ".." {
			continue
		}
		entries = append(entries, WorkspaceFileEntry{Name: name, IsDir: isDir})
	}
	return entries
}
