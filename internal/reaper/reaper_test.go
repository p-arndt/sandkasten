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
	st.On("UpdateSessionStatus", "orphan-session", "crashed").Return(nil)
	sm.On("CleanupSessionLock", "orphan-session").Return()

	r.reconcile(context.Background())

	st.AssertCalled(t, "UpdateSessionStatus", "orphan-session", "crashed")
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

	r.reconcile(context.Background())

	st.AssertNotCalled(t, "UpdateSessionStatus")
}
