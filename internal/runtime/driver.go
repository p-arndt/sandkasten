package runtime

import (
	"context"

	"github.com/p-arndt/sandkasten/protocol"
)

type CreateOpts struct {
	SessionID   string
	Image       string
	WorkspaceID string
}

type SessionInfo struct {
	SessionID  string
	InitPID    int
	CgroupPath string
	Mnt        string
	RunnerSock string
}

type Driver interface {
	Create(ctx context.Context, opts CreateOpts) (*SessionInfo, error)
	Exec(ctx context.Context, sessionID string, req protocol.Request) (*protocol.Response, error)
	Destroy(ctx context.Context, sessionID string) error
	IsRunning(ctx context.Context, sessionID string) (bool, error)
	Ping(ctx context.Context) error
	Close() error
}
