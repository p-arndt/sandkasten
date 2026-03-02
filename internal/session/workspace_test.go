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
	cfg := &config.Config{DataDir: dir, Workspace: config.WorkspaceConfig{Enabled: true}}
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
	cfg := &config.Config{DataDir: dir, Workspace: config.WorkspaceConfig{Enabled: true}}
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
	cfg := &config.Config{DataDir: dir, Workspace: config.WorkspaceConfig{Enabled: true}}
	mgr := NewManager(cfg, nil, nil, nil, nil)

	err := mgr.WriteWorkspaceFile(context.Background(), "ws", "../etc/passwd", []byte("x"), false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file path")
}

func TestWriteWorkspaceFile_RejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, Workspace: config.WorkspaceConfig{Enabled: true}}
	mgr := NewManager(cfg, nil, nil, nil, nil)

	workspaceDir := filepath.Join(dir, "workspaces", "ws3")
	require.NoError(t, os.MkdirAll(workspaceDir, 0755))

	hostDir := filepath.Join(dir, "host-target")
	require.NoError(t, os.MkdirAll(hostDir, 0755))

	linkPath := filepath.Join(workspaceDir, "escape")
	require.NoError(t, os.Symlink(hostDir, linkPath))

	err := mgr.WriteWorkspaceFile(context.Background(), "ws3", "escape/pwned.txt", []byte("owned"), false)
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(hostDir, "pwned.txt"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestWriteWorkspaceFile_RejectsSymlinkLeaf(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, Workspace: config.WorkspaceConfig{Enabled: true}}
	mgr := NewManager(cfg, nil, nil, nil, nil)

	workspaceDir := filepath.Join(dir, "workspaces", "ws4")
	require.NoError(t, os.MkdirAll(workspaceDir, 0755))

	hostFile := filepath.Join(dir, "host-secret.txt")
	require.NoError(t, os.WriteFile(hostFile, []byte("before"), 0644))

	linkPath := filepath.Join(workspaceDir, "leak.txt")
	require.NoError(t, os.Symlink(hostFile, linkPath))

	err := mgr.WriteWorkspaceFile(context.Background(), "ws4", "leak.txt", []byte("after"), false)
	require.Error(t, err)

	data, readErr := os.ReadFile(hostFile)
	require.NoError(t, readErr)
	assert.Equal(t, "before", string(data))
}

func TestWorkspaceFSDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, Workspace: config.WorkspaceConfig{Enabled: false}}
	mgr := NewManager(cfg, nil, nil, nil, nil)

	err := mgr.WriteWorkspaceFile(context.Background(), "ws", "a.txt", []byte("x"), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspaces not enabled")

	_, _, err = mgr.ReadWorkspaceFile(context.Background(), "ws", "a.txt", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspaces not enabled")

	_, err = mgr.ListWorkspaceFiles(context.Background(), "ws", ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspaces not enabled")
}
