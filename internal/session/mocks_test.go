package session

import (
	"context"
	"time"

	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/internal/workspace"
	"github.com/p-arndt/sandkasten/protocol"
	"github.com/stretchr/testify/mock"
)

// MockDockerClient mocks the DockerClient interface.
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) CreateContainer(ctx context.Context, opts docker.CreateOpts) (string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Error(1)
}

func (m *MockDockerClient) ExecRunner(ctx context.Context, containerID string, req protocol.Request) (*protocol.Response, error) {
	args := m.Called(ctx, containerID, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*protocol.Response), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDockerClient) RemoveContainer(ctx context.Context, containerID string, sessionID string) error {
	args := m.Called(ctx, containerID, sessionID)
	return args.Error(0)
}

// MockSessionStore mocks the SessionStore interface.
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

// MockContainerPool mocks the ContainerPool interface.
type MockContainerPool struct {
	mock.Mock
}

func (m *MockContainerPool) Get(ctx context.Context, image string) (string, bool) {
	args := m.Called(ctx, image)
	return args.String(0), args.Bool(1)
}

// MockWorkspaceManager mocks the WorkspaceManager interface.
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

func (m *MockWorkspaceManager) List(ctx context.Context) ([]*workspace.Workspace, error) {
	args := m.Called(ctx)
	if ws := args.Get(0); ws != nil {
		return ws.([]*workspace.Workspace), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceManager) Delete(ctx context.Context, workspaceID string) error {
	args := m.Called(ctx, workspaceID)
	return args.Error(0)
}
