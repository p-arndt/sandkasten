package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/runtime"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateSuccess(t *testing.T) {
	mgr, rt, st := newTestManager()

	rt.On("Create", mock.Anything, mock.AnythingOfType("runtime.CreateOpts")).Return(&runtime.SessionInfo{
		SessionID:  "test-session",
		InitPID:    12345,
		CgroupPath: "/sys/fs/cgroup/sandkasten/test-session",
	}, nil)
	st.On("CreateSession", mock.AnythingOfType("*store.Session")).Return(nil)

	info, err := mgr.Create(context.Background(), CreateOpts{})
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "base", info.Image)
	assert.Equal(t, "running", info.Status)
	assert.Equal(t, "/workspace", info.Cwd)
	assert.Equal(t, "cold", info.AcquireSource)

	rt.AssertExpectations(t)
	st.AssertExpectations(t)
}

func TestCreateInvalidImage(t *testing.T) {
	mgr, _, _ := newTestManager()

	_, err := mgr.Create(context.Background(), CreateOpts{Image: "evil-image"})
	assert.ErrorIs(t, err, ErrInvalidImage)
}

func TestCreateRuntimeFailure(t *testing.T) {
	mgr, rt, _ := newTestManager()

	rt.On("Create", mock.Anything, mock.AnythingOfType("runtime.CreateOpts")).Return(nil, fmt.Errorf("runtime error"))

	_, err := mgr.Create(context.Background(), CreateOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create sandbox")
}

func TestCreateStoreFailureDestroysSession(t *testing.T) {
	mgr, rt, st := newTestManager()

	rt.On("Create", mock.Anything, mock.AnythingOfType("runtime.CreateOpts")).Return(&runtime.SessionInfo{
		SessionID:  "test-session",
		InitPID:    12345,
		CgroupPath: "/sys/fs/cgroup/sandkasten/test-session",
	}, nil)
	st.On("CreateSession", mock.AnythingOfType("*store.Session")).Return(fmt.Errorf("db error"))
	rt.On("Destroy", mock.Anything, mock.Anything).Return(nil)

	_, err := mgr.Create(context.Background(), CreateOpts{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store session")

	rt.AssertCalled(t, "Destroy", mock.Anything, mock.Anything)
}

func TestCreate_PoolHit_NoWorkspace(t *testing.T) {
	rt := &MockRuntimeDriver{}
	st := &MockSessionStore{}
	pl := &MockContainerPool{}
	cfg := testConfig()
	mgr := NewManager(cfg, st, rt, nil, pl)

	pooledSess := &store.Session{
		ID: "pool-123", Image: "python", InitPID: 1, CgroupPath: "/cgroup/pool-123",
		Status: store.StatusPoolIdle, Cwd: "/workspace", WorkspaceID: "",
		CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(24 * time.Hour), LastActivity: time.Now().UTC(),
	}

	pl.On("Get", mock.Anything, "python", "").Return("pool-123", true)
	st.On("GetSession", "pool-123").Return(pooledSess, nil)
	st.On("UpdateSessionStatus", "pool-123", "running").Return(nil)
	st.On("UpdateSessionActivity", "pool-123", "/workspace", mock.AnythingOfType("time.Time")).Return(nil)
	pl.On("Refill", mock.Anything, "python", "", 0).Maybe().Return(nil) // runs in goroutine

	info, err := mgr.Create(context.Background(), CreateOpts{Image: "python"})
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "pool-123", info.ID)
	assert.Equal(t, "python", info.Image)
	assert.Equal(t, "running", info.Status)
	assert.Equal(t, "pool", info.AcquireSource)

	rt.AssertNotCalled(t, "Create")
	pl.AssertNumberOfCalls(t, "Get", 1)
	st.AssertExpectations(t)
}

func TestCreate_PoolHit_WithWorkspace(t *testing.T) {
	rt := &MockRuntimeDriver{}
	st := &MockSessionStore{}
	pl := &MockContainerPool{}
	cfg := testConfig()
	cfg.Workspace.Enabled = true
	ws := &MockWorkspaceManager{}
	ws.On("Exists", mock.Anything, "my-ws").Return(true, nil)
	mgr := NewManager(cfg, st, rt, ws, pl)

	pooledSess := &store.Session{
		ID: "pool-456", Image: "python", InitPID: 1, CgroupPath: "/cgroup/pool-456",
		Status: store.StatusPoolIdle, Cwd: "/workspace", WorkspaceID: "",
		CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(24 * time.Hour), LastActivity: time.Now().UTC(),
	}

	pl.On("Get", mock.Anything, "python", "my-ws").Return("pool-456", true)
	st.On("GetSession", "pool-456").Return(pooledSess, nil)
	rt.On("MountWorkspace", mock.Anything, "pool-456", "my-ws").Return(nil)
	st.On("UpdateSessionWorkspace", "pool-456", "my-ws").Return(nil)
	st.On("UpdateSessionStatus", "pool-456", "running").Return(nil)
	st.On("UpdateSessionActivity", "pool-456", "/workspace", mock.AnythingOfType("time.Time")).Return(nil)
	pl.On("Refill", mock.Anything, "python", "my-ws", 1).Maybe().Return(nil) // runs in goroutine

	info, err := mgr.Create(context.Background(), CreateOpts{Image: "python", WorkspaceID: "my-ws"})
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "pool-456", info.ID)
	assert.Equal(t, "my-ws", info.WorkspaceID)
	assert.Equal(t, "pool", info.AcquireSource)

	rt.AssertNotCalled(t, "Create")
	rt.AssertCalled(t, "MountWorkspace", mock.Anything, "pool-456", "my-ws")
	pl.AssertNumberOfCalls(t, "Get", 1)
	st.AssertExpectations(t)
}

func TestCreate_PoolHit_MountWorkspaceFails_FallsThroughToNormalCreate(t *testing.T) {
	rt := &MockRuntimeDriver{}
	st := &MockSessionStore{}
	pl := &MockContainerPool{}
	cfg := testConfig()
	cfg.Workspace.Enabled = true
	ws := &MockWorkspaceManager{}
	ws.On("Exists", mock.Anything, "my-ws").Return(true, nil)
	mgr := NewManager(cfg, st, rt, ws, pl)

	pooledSess := &store.Session{
		ID: "pool-789", Image: "python", InitPID: 1, CgroupPath: "/cgroup/pool-789",
		Status: store.StatusPoolIdle, Cwd: "/workspace", WorkspaceID: "",
		CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(24 * time.Hour), LastActivity: time.Now().UTC(),
	}

	pl.On("Get", mock.Anything, "python", "my-ws").Return("pool-789", true)
	st.On("GetSession", "pool-789").Return(pooledSess, nil)
	rt.On("MountWorkspace", mock.Anything, "pool-789", "my-ws").Return(fmt.Errorf("mount failed"))
	st.On("UpdateSessionStatus", "pool-789", "destroyed").Maybe().Return(nil)
	rt.On("Destroy", mock.Anything, "pool-789").Return(nil)
	rt.On("Create", mock.Anything, mock.MatchedBy(func(opts runtime.CreateOpts) bool {
		return opts.Image == "python" && opts.WorkspaceID == "my-ws"
	})).Return(&runtime.SessionInfo{
		SessionID: "new-session", InitPID: 999, CgroupPath: "/cgroup/new-session",
	}, nil)
	st.On("CreateSession", mock.AnythingOfType("*store.Session")).Return(nil)
	pl.On("Refill", mock.Anything, "python", "my-ws", 1).Maybe().Return(nil)

	info, err := mgr.Create(context.Background(), CreateOpts{Image: "python", WorkspaceID: "my-ws"})
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.NotEmpty(t, info.ID)
	assert.Equal(t, "python", info.Image)
	assert.Equal(t, "my-ws", info.WorkspaceID)

	rt.AssertCalled(t, "Destroy", mock.Anything, "pool-789")
	rt.AssertCalled(t, "Create", mock.Anything, mock.MatchedBy(func(opts runtime.CreateOpts) bool {
		return opts.WorkspaceID == "my-ws"
	}))
}
