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
			r.logger.Warn("reconcile: session process not running, marking crashed",
				"session_id", sess.ID)
			r.store.UpdateSessionStatus(sess.ID, "crashed")
			if r.sessionManager != nil {
				r.sessionManager.CleanupSessionLock(sess.ID)
			}
		}
	}

	r.logger.Info("reconciliation complete")
}
