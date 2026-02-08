package session

import (
	"context"
	"fmt"
	"testing"

	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateFromPool(t *testing.T) {
	mgr, dc, st, pool, _ := newTestManager()

	pool.On("Get", mock.Anything, "sandbox-runtime:base").Return("pool-container-id", true)
	st.On("CreateSession", mock.AnythingOfType("*store.Session")).Return(nil)

	info, err := mgr.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "sandbox-runtime:base", info.Image)
	assert.Equal(t, "running", info.Status)
	assert.Equal(t, "/workspace", info.Cwd)

	pool.AssertExpectations(t)
	st.AssertExpectations(t)
	dc.AssertNotCalled(t, "CreateContainer")
}

func TestCreateFromDocker(t *testing.T) {
	mgr, dc, st, pool, _ := newTestManager()

	pool.On("Get", mock.Anything, "sandbox-runtime:base").Return("", false)
	dc.On("CreateContainer", mock.Anything, mock.AnythingOfType("docker.CreateOpts")).Return("new-container-id", nil)
	st.On("CreateSession", mock.AnythingOfType("*store.Session")).Return(nil)

	info, err := mgr.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "sandbox-runtime:base", info.Image)
	dc.AssertExpectations(t)
}

func TestCreateInvalidImage(t *testing.T) {
	mgr, _, _, _, _ := newTestManager()

	_, err := mgr.Create(context.Background(), CreateOpts{Image: "evil-image"})
	assert.ErrorIs(t, err, ErrInvalidImage)
}

func TestCreateDockerFailure(t *testing.T) {
	mgr, dc, _, pool, _ := newTestManager()

	pool.On("Get", mock.Anything, "sandbox-runtime:base").Return("", false)
	dc.On("CreateContainer", mock.Anything, mock.AnythingOfType("docker.CreateOpts")).Return("", fmt.Errorf("docker error"))

	_, err := mgr.Create(context.Background(), CreateOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create container")
}

func TestCreateWithWorkspace(t *testing.T) {
	mgr, dc, st, pool, ws := newTestManager()
	mgr.cfg.Workspace.Enabled = true

	ws.On("Exists", mock.Anything, "sandkasten-ws-my-ws").Return(false, nil)
	ws.On("Create", mock.Anything, "sandkasten-ws-my-ws", mock.Anything).Return(nil)
	pool.On("Get", mock.Anything, "sandbox-runtime:base").Return("", false)
	dc.On("CreateContainer", mock.Anything, mock.MatchedBy(func(opts docker.CreateOpts) bool {
		return opts.WorkspaceID == "my-ws"
	})).Return("ws-container", nil)
	st.On("CreateSession", mock.MatchedBy(func(s *store.Session) bool {
		return s.WorkspaceID == "my-ws"
	})).Return(nil)

	info, err := mgr.Create(context.Background(), CreateOpts{WorkspaceID: "my-ws"})
	require.NoError(t, err)
	assert.Equal(t, "my-ws", info.WorkspaceID)

	ws.AssertExpectations(t)
}

func TestCreateStoreFailureRemovesContainer(t *testing.T) {
	mgr, dc, st, pool, _ := newTestManager()

	pool.On("Get", mock.Anything, "sandbox-runtime:base").Return("", false)
	dc.On("CreateContainer", mock.Anything, mock.AnythingOfType("docker.CreateOpts")).Return("container-to-cleanup", nil)
	st.On("CreateSession", mock.AnythingOfType("*store.Session")).Return(fmt.Errorf("db error"))
	dc.On("RemoveContainer", mock.Anything, "container-to-cleanup", mock.Anything).Return(nil)

	_, err := mgr.Create(context.Background(), CreateOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store session")

	dc.AssertCalled(t, "RemoveContainer", mock.Anything, "container-to-cleanup", mock.Anything)
}
