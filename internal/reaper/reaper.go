package reaper

import (
	"context"
	"log/slog"
	"time"
)

// SessionManager interface for lock cleanup
type SessionManager interface {
	CleanupSessionLock(id string)
}

type Reaper struct {
	store          ReaperStore
	docker         ReaperDocker
	sessionManager SessionManager
	interval       time.Duration
	logger         *slog.Logger
}

func New(st ReaperStore, dc ReaperDocker, interval time.Duration, logger *slog.Logger) *Reaper {
	return &Reaper{
		store:    st,
		docker:   dc,
		interval: interval,
		logger:   logger,
	}
}

// SetSessionManager sets the session manager for lock cleanup (called after manager is created)
func (r *Reaper) SetSessionManager(sm SessionManager) {
	r.sessionManager = sm
}

// Run starts the reaper loop. It blocks until ctx is cancelled.
func (r *Reaper) Run(ctx context.Context) {
	r.logger.Info("reaper started", "interval", r.interval)

	// Reconcile on startup.
	r.reconcile(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("reaper stopped")
			return
		case <-ticker.C:
			r.reapExpired(ctx)
		}
	}
}

// reapExpired finds and destroys sessions past their TTL.
func (r *Reaper) reapExpired(ctx context.Context) {
	expired, err := r.store.ListExpiredSessions()
	if err != nil {
		r.logger.Error("reaper: list expired", "error", err)
		return
	}

	for _, sess := range expired {
		r.logger.Info("reaping expired session", "session_id", sess.ID, "expired_at", sess.ExpiresAt)

		if err := r.docker.RemoveContainer(ctx, sess.ContainerID, sess.ID); err != nil {
			r.logger.Error("reaper: remove container", "session_id", sess.ID, "error", err)
		}

		if err := r.store.UpdateSessionStatus(sess.ID, "expired"); err != nil {
			r.logger.Error("reaper: update status", "session_id", sess.ID, "error", err)
		}

		// Clean up session lock to prevent memory leak
		if r.sessionManager != nil {
			r.sessionManager.CleanupSessionLock(sess.ID)
		}
	}

	if len(expired) > 0 {
		r.logger.Info("reaper: reaped sessions", "count", len(expired))
	}
}

// reconcile syncs DB state with Docker reality.
func (r *Reaper) reconcile(ctx context.Context) {
	r.logger.Info("reconciliation starting")

	containers, err := r.docker.ListSandboxContainers(ctx)
	if err != nil {
		r.logger.Error("reconcile: list containers", "error", err)
		return
	}

	// Build set of container session IDs.
	containerSessions := make(map[string]string) // sessionID -> containerID
	for _, c := range containers {
		containerSessions[c.SessionID] = c.ContainerID
	}

	// Check all running sessions in DB.
	running, err := r.store.ListRunningSessions()
	if err != nil {
		r.logger.Error("reconcile: list running sessions", "error", err)
		return
	}

	for _, sess := range running {
		if _, exists := containerSessions[sess.ID]; !exists {
			r.logger.Warn("reconcile: container missing for running session, marking crashed",
				"session_id", sess.ID)
			r.store.UpdateSessionStatus(sess.ID, "crashed")
			// Clean up session lock
			if r.sessionManager != nil {
				r.sessionManager.CleanupSessionLock(sess.ID)
			}
		}
		// Remove from map — anything left is an orphan.
		delete(containerSessions, sess.ID)
	}

	// Remaining entries are orphan containers (no DB session).
	for sessionID, containerID := range containerSessions {
		r.logger.Warn("reconcile: orphan container, removing",
			"session_id", sessionID, "container_id", containerID[:12])
		r.docker.RemoveContainer(ctx, containerID, sessionID)
	}

	r.logger.Info("reconciliation complete")
}
