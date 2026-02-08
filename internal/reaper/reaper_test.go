package reaper

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestReapExpired_NoExpired(t *testing.T) {
	st := &MockReaperStore{}
	dc := &MockReaperDocker{}
	r := New(st, dc, time.Minute, testLogger())

	st.On("ListExpiredSessions").Return([]*store.Session{}, nil)

	r.reapExpired(context.Background())

	st.AssertExpectations(t)
	dc.AssertNotCalled(t, "RemoveContainer")
}

func TestReapExpired_WithExpired(t *testing.T) {
	st := &MockReaperStore{}
	dc := &MockReaperDocker{}
	sm := &MockSessionManager{}
	r := New(st, dc, time.Minute, testLogger())
	r.SetSessionManager(sm)

	expired := []*store.Session{
		{ID: "s1", ContainerID: "c1", ExpiresAt: time.Now().Add(-time.Minute)},
		{ID: "s2", ContainerID: "c2", ExpiresAt: time.Now().Add(-2 * time.Minute)},
	}

	st.On("ListExpiredSessions").Return(expired, nil)
	dc.On("RemoveContainer", mock.Anything, "c1", "s1").Return(nil)
	dc.On("RemoveContainer", mock.Anything, "c2", "s2").Return(nil)
	st.On("UpdateSessionStatus", "s1", "expired").Return(nil)
	st.On("UpdateSessionStatus", "s2", "expired").Return(nil)
	sm.On("CleanupSessionLock", "s1").Return()
	sm.On("CleanupSessionLock", "s2").Return()

	r.reapExpired(context.Background())

	st.AssertExpectations(t)
	dc.AssertExpectations(t)
	sm.AssertExpectations(t)
}

func TestReapExpired_NoSessionManager(t *testing.T) {
	st := &MockReaperStore{}
	dc := &MockReaperDocker{}
	r := New(st, dc, time.Minute, testLogger())
	// No session manager set

	expired := []*store.Session{
		{ID: "s1", ContainerID: "c1"},
	}

	st.On("ListExpiredSessions").Return(expired, nil)
	dc.On("RemoveContainer", mock.Anything, "c1", "s1").Return(nil)
	st.On("UpdateSessionStatus", "s1", "expired").Return(nil)

	// Should not panic even without session manager
	require.NotPanics(t, func() {
		r.reapExpired(context.Background())
	})
}

func TestReconcile_MissingContainer(t *testing.T) {
	st := &MockReaperStore{}
	dc := &MockReaperDocker{}
	sm := &MockSessionManager{}
	r := New(st, dc, time.Minute, testLogger())
	r.SetSessionManager(sm)

	// DB has a running session but no container exists
	dc.On("ListSandboxContainers", mock.Anything).Return([]docker.ContainerInfo{}, nil)
	st.On("ListRunningSessions").Return([]*store.Session{
		{ID: "orphan-session", ContainerID: "missing-container"},
	}, nil)
	st.On("UpdateSessionStatus", "orphan-session", "crashed").Return(nil)
	sm.On("CleanupSessionLock", "orphan-session").Return()

	r.reconcile(context.Background())

	st.AssertCalled(t, "UpdateSessionStatus", "orphan-session", "crashed")
	sm.AssertCalled(t, "CleanupSessionLock", "orphan-session")
}

func TestReconcile_OrphanContainer(t *testing.T) {
	st := &MockReaperStore{}
	dc := &MockReaperDocker{}
	r := New(st, dc, time.Minute, testLogger())

	// Container exists but no DB session
	dc.On("ListSandboxContainers", mock.Anything).Return([]docker.ContainerInfo{
		{ContainerID: "orphan-container-id-12345", SessionID: "no-db-session"},
	}, nil)
	st.On("ListRunningSessions").Return([]*store.Session{}, nil)
	dc.On("RemoveContainer", mock.Anything, "orphan-container-id-12345", "no-db-session").Return(nil)

	r.reconcile(context.Background())

	dc.AssertCalled(t, "RemoveContainer", mock.Anything, "orphan-container-id-12345", "no-db-session")
}

func TestReconcile_MatchingSessionAndContainer(t *testing.T) {
	st := &MockReaperStore{}
	dc := &MockReaperDocker{}
	r := New(st, dc, time.Minute, testLogger())

	dc.On("ListSandboxContainers", mock.Anything).Return([]docker.ContainerInfo{
		{ContainerID: "container-1", SessionID: "session-1"},
	}, nil)
	st.On("ListRunningSessions").Return([]*store.Session{
		{ID: "session-1", ContainerID: "container-1"},
	}, nil)

	r.reconcile(context.Background())

	// Neither should be cleaned up
	dc.AssertNotCalled(t, "RemoveContainer")
	st.AssertNotCalled(t, "UpdateSessionStatus")
}
