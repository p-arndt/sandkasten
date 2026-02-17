package session

import (
	"context"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWriteText(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := &store.Session{
		ID:        "s1",
		InitPID:   12345,
		Status:    "running",
		Cwd:       "/workspace",
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.MatchedBy(func(req protocol.Request) bool {
		return req.Type == protocol.RequestWrite && req.Path == "/workspace/test.py" && req.Text == "print('hello')"
	})).Return(&protocol.Response{
		Type: protocol.ResponseWrite,
		OK:   true,
	}, nil)
	st.On("UpdateSessionActivity", "s1", "/workspace", mock.AnythingOfType("time.Time")).Return(nil)

	err := mgr.Write(context.Background(), "s1", "/workspace/test.py", []byte("print('hello')"), false)
	require.NoError(t, err)
}

func TestWriteBase64(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := &store.Session{
		ID:        "s1",
		InitPID:   12345,
		Status:    "running",
		Cwd:       "/workspace",
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.MatchedBy(func(req protocol.Request) bool {
		return req.Type == protocol.RequestWrite && req.ContentBase64 == "aGVsbG8="
	})).Return(&protocol.Response{
		Type: protocol.ResponseWrite,
		OK:   true,
	}, nil)
	st.On("UpdateSessionActivity", "s1", "/workspace", mock.AnythingOfType("time.Time")).Return(nil)

	err := mgr.Write(context.Background(), "s1", "/workspace/data.bin", []byte("aGVsbG8="), true)
	require.NoError(t, err)
}

func TestReadSuccess(t *testing.T) {
	mgr, rt, st := newTestManager()
	sess := &store.Session{
		ID:        "s1",
		InitPID:   12345,
		Status:    "running",
		Cwd:       "/workspace",
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}

	st.On("GetSession", "s1").Return(sess, nil)
	rt.On("Exec", mock.Anything, "s1", mock.MatchedBy(func(req protocol.Request) bool {
		return req.Type == protocol.RequestRead && req.Path == "/workspace/test.py"
	})).Return(&protocol.Response{
		Type:          protocol.ResponseRead,
		ContentBase64: "cHJpbnQoJ2hlbGxvJyk=",
		Truncated:     false,
	}, nil)
	st.On("UpdateSessionActivity", "s1", "/workspace", mock.AnythingOfType("time.Time")).Return(nil)

	content, truncated, err := mgr.Read(context.Background(), "s1", "/workspace/test.py", 0)
	require.NoError(t, err)
	assert.Equal(t, "cHJpbnQoJ2hlbGxvJyk=", content)
	assert.False(t, truncated)
}

func TestReadNotFound(t *testing.T) {
	mgr, _, st := newTestManager()

	st.On("GetSession", "nonexistent").Return(nil, nil)

	_, _, err := mgr.Read(context.Background(), "nonexistent", "/workspace/test.py", 0)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestWriteNotFound(t *testing.T) {
	mgr, _, st := newTestManager()

	st.On("GetSession", "nonexistent").Return(nil, nil)

	err := mgr.Write(context.Background(), "nonexistent", "/workspace/test.py", []byte("data"), false)
	assert.ErrorIs(t, err, ErrNotFound)
}
