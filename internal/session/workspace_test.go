package session

import (
	"context"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListWorkspacesEnabled(t *testing.T) {
	mgr, _, _, _, ws := newTestManager()
	mgr.cfg.Workspace.Enabled = true

	expected := []*workspace.Workspace{
		{ID: "ws-1", CreatedAt: time.Now()},
		{ID: "ws-2", CreatedAt: time.Now()},
	}
	ws.On("List", mock.Anything).Return(expected, nil)

	result, err := mgr.ListWorkspaces(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListWorkspacesDisabled(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()
	mgr.cfg.Workspace.Enabled = false

	_, err := mgr.ListWorkspaces(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestDeleteWorkspaceEnabled(t *testing.T) {
	mgr, _, _, _, ws := newTestManager()
	mgr.cfg.Workspace.Enabled = true

	ws.On("Delete", mock.Anything, "sandkasten-ws-my-ws").Return(nil)

	err := mgr.DeleteWorkspace(context.Background(), "my-ws")
	require.NoError(t, err)
	ws.AssertCalled(t, "Delete", mock.Anything, "sandkasten-ws-my-ws")
}

func TestDeleteWorkspaceDisabled(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()
	mgr.cfg.Workspace.Enabled = false

	err := mgr.DeleteWorkspace(context.Background(), "my-ws")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}
