package session

import (
	"context"
	"fmt"

	"github.com/p-arndt/sandkasten/internal/workspace"
)

func (m *Manager) ListWorkspaces(ctx context.Context) ([]*workspace.Workspace, error) {
	if !m.cfg.Workspace.Enabled {
		return nil, fmt.Errorf("workspaces not enabled")
	}
	return m.workspace.List(ctx)
}

func (m *Manager) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	if !m.cfg.Workspace.Enabled {
		return fmt.Errorf("workspaces not enabled")
	}
	return m.workspace.Delete(ctx, "sandkasten-ws-"+workspaceID)
}
