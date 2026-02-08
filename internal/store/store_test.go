package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st
}

func testSession(id string) *Session {
	now := time.Now().UTC()
	return &Session{
		ID:           id,
		Image:        "sandbox-runtime:base",
		ContainerID:  "container-" + id,
		Status:       "running",
		Cwd:          "/workspace",
		CreatedAt:    now,
		ExpiresAt:    now.Add(5 * time.Minute),
		LastActivity: now,
	}
}

func TestCreateAndGetSession(t *testing.T) {
	st := newTestStore(t)
	sess := testSession("test-1")

	require.NoError(t, st.CreateSession(sess))

	got, err := st.GetSession("test-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, sess.ID, got.ID)
	assert.Equal(t, sess.Image, got.Image)
	assert.Equal(t, sess.ContainerID, got.ContainerID)
	assert.Equal(t, sess.Status, got.Status)
	assert.Equal(t, sess.Cwd, got.Cwd)
}

func TestGetSessionNotFound(t *testing.T) {
	st := newTestStore(t)

	got, err := st.GetSession("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListSessions(t *testing.T) {
	st := newTestStore(t)

	require.NoError(t, st.CreateSession(testSession("s1")))
	require.NoError(t, st.CreateSession(testSession("s2")))
	require.NoError(t, st.CreateSession(testSession("s3")))

	sessions, err := st.ListSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 3)
}

func TestListSessionsEmpty(t *testing.T) {
	st := newTestStore(t)

	sessions, err := st.ListSessions()
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestUpdateSessionStatus(t *testing.T) {
	st := newTestStore(t)
	require.NoError(t, st.CreateSession(testSession("s1")))

	require.NoError(t, st.UpdateSessionStatus("s1", "destroyed"))

	got, err := st.GetSession("s1")
	require.NoError(t, err)
	assert.Equal(t, "destroyed", got.Status)
}

func TestUpdateSessionStatusNotFound(t *testing.T) {
	st := newTestStore(t)

	err := st.UpdateSessionStatus("nonexistent", "destroyed")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateSessionActivity(t *testing.T) {
	st := newTestStore(t)
	require.NoError(t, st.CreateSession(testSession("s1")))

	newExpiry := time.Now().UTC().Add(10 * time.Minute)
	require.NoError(t, st.UpdateSessionActivity("s1", "/home", newExpiry))

	got, err := st.GetSession("s1")
	require.NoError(t, err)
	assert.Equal(t, "/home", got.Cwd)
}

func TestListExpiredSessions(t *testing.T) {
	st := newTestStore(t)

	// Create a session that's already expired
	expired := testSession("expired-1")
	expired.ExpiresAt = time.Now().UTC().Add(-1 * time.Minute)
	require.NoError(t, st.CreateSession(expired))

	// Create a session that's still valid
	valid := testSession("valid-1")
	valid.ExpiresAt = time.Now().UTC().Add(10 * time.Minute)
	require.NoError(t, st.CreateSession(valid))

	sessions, err := st.ListExpiredSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "expired-1", sessions[0].ID)
}

func TestListRunningSessions(t *testing.T) {
	st := newTestStore(t)

	running := testSession("running-1")
	require.NoError(t, st.CreateSession(running))

	destroyed := testSession("destroyed-1")
	require.NoError(t, st.CreateSession(destroyed))
	require.NoError(t, st.UpdateSessionStatus("destroyed-1", "destroyed"))

	sessions, err := st.ListRunningSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "running-1", sessions[0].ID)
}

func TestDeleteSession(t *testing.T) {
	st := newTestStore(t)
	require.NoError(t, st.CreateSession(testSession("s1")))

	require.NoError(t, st.DeleteSession("s1"))

	got, err := st.GetSession("s1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDeleteSessionNotFound(t *testing.T) {
	st := newTestStore(t)

	err := st.DeleteSession("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSessionWithWorkspaceID(t *testing.T) {
	st := newTestStore(t)

	sess := testSession("ws-1")
	sess.WorkspaceID = "my-workspace"
	require.NoError(t, st.CreateSession(sess))

	got, err := st.GetSession("ws-1")
	require.NoError(t, err)
	assert.Equal(t, "my-workspace", got.WorkspaceID)
}

func TestSessionWithEmptyWorkspaceID(t *testing.T) {
	st := newTestStore(t)

	sess := testSession("no-ws")
	require.NoError(t, st.CreateSession(sess))

	got, err := st.GetSession("no-ws")
	require.NoError(t, err)
	assert.Empty(t, got.WorkspaceID)
}

func TestDuplicateSessionID(t *testing.T) {
	st := newTestStore(t)
	require.NoError(t, st.CreateSession(testSession("dup")))

	err := st.CreateSession(testSession("dup"))
	assert.Error(t, err)
}
