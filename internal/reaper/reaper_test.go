package reaper

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestReapExpired_NoExpired(t *testing.T) {
	st := &MockReaperStore{}
	rt := &MockReaperRuntime{}
	r := New(st, rt, time.Minute, testLogger())

	st.On("ListExpiredSessions").Return([]*store.Session{}, nil)

	r.reapExpired(context.Background())

	st.AssertExpectations(t)
	rt.AssertNotCalled(t, "Destroy")
}

func TestReapExpired_WithExpired(t *testing.T) {
	st := &MockReaperStore{}
	rt := &MockReaperRuntime{}
	sm := &MockSessionManager{}
	r := New(st, rt, time.Minute, testLogger())
	r.SetSessionManager(sm)

	expired := []*store.Session{
		{ID: "s1", InitPID: 123, CgroupPath: "/sys/fs/cgroup/sandkasten/s1", ExpiresAt: time.Now().Add(-time.Minute)},
		{ID: "s2", InitPID: 456, CgroupPath: "/sys/fs/cgroup/sandkasten/s2", ExpiresAt: time.Now().Add(-2 * time.Minute)},
	}

	st.On("ListExpiredSessions").Return(expired, nil)
	rt.On("Destroy", mock.Anything, "s1").Return(nil)
	rt.On("Destroy", mock.Anything, "s2").Return(nil)
	st.On("UpdateSessionStatus", "s1", "expired").Return(nil)
	st.On("UpdateSessionStatus", "s2", "expired").Return(nil)
	sm.On("CleanupSessionLock", "s1").Return()
	sm.On("CleanupSessionLock", "s2").Return()

	r.reapExpired(context.Background())

	st.AssertExpectations(t)
	rt.AssertExpectations(t)
	sm.AssertExpectations(t)
}

func TestReapExpired_NoSessionManager(t *testing.T) {
	st := &MockReaperStore{}
	rt := &MockReaperRuntime{}
	r := New(st, rt, time.Minute, testLogger())

	expired := []*store.Session{
		{ID: "s1", InitPID: 123},
	}

	st.On("ListExpiredSessions").Return(expired, nil)
	rt.On("Destroy", mock.Anything, "s1").Return(nil)
	st.On("UpdateSessionStatus", "s1", "expired").Return(nil)

	require.NotPanics(t, func() {
		r.reapExpired(context.Background())
	})
}

func TestReconcile_SessionNotRunning(t *testing.T) {
	st := &MockReaperStore{}
	rt := &MockReaperRuntime{}
	sm := &MockSessionManager{}
	r := New(st, rt, time.Minute, testLogger())
	r.SetSessionManager(sm)

	st.On("ListRunningSessions").Return([]*store.Session{
		{ID: "orphan-session", InitPID: 999},
	}, nil)
	rt.On("IsRunning", mock.Anything, "orphan-session").Return(false, nil)
	rt.On("Destroy", mock.Anything, "orphan-session").Return(nil)
	st.On("UpdateSessionStatus", "orphan-session", "crashed").Return(nil)
	sm.On("CleanupSessionLock", "orphan-session").Return()
	rt.On("ListSessionDirIDs", mock.Anything).Return([]string{}, nil)

	r.reconcile(context.Background())

	st.AssertCalled(t, "UpdateSessionStatus", "orphan-session", "crashed")
	rt.AssertCalled(t, "Destroy", mock.Anything, "orphan-session")
	sm.AssertCalled(t, "CleanupSessionLock", "orphan-session")
}

func TestReconcile_SessionStillRunning(t *testing.T) {
	st := &MockReaperStore{}
	rt := &MockReaperRuntime{}
	r := New(st, rt, time.Minute, testLogger())

	st.On("ListRunningSessions").Return([]*store.Session{
		{ID: "running-session", InitPID: 123},
	}, nil)
	rt.On("IsRunning", mock.Anything, "running-session").Return(true, nil)
	rt.On("ListSessionDirIDs", mock.Anything).Return([]string{}, nil)

	r.reconcile(context.Background())

	st.AssertNotCalled(t, "UpdateSessionStatus")
}

func TestReconcileOrphans_SkipsPoolIdle(t *testing.T) {
	st := &MockReaperStore{}
	rt := &MockReaperRuntime{}
	r := New(st, rt, time.Minute, testLogger())

	// Session dir exists for pool_idle session - should NOT be destroyed
	st.On("ListRunningSessions").Return([]*store.Session{}, nil)
	rt.On("ListSessionDirIDs", mock.Anything).Return([]string{"pool-session-1"}, nil)
	st.On("GetSession", "pool-session-1").Return(&store.Session{
		ID: "pool-session-1", Status: store.StatusPoolIdle,
	}, nil)

	r.reconcile(context.Background())

	rt.AssertNotCalled(t, "Destroy")
}

func TestReconcileOrphans_DestroysNonRunningNonPoolIdle(t *testing.T) {
	st := &MockReaperStore{}
	rt := &MockReaperRuntime{}
	sm := &MockSessionManager{}
	r := New(st, rt, time.Minute, testLogger())
	r.SetSessionManager(sm)

	st.On("ListRunningSessions").Return([]*store.Session{}, nil)
	rt.On("ListSessionDirIDs", mock.Anything).Return([]string{"orphan-dir"}, nil)
	st.On("GetSession", "orphan-dir").Return(&store.Session{
		ID: "orphan-dir", Status: "destroyed",
	}, nil)
	rt.On("Destroy", mock.Anything, "orphan-dir").Return(nil)
	sm.On("CleanupSessionLock", "orphan-dir").Return()

	r.reconcile(context.Background())

	rt.AssertCalled(t, "Destroy", mock.Anything, "orphan-dir")
}
