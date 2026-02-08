package pool

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/stretchr/testify/mock"
)

// MockPoolDocker mocks the PoolDocker interface.
type MockPoolDocker struct {
	mock.Mock
}

func (m *MockPoolDocker) CreateContainer(ctx context.Context, opts docker.CreateOpts) (string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Error(1)
}

func (m *MockPoolDocker) RemoveContainer(ctx context.Context, containerID string, sessionID string) error {
	args := m.Called(ctx, containerID, sessionID)
	return args.Error(0)
}
