package pool

import "context"

// Pool provides pre-warmed sandbox sessions for fast acquisition.
// Sessions are created without workspace (workspace_id=""); sessions with
// workspace always use the normal create path.
type Pool interface {
	// Get acquires an idle session for the given image and workspaceID.
	// workspaceID is reserved for future use; currently only "" is supported.
	// Returns (sessionID, true) if a session was available, ( "", false) otherwise.
	Get(ctx context.Context, image string, workspaceID string) (string, bool)

	// Put returns a session to the pool. Only used when refill-on-release is implemented.
	// Currently a no-op; sessions are destroyed and refill happens in background.
	Put(ctx context.Context, sessionID string) error

	// Refill ensures the pool for the given image has at least count idle sessions.
	// Called in background after create or when pool is low.
	Refill(ctx context.Context, image string, count int) error

	// RefillAll pre-warms the pool for all configured images (daemon startup).
	RefillAll(ctx context.Context)
}
