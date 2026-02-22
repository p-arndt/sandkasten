package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/p-arndt/sandkasten/internal/runtime"
	storemod "github.com/p-arndt/sandkasten/internal/store"
)

func (m *Manager) Create(ctx context.Context, opts CreateOpts) (*SessionInfo, error) {
	image := m.resolveImage(opts.Image)
	if !isImageNameSafe(image) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidImage, image)
	}
	if !m.isImageAllowed(image) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidImage, image)
	}

	ttl := m.resolveTTL(opts.TTLSeconds)
	workspaceID := opts.WorkspaceID
	acquireDetail := ""

	if err := m.ensureWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Try pool acquire first (image+workspace aware).
	if m.pool != nil {
		if sessionID, ok := m.pool.Get(ctx, image, workspaceID); ok {
			sess, err := m.store.GetSession(sessionID)
			if err == nil && sess != nil {
				// Workspace-aware pool entry: already mounted at create time.
				if workspaceID != "" && sess.WorkspaceID == workspaceID {
					if info := m.finishPoolAcquire(ctx, sessionID, sess, workspaceID, ttl); info != nil {
						go m.pool.Refill(context.Background(), image, workspaceID, 1)
						return info, nil
					}
					acquireDetail = "pool_finish_acquire_failed"
				} else if workspaceID != "" && sess.WorkspaceID != "" && sess.WorkspaceID != workspaceID {
					acquireDetail = "pool_workspace_mismatch"
					_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
					_ = m.runtime.Destroy(ctx, sessionID)
					go m.pool.Refill(context.Background(), image, sess.WorkspaceID, 1)
					// mismatch should not happen for keyed acquire, but fail safe to cold create
				} else {
					// Legacy/unbound pool entry: bind-mount workspace when needed.
					if workspaceID != "" {
						if err := m.runtime.MountWorkspace(ctx, sessionID, workspaceID); err != nil {
							acquireDetail = "pool_mount_workspace_failed"
							_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
							_ = m.runtime.Destroy(ctx, sessionID)
						} else if err := m.store.UpdateSessionWorkspace(sessionID, workspaceID); err != nil {
							acquireDetail = "pool_update_workspace_failed"
							_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
							_ = m.runtime.Destroy(ctx, sessionID)
						} else if info := m.finishPoolAcquire(ctx, sessionID, sess, workspaceID, ttl); info != nil {
							go m.pool.Refill(context.Background(), image, workspaceID, 1)
							return info, nil
						} else {
							acquireDetail = "pool_finish_acquire_failed"
						}
					} else if info := m.finishPoolAcquire(ctx, sessionID, sess, workspaceID, ttl); info != nil {
						go m.pool.Refill(context.Background(), image, "", 0)
						return info, nil
					} else {
						acquireDetail = "pool_finish_acquire_failed"
					}
				}
			} else {
				acquireDetail = "pool_session_lookup_failed"
			}
		} else if workspaceID != "" && !m.cfg.Defaults.ReadonlyRootfs {
			// Optional fallback to global image pool when writable rootfs allows late bind-mount.
			if sessionID, ok := m.pool.Get(ctx, image, ""); ok {
				sess, err := m.store.GetSession(sessionID)
				if err == nil && sess != nil {
					if err := m.runtime.MountWorkspace(ctx, sessionID, workspaceID); err != nil {
						acquireDetail = "pool_mount_workspace_failed"
						_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
						_ = m.runtime.Destroy(ctx, sessionID)
					} else if err := m.store.UpdateSessionWorkspace(sessionID, workspaceID); err != nil {
						acquireDetail = "pool_update_workspace_failed"
						_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
						_ = m.runtime.Destroy(ctx, sessionID)
					} else if info := m.finishPoolAcquire(ctx, sessionID, sess, workspaceID, ttl); info != nil {
						go m.pool.Refill(context.Background(), image, workspaceID, 1)
						return info, nil
					}
				}
			}
			if acquireDetail == "" {
				acquireDetail = "pool_empty"
			}
		} else {
			acquireDetail = "pool_empty"
		}
	}

	sessionID := uuid.New().String()[:12]
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)

	info, err := m.runtime.Create(ctx, runtime.CreateOpts{
		SessionID:   sessionID,
		Image:       image,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}

	sess := &storemod.Session{
		ID:           sessionID,
		Image:        image,
		InitPID:      info.InitPID,
		CgroupPath:   info.CgroupPath,
		Status:       "running",
		Cwd:          "/workspace",
		WorkspaceID:  workspaceID,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastActivity: now,
	}

	if err := m.store.CreateSession(sess); err != nil {
		_ = m.runtime.Destroy(ctx, sessionID)
		return nil, fmt.Errorf("store session: %w", err)
	}

	// Background refill when pool is enabled (replenish after normal create)
	if m.pool != nil {
		if workspaceID != "" {
			go m.pool.Refill(context.Background(), image, workspaceID, 1)
		} else {
			go m.pool.Refill(context.Background(), image, "", 0)
		}
	}

	return &SessionInfo{
		ID:            sessionID,
		Image:         image,
		Status:        "running",
		Cwd:           "/workspace",
		AcquireSource: "cold",
		AcquireDetail: acquireDetail,
		WorkspaceID:   workspaceID,
		CreatedAt:     now,
		ExpiresAt:     expiresAt,
	}, nil
}

// finishPoolAcquire updates session status/activity and returns SessionInfo on success.
// Returns nil on any error (caller should fall through to normal create).
func (m *Manager) finishPoolAcquire(ctx context.Context, sessionID string, sess *storemod.Session, workspaceID string, ttl int) *SessionInfo {
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)
	if err := m.store.UpdateSessionStatus(sessionID, "running"); err != nil {
		_ = m.runtime.Destroy(ctx, sessionID)
		_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
		return nil
	}
	if err := m.store.UpdateSessionActivity(sessionID, sess.Cwd, expiresAt); err != nil {
		_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
		_ = m.runtime.Destroy(ctx, sessionID)
		return nil
	}
	return &SessionInfo{
		ID:            sessionID,
		Image:         sess.Image,
		Status:        "running",
		Cwd:           sess.Cwd,
		AcquireSource: "pool",
		WorkspaceID:   workspaceID,
		CreatedAt:     sess.CreatedAt,
		ExpiresAt:     expiresAt,
	}
}

func (m *Manager) resolveImage(image string) string {
	if image == "" {
		return m.cfg.DefaultImage
	}
	return image
}

func (m *Manager) resolveTTL(ttl int) int {
	if ttl <= 0 {
		return m.cfg.SessionTTLSeconds
	}
	return ttl
}

func (m *Manager) ensureWorkspace(ctx context.Context, workspaceID string) error {
	if workspaceID == "" || !m.cfg.Workspace.Enabled {
		return nil
	}

	if m.workspace == nil {
		return nil
	}

	exists, err := m.workspace.Exists(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("check workspace: %w", err)
	}

	if !exists {
		if err := m.workspace.Create(ctx, workspaceID, map[string]string{
			"sandkasten.workspace_id": workspaceID,
		}); err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}
	}

	return nil
}
