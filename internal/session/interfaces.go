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
	Stats(ctx context.Context, sessionID string) (*protocol.SessionStats, error)
	Ping(ctx context.Context) error
	Close() error
	MountWorkspace(ctx context.Context, sessionID string, workspaceID string) error
}

type SessionStore interface {
	CreateSession(sess *store.Session) error
	GetSession(id string) (*store.Session, error)
	ListSessions() ([]*store.Session, error)
	UpdateSessionActivity(id string, cwd string, expiresAt time.Time) error
	UpdateSessionStatus(id string, status string) error
	UpdateSessionWorkspace(id string, workspaceID string) error
}

// ContainerPool provides pre-warmed sessions for fast acquisition.
// Pools can be keyed by image only (workspace_id="") or image+workspace_id.
type ContainerPool interface {
	Get(ctx context.Context, image string, workspaceID string) (string, bool)
	Put(ctx context.Context, sessionID string) error
	Refill(ctx context.Context, image string, workspaceID string, count int) error
}

type WorkspaceManager interface {
	Create(ctx context.Context, workspaceID string, labels map[string]string) error
	Exists(ctx context.Context, workspaceID string) (bool, error)
	Delete(ctx context.Context, workspaceID string) error
}
