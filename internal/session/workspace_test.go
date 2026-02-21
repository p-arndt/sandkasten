package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/p-arndt/sandkasten/internal/config"
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

func TestWriteWorkspaceFile_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir}
	mgr := NewManager(cfg, nil, nil, nil, nil)

	ctx := context.Background()

	err := mgr.WriteWorkspaceFile(ctx, "test-ws", "code.py", []byte("print(42)"), false)
	require.NoError(t, err)

	content, truncated, err := mgr.ReadWorkspaceFile(ctx, "test-ws", "code.py", 0)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Equal(t, "cHJpbnQoNDIp", content) // base64 of "print(42)"

	// Verify file on disk
	fullPath := filepath.Join(dir, "workspaces", "test-ws", "code.py")
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, "print(42)", string(data))
}

func TestWriteWorkspaceFile_Base64(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir}
	mgr := NewManager(cfg, nil, nil, nil, nil)

	err := mgr.WriteWorkspaceFile(context.Background(), "ws2", "binary.dat", []byte("aGVsbG8="), true)
	require.NoError(t, err)

	fullPath := filepath.Join(dir, "workspaces", "ws2", "binary.dat")
	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestWriteWorkspaceFile_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir}
	mgr := NewManager(cfg, nil, nil, nil, nil)

	err := mgr.WriteWorkspaceFile(context.Background(), "ws", "../etc/passwd", []byte("x"), false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file path")
}
