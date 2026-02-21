package reaper

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/store"
)

type ReaperStore interface {
	ListExpiredSessions() ([]*store.Session, error)
	ListRunningSessions() ([]*store.Session, error)
	GetSession(id string) (*store.Session, error)
	UpdateSessionStatus(id string, status string) error
}

type ReaperRuntime interface {
	Destroy(ctx context.Context, sessionID string) error
	IsRunning(ctx context.Context, sessionID string) (bool, error)
	ListSessionDirIDs(ctx context.Context) ([]string, error)
}
