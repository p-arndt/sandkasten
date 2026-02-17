package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/p-arndt/sandkasten/protocol"
)

type WorkspaceInfo struct {
	ID string `json:"id"`
}

func (m *Manager) ListWorkspaces(ctx context.Context) ([]*WorkspaceInfo, error) {
	if !m.cfg.Workspace.Enabled {
		return nil, fmt.Errorf("workspaces not enabled")
	}

	workspaceDir := filepath.Join(m.cfg.DataDir, "workspaces")
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workspaces dir: %w", err)
	}

	var result []*WorkspaceInfo
	for _, entry := range entries {
		if entry.IsDir() {
			result = append(result, &WorkspaceInfo{
				ID: entry.Name(),
			})
		}
	}

	return result, nil
}

func (m *Manager) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	if !m.cfg.Workspace.Enabled {
		return fmt.Errorf("workspaces not enabled")
	}

	shortID := strings.TrimPrefix(workspaceID, protocol.WorkspaceVolumePrefix)
	workspacePath := filepath.Join(m.cfg.DataDir, "workspaces", shortID)

	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	return nil
}
