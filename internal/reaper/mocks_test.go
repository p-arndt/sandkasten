package reaper

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/mock"
)

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

type MockReaperRuntime struct {
	mock.Mock
}

func (m *MockReaperRuntime) Destroy(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockReaperRuntime) IsRunning(ctx context.Context, sessionID string) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}

type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) CleanupSessionLock(id string) {
	m.Called(id)
}
