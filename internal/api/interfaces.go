package api

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/protocol"
)

type SessionService interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.SessionInfo, error)
	Get(ctx context.Context, id string) (*session.SessionInfo, error)
	GetStats(ctx context.Context, id string) (*protocol.SessionStats, error)
	List(ctx context.Context) ([]session.SessionInfo, error)
	Destroy(ctx context.Context, sessionID string) error
	Exec(ctx context.Context, sessionID, cmd string, timeoutMs int, rawOutput bool) (*session.ExecResult, error)
	ExecStream(ctx context.Context, sessionID, cmd string, timeoutMs int, rawOutput bool, chunkChan chan<- session.ExecChunk) error
	Write(ctx context.Context, sessionID, path string, content []byte, isBase64 bool) error
	Read(ctx context.Context, sessionID, path string, maxBytes int) (string, bool, error)
	ListWorkspaces(ctx context.Context) ([]*session.WorkspaceInfo, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
	ListWorkspaceFiles(ctx context.Context, workspaceID, path string) ([]session.WorkspaceFileEntry, error)
	ReadWorkspaceFile(ctx context.Context, workspaceID, path string, maxBytes int) (contentBase64 string, truncated bool, err error)
	WriteWorkspaceFile(ctx context.Context, workspaceID, path string, content []byte, isBase64 bool) error
}
