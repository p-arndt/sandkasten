package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWorkspacesEnabled(t *testing.T) {
	mgr, _, _ := newTestManager()
	mgr.cfg.Workspace.Enabled = true

	result, err := mgr.ListWorkspaces(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestListWorkspacesDisabled(t *testing.T) {
	mgr, _, _ := newTestManager()
	mgr.cfg.Workspace.Enabled = false

	_, err := mgr.ListWorkspaces(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestDeleteWorkspaceEnabled(t *testing.T) {
	mgr, _, _ := newTestManager()
	mgr.cfg.Workspace.Enabled = true

	err := mgr.DeleteWorkspace(context.Background(), "my-ws")
	require.NoError(t, err)
}

func TestDeleteWorkspaceDisabled(t *testing.T) {
	mgr, _, _ := newTestManager()
	mgr.cfg.Workspace.Enabled = false

	err := mgr.DeleteWorkspace(context.Background(), "my-ws")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}
