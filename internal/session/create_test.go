package session

import (
	"context"
	"fmt"
	"testing"

	"github.com/p-arndt/sandkasten/internal/runtime"
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
