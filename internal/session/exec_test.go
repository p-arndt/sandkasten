package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func runningSession(id string) *store.Session {
	now := time.Now().UTC()
	return &store.Session{
		ID:           id,
		Image:        "base",
		InitPID:      12345,
		CgroupPath:   "/sys/fs/cgroup/sandkasten/" + id,
		Status:       "running",
		Cwd:          "/workspace",
		CreatedAt:    now,
		ExpiresAt:    now.Add(5 * time.Minute),
		LastActivity: now,
	}
}

func TestExecSuccess(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := runningSession("s1")

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.AnythingOfType("protocol.Request")).Return(&protocol.Response{
		Type:       protocol.ResponseExec,
		ExitCode:   0,
		Cwd:        "/workspace",
		Output:     "hello\n",
		DurationMs: 42,
	}, nil)
	st.On("UpdateSessionActivity", "s1", "/workspace", mock.AnythingOfType("time.Time")).Return(nil)

	result, err := mgr.Exec(context.Background(), "s1", "echo hello", 5000)
	require.NoError(t, err)

	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "hello\n", result.Output)
	assert.Equal(t, "/workspace", result.Cwd)
	assert.Equal(t, int64(42), result.DurationMs)
}

func TestExecNotFound(t *testing.T) {
	mgr, _, st := newTestManager()

	st.On("GetSession", "nonexistent").Return(nil, nil)

	_, err := mgr.Exec(context.Background(), "nonexistent", "ls", 0)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestExecExpired(t *testing.T) {
	mgr, _, st := newTestManager()
	sess := runningSession("expired")
	sess.ExpiresAt = time.Now().UTC().Add(-1 * time.Minute)

	st.On("GetSession", "expired").Return(sess, nil)

	_, err := mgr.Exec(context.Background(), "expired", "ls", 0)
	assert.ErrorIs(t, err, ErrExpired)
}

func TestExecNotRunning(t *testing.T) {
	mgr, _, st := newTestManager()
	sess := runningSession("stopped")
	sess.Status = "destroyed"

	st.On("GetSession", "stopped").Return(sess, nil)

	_, err := mgr.Exec(context.Background(), "stopped", "ls", 0)
	assert.ErrorIs(t, err, ErrNotRunning)
}

func TestExecRunnerError(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := runningSession("s1")

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.Anything).Return(&protocol.Response{
		Type:  protocol.ResponseError,
		Error: "command not found",
	}, nil)

	_, err := mgr.Exec(context.Background(), "s1", "badcmd", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runner error")
}

func TestExecRuntimeFailure(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := runningSession("s1")

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.Anything).Return(nil, fmt.Errorf("runtime exec failed"))

	_, err := mgr.Exec(context.Background(), "s1", "ls", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exec")
}

func TestExecStreamSuccess(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := runningSession("s1")

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.Anything).Return(&protocol.Response{
		Type:       protocol.ResponseExec,
		ExitCode:   0,
		Cwd:        "/workspace",
		Output:     "streaming output\n",
		DurationMs: 100,
	}, nil)
	st.On("UpdateSessionActivity", "s1", "/workspace", mock.AnythingOfType("time.Time")).Return(nil)

	chunkChan := make(chan ExecChunk, 10)
	err := mgr.ExecStream(context.Background(), "s1", "echo streaming output", 5000, chunkChan)
	require.NoError(t, err)

	chunk := <-chunkChan
	assert.True(t, chunk.Done)
	assert.Equal(t, "streaming output\n", chunk.Output)
	assert.Equal(t, 0, chunk.ExitCode)
}

func TestExecTimeoutEnforcement(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := runningSession("s1")

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.MatchedBy(func(req protocol.Request) bool {
		return req.TimeoutMs == 120000
	})).Return(&protocol.Response{
		Type:     protocol.ResponseExec,
		ExitCode: 0,
		Output:   "ok",
	}, nil)
	st.On("UpdateSessionActivity", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	_, err := mgr.Exec(context.Background(), "s1", "echo ok", 999999)
	require.NoError(t, err)
}

func TestExecTimeoutMappedToErrTimeout(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := runningSession("s1")

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.Anything).Return(&protocol.Response{
		Type:     protocol.ResponseExec,
		ExitCode: -1,
		Output:   "timeout: command exceeded 30s",
	}, nil)

	_, err := mgr.Exec(context.Background(), "s1", "sleep 999", 1000)
	assert.ErrorIs(t, err, ErrTimeout)
}

func TestExecStreamTimeoutMappedToErrTimeout(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := runningSession("s1")

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.Anything).Return(&protocol.Response{
		Type:     protocol.ResponseExec,
		ExitCode: -1,
		Output:   "timeout: command exceeded 30s",
	}, nil)

	chunkChan := make(chan ExecChunk, 1)
	err := mgr.ExecStream(context.Background(), "s1", "sleep 999", 1000, chunkChan)
	assert.ErrorIs(t, err, ErrTimeout)
}
