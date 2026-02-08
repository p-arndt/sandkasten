package pool

import (
	"context"

	"github.com/p-arndt/sandkasten/internal/docker"
)

// PoolDocker abstracts docker operations needed by the pool.
type PoolDocker interface {
	CreateContainer(ctx context.Context, opts docker.CreateOpts) (string, error)
	RemoveContainer(ctx context.Context, containerID string, sessionID string) error
}
