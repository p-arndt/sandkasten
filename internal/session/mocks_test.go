package session

import (
	"context"
	"time"

	"github.com/p-arndt/sandkasten/internal/runtime"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/protocol"
	"github.com/stretchr/testify/mock"
)

type MockRuntimeDriver struct {
	mock.Mock
}

func (m *MockRuntimeDriver) Create(ctx context.Context, opts runtime.CreateOpts) (*runtime.SessionInfo, error) {
	args := m.Called(ctx, opts)
	if info := args.Get(0); info != nil {
		return info.(*runtime.SessionInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRuntimeDriver) Exec(ctx context.Context, sessionID string, req protocol.Request) (*protocol.Response, error) {
	args := m.Called(ctx, sessionID, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*protocol.Response), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRuntimeDriver) Destroy(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockRuntimeDriver) IsRunning(ctx context.Context, sessionID string) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRuntimeDriver) Stats(ctx context.Context, sessionID string) (*protocol.SessionStats, error) {
	args := m.Called(ctx, sessionID)
	if stats := args.Get(0); stats != nil {
		return stats.(*protocol.SessionStats), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRuntimeDriver) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRuntimeDriver) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRuntimeDriver) MountWorkspace(ctx context.Context, sessionID string, workspaceID string) error {
	args := m.Called(ctx, sessionID, workspaceID)
	return args.Error(0)
}

type MockSessionStore struct {
	mock.Mock
}

func (m *MockSessionStore) CreateSession(sess *store.Session) error {
	args := m.Called(sess)
	return args.Error(0)
}

func (m *MockSessionStore) GetSession(id string) (*store.Session, error) {
	args := m.Called(id)
	if sess := args.Get(0); sess != nil {
		return sess.(*store.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionStore) ListSessions() ([]*store.Session, error) {
	args := m.Called()
	if sessions := args.Get(0); sessions != nil {
		return sessions.([]*store.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionStore) UpdateSessionActivity(id string, cwd string, expiresAt time.Time) error {
	args := m.Called(id, cwd, expiresAt)
	return args.Error(0)
}

func (m *MockSessionStore) UpdateSessionStatus(id string, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *MockSessionStore) UpdateSessionWorkspace(id string, workspaceID string) error {
	args := m.Called(id, workspaceID)
	return args.Error(0)
}

type MockContainerPool struct {
	mock.Mock
}

func (m *MockContainerPool) Get(ctx context.Context, image string, workspaceID string) (string, bool) {
	args := m.Called(ctx, image, workspaceID)
	return args.String(0), args.Bool(1)
}

func (m *MockContainerPool) Put(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockContainerPool) Refill(ctx context.Context, image string, workspaceID string, count int) error {
	args := m.Called(ctx, image, workspaceID, count)
	return args.Error(0)
}

type MockWorkspaceManager struct {
	mock.Mock
}

func (m *MockWorkspaceManager) Create(ctx context.Context, workspaceID string, labels map[string]string) error {
	args := m.Called(ctx, workspaceID, labels)
	return args.Error(0)
}

func (m *MockWorkspaceManager) Exists(ctx context.Context, workspaceID string) (bool, error) {
	args := m.Called(ctx, workspaceID)
	return args.Bool(0), args.Error(1)
}

func (m *MockWorkspaceManager) Delete(ctx context.Context, workspaceID string) error {
	args := m.Called(ctx, workspaceID)
	return args.Error(0)
}
