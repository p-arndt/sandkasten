package reaper

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
)

// ReaperStore abstracts store operations needed by the reaper.
type ReaperStore interface {
	ListExpiredSessions() ([]*store.Session, error)
	ListRunningSessions() ([]*store.Session, error)
	UpdateSessionStatus(id string, status string) error
}

// ReaperDocker abstracts docker operations needed by the reaper.
type ReaperDocker interface {
	RemoveContainer(ctx context.Context, containerID string, sessionID string) error
	ListSandboxContainers(ctx context.Context) ([]docker.ContainerInfo, error)
}
