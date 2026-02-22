package pool

import "context"

// Pool provides pre-warmed sandbox sessions for fast acquisition.
// Sessions can be pooled globally per-image (workspace_id="") or on-demand
// per image+workspace key for workspace-aware pooling.
type Pool interface {
	// Get acquires an idle session for the given image and workspaceID.
	// Returns (sessionID, true) if a session was available, ( "", false) otherwise.
	Get(ctx context.Context, image string, workspaceID string) (string, bool)

	// Put returns a session to the pool. Only used when refill-on-release is implemented.
	// Currently a no-op; sessions are destroyed and refill happens in background.
	Put(ctx context.Context, sessionID string) error

	// Refill ensures the pool for the given image+workspace key has at least count idle sessions.
	// For workspaceID=="", count<=0 means refill to configured static target.
	// For workspaceID!="", count<=0 means refill to 1.
	// Called in background after create or when pool is low.
	Refill(ctx context.Context, image string, workspaceID string, count int) error

	// RefillAll pre-warms the pool for all configured images (daemon startup).
	RefillAll(ctx context.Context)
}
