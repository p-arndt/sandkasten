package session

import (
	"context"
	"time"

	"github.com/p-arndt/sandkasten/internal/runtime"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/protocol"
)

type RuntimeDriver interface {
	Create(ctx context.Context, opts runtime.CreateOpts) (*runtime.SessionInfo, error)
	Exec(ctx context.Context, sessionID string, req protocol.Request) (*protocol.Response, error)
	Destroy(ctx context.Context, sessionID string) error
	IsRunning(ctx context.Context, sessionID string) (bool, error)
	Ping(ctx context.Context) error
	Close() error
}

type SessionStore interface {
	CreateSession(sess *store.Session) error
	GetSession(id string) (*store.Session, error)
	ListSessions() ([]*store.Session, error)
	UpdateSessionActivity(id string, cwd string, expiresAt time.Time) error
	UpdateSessionStatus(id string, status string) error
}

type ContainerPool interface {
	Get(ctx context.Context, image string) (string, bool)
}

type WorkspaceManager interface {
	Create(ctx context.Context, workspaceID string, labels map[string]string) error
	Exists(ctx context.Context, workspaceID string) (bool, error)
	Delete(ctx context.Context, workspaceID string) error
}
