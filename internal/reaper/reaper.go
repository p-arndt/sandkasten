package reaper

import (
	"context"
	"log/slog"
	"time"
)

type SessionManager interface {
	CleanupSessionLock(id string)
}

type Reaper struct {
	store          ReaperStore
	runtime        ReaperRuntime
	sessionManager SessionManager
	interval       time.Duration
	logger         *slog.Logger
}

func New(st ReaperStore, rt ReaperRuntime, interval time.Duration, logger *slog.Logger) *Reaper {
	return &Reaper{
		store:    st,
		runtime:  rt,
		interval: interval,
		logger:   logger,
	}
}

func (r *Reaper) SetSessionManager(sm SessionManager) {
	r.sessionManager = sm
}

func (r *Reaper) Run(ctx context.Context) {
	r.logger.Info("reaper started", "interval", r.interval)

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

func (r *Reaper) reapExpired(ctx context.Context) {
	expired, err := r.store.ListExpiredSessions()
	if err != nil {
		r.logger.Error("reaper: list expired", "error", err)
		return
	}

	for _, sess := range expired {
		r.logger.Info("reaping expired session", "session_id", sess.ID, "expired_at", sess.ExpiresAt)

		if err := r.runtime.Destroy(ctx, sess.ID); err != nil {
			r.logger.Error("reaper: destroy session", "session_id", sess.ID, "error", err)
		}

		if err := r.store.UpdateSessionStatus(sess.ID, "expired"); err != nil {
			r.logger.Error("reaper: update status", "session_id", sess.ID, "error", err)
		}

		if r.sessionManager != nil {
			r.sessionManager.CleanupSessionLock(sess.ID)
		}
	}

	if len(expired) > 0 {
		r.logger.Info("reaper: reaped sessions", "count", len(expired))
	}
}

func (r *Reaper) reconcile(ctx context.Context) {
	r.logger.Info("reconciliation starting")

	running, err := r.store.ListRunningSessions()
	if err != nil {
		r.logger.Error("reconcile: list running sessions", "error", err)
		return
	}

	for _, sess := range running {
		isRunning, err := r.runtime.IsRunning(ctx, sess.ID)
		if err != nil {
			r.logger.Warn("reconcile: error checking session status",
				"session_id", sess.ID, "error", err)
			continue
		}

		if !isRunning {
			r.logger.Warn("reconcile: session process not running, marking crashed and cleaning up",
				"session_id", sess.ID)
			if err := r.runtime.Destroy(ctx, sess.ID); err != nil {
				r.logger.Error("reconcile: destroy crashed session", "session_id", sess.ID, "error", err)
			}
			if err := r.store.UpdateSessionStatus(sess.ID, "crashed"); err != nil {
				r.logger.Error("reconcile: update status to crashed", "session_id", sess.ID, "error", err)
			}
			if r.sessionManager != nil {
				r.sessionManager.CleanupSessionLock(sess.ID)
			}
		}
	}

	r.reconcileOrphans(ctx)
	r.logger.Info("reconciliation complete")
}

// reconcileOrphans scans session dirs on disk and destroys any that are not in the store
// or not in status "running" (e.g. orphan dirs left after daemon crash).
func (r *Reaper) reconcileOrphans(ctx context.Context) {
	ids, err := r.runtime.ListSessionDirIDs(ctx)
	if err != nil {
		r.logger.Error("reconcile: list session dirs", "error", err)
		return
	}
	for _, id := range ids {
		sess, err := r.store.GetSession(id)
		if err != nil {
			r.logger.Warn("reconcile: get session for orphan check", "session_id", id, "error", err)
			continue
		}
		if sess == nil || sess.Status != "running" {
			r.logger.Info("reconcile: cleaning orphan session dir", "session_id", id)
			if err := r.runtime.Destroy(ctx, id); err != nil {
				r.logger.Error("reconcile: destroy orphan session", "session_id", id, "error", err)
			}
			if sess != nil && r.sessionManager != nil {
				r.sessionManager.CleanupSessionLock(id)
			}
		}
	}
}
