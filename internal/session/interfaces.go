package session

import (
	"context"
	"time"

	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/internal/workspace"
	"github.com/p-arndt/sandkasten/protocol"
)

// DockerClient abstracts docker operations needed by session management.
type DockerClient interface {
	CreateContainer(ctx context.Context, opts docker.CreateOpts) (string, error)
	ExecRunner(ctx context.Context, containerID string, req protocol.Request) (*protocol.Response, error)
	RemoveContainer(ctx context.Context, containerID string, sessionID string) error
}

// SessionStore abstracts store operations needed by session management.
type SessionStore interface {
	CreateSession(sess *store.Session) error
	GetSession(id string) (*store.Session, error)
	ListSessions() ([]*store.Session, error)
	UpdateSessionActivity(id string, cwd string, expiresAt time.Time) error
	UpdateSessionStatus(id string, status string) error
}

// ContainerPool abstracts pool operations needed by session management.
type ContainerPool interface {
	Get(ctx context.Context, image string) (string, bool)
}

// WorkspaceManager abstracts workspace operations needed by session management.
type WorkspaceManager interface {
	Create(ctx context.Context, workspaceID string, labels map[string]string) error
	Exists(ctx context.Context, workspaceID string) (bool, error)
	List(ctx context.Context) ([]*workspace.Workspace, error)
	Delete(ctx context.Context, workspaceID string) error
}
