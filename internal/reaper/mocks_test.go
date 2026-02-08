package reaper

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/mock"
)

// MockReaperStore mocks the ReaperStore interface.
type MockReaperStore struct {
	mock.Mock
}

func (m *MockReaperStore) ListExpiredSessions() ([]*store.Session, error) {
	args := m.Called()
	if sessions := args.Get(0); sessions != nil {
		return sessions.([]*store.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockReaperStore) ListRunningSessions() ([]*store.Session, error) {
	args := m.Called()
	if sessions := args.Get(0); sessions != nil {
		return sessions.([]*store.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockReaperStore) UpdateSessionStatus(id string, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}

// MockReaperDocker mocks the ReaperDocker interface.
type MockReaperDocker struct {
	mock.Mock
}

func (m *MockReaperDocker) RemoveContainer(ctx context.Context, containerID string, sessionID string) error {
	args := m.Called(ctx, containerID, sessionID)
	return args.Error(0)
}

func (m *MockReaperDocker) ListSandboxContainers(ctx context.Context) ([]docker.ContainerInfo, error) {
	args := m.Called(ctx)
	if containers := args.Get(0); containers != nil {
		return containers.([]docker.ContainerInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockSessionManager mocks the SessionManager interface.
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) CleanupSessionLock(id string) {
	m.Called(id)
}
