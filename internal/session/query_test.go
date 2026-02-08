package session

import (
	"context"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetSuccess(t *testing.T) {
	mgr, _, st, _, _ := newTestManager()
	now := time.Now().UTC()
	sess := &store.Session{
		ID:        "s1",
		Image:     "sandbox-runtime:base",
		Status:    "running",
		Cwd:       "/workspace",
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	st.On("GetSession", "s1").Return(sess, nil)

	info, err := mgr.Get(context.Background(), "s1")
	require.NoError(t, err)

	assert.Equal(t, "s1", info.ID)
	assert.Equal(t, "sandbox-runtime:base", info.Image)
	assert.Equal(t, "running", info.Status)
}

func TestGetNotFound(t *testing.T) {
	mgr, _, st, _, _ := newTestManager()

	st.On("GetSession", "nonexistent").Return(nil, nil)

	_, err := mgr.Get(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListSuccess(t *testing.T) {
	mgr, _, st, _, _ := newTestManager()
	now := time.Now().UTC()

	st.On("ListSessions").Return([]*store.Session{
		{ID: "s1", Image: "base", Status: "running", Cwd: "/workspace", CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)},
		{ID: "s2", Image: "python", Status: "destroyed", Cwd: "/home", CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)},
	}, nil)

	sessions, err := mgr.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.Equal(t, "s1", sessions[0].ID)
	assert.Equal(t, "s2", sessions[1].ID)
}

func TestListEmpty(t *testing.T) {
	mgr, _, st, _, _ := newTestManager()

	st.On("ListSessions").Return([]*store.Session{}, nil)

	sessions, err := mgr.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestDestroySuccess(t *testing.T) {
	mgr, dc, st, _, _ := newTestManager()
	sess := &store.Session{
		ID:          "s1",
		ContainerID: "container-s1",
		Status:      "running",
	}

	st.On("GetSession", "s1").Return(sess, nil)
	dc.On("RemoveContainer", mock.Anything, "container-s1", "s1").Return(nil)
	st.On("UpdateSessionStatus", "s1", "destroyed").Return(nil)

	err := mgr.Destroy(context.Background(), "s1")
	require.NoError(t, err)

	dc.AssertCalled(t, "RemoveContainer", mock.Anything, "container-s1", "s1")
	st.AssertCalled(t, "UpdateSessionStatus", "s1", "destroyed")
}

func TestDestroyNotFound(t *testing.T) {
	mgr, _, st, _, _ := newTestManager()

	st.On("GetSession", "nonexistent").Return(nil, nil)

	err := mgr.Destroy(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDestroyRemovesLock(t *testing.T) {
	mgr, dc, st, _, _ := newTestManager()
	sess := &store.Session{
		ID:          "s1",
		ContainerID: "container-s1",
		Status:      "running",
	}

	// Create a lock first
	_ = mgr.sessionLock("s1")
	assert.Len(t, mgr.locks, 1)

	st.On("GetSession", "s1").Return(sess, nil)
	dc.On("RemoveContainer", mock.Anything, "container-s1", "s1").Return(nil)
	st.On("UpdateSessionStatus", "s1", "destroyed").Return(nil)

	err := mgr.Destroy(context.Background(), "s1")
	require.NoError(t, err)

	// Lock should be cleaned up
	assert.Len(t, mgr.locks, 0)
}
