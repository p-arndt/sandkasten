package api

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/stretchr/testify/mock"
)

type MockSessionService struct {
	mock.Mock
}

func (m *MockSessionService) Create(ctx context.Context, opts session.CreateOpts) (*session.SessionInfo, error) {
	args := m.Called(ctx, opts)
	if info := args.Get(0); info != nil {
		return info.(*session.SessionInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionService) Get(ctx context.Context, id string) (*session.SessionInfo, error) {
	args := m.Called(ctx, id)
	if info := args.Get(0); info != nil {
		return info.(*session.SessionInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionService) List(ctx context.Context) ([]session.SessionInfo, error) {
	args := m.Called(ctx)
	if sessions := args.Get(0); sessions != nil {
		return sessions.([]session.SessionInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionService) Destroy(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionService) Exec(ctx context.Context, sessionID, cmd string, timeoutMs int) (*session.ExecResult, error) {
	args := m.Called(ctx, sessionID, cmd, timeoutMs)
	if result := args.Get(0); result != nil {
		return result.(*session.ExecResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionService) ExecStream(ctx context.Context, sessionID, cmd string, timeoutMs int, chunkChan chan<- session.ExecChunk) error {
	args := m.Called(ctx, sessionID, cmd, timeoutMs, chunkChan)
	return args.Error(0)
}

func (m *MockSessionService) Write(ctx context.Context, sessionID, path string, content []byte, isBase64 bool) error {
	args := m.Called(ctx, sessionID, path, content, isBase64)
	return args.Error(0)
}

func (m *MockSessionService) Read(ctx context.Context, sessionID, path string, maxBytes int) (string, bool, error) {
	args := m.Called(ctx, sessionID, path, maxBytes)
	return args.String(0), args.Bool(1), args.Error(2)
}

func (m *MockSessionService) ListWorkspaces(ctx context.Context) ([]*session.WorkspaceInfo, error) {
	args := m.Called(ctx)
	if ws := args.Get(0); ws != nil {
		return ws.([]*session.WorkspaceInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionService) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	args := m.Called(ctx, workspaceID)
	return args.Error(0)
}

func (m *MockSessionService) ListWorkspaceFiles(ctx context.Context, workspaceID, path string) ([]session.WorkspaceFileEntry, error) {
	args := m.Called(ctx, workspaceID, path)
	if e := args.Get(0); e != nil {
		return e.([]session.WorkspaceFileEntry), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSessionService) ReadWorkspaceFile(ctx context.Context, workspaceID, path string, maxBytes int) (string, bool, error) {
	args := m.Called(ctx, workspaceID, path, maxBytes)
	return args.String(0), args.Bool(1), args.Error(2)
}
