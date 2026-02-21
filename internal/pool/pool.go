package pool

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/p-arndt/sandkasten/internal/config"
	storemod "github.com/p-arndt/sandkasten/internal/store"
)

// PoolConfig defines the interface the pool needs from runtime and store.
type PoolConfig struct {
	Store        Store
	CreateFunc   CreateFunc
	Logger       *slog.Logger
	SessionTTL   int
	PoolExpiry   time.Duration // far future for pool_idle sessions
}

type Store interface {
	CreateSession(sess *storemod.Session) error
	GetSession(id string) (*storemod.Session, error)
	UpdateSessionStatus(id string, status string) error
	UpdateSessionActivity(id string, cwd string, expiresAt time.Time) error
}

// CreateFunc creates a new sandbox and returns (sessionID, initPID, cgroupPath, error).
// The pool provides sessionID; CreateFunc creates the sandbox and registers it.
type CreateFunc func(ctx context.Context, sessionID string, image string) (*CreateResult, error)

type CreateResult struct {
	InitPID    int
	CgroupPath string
}

type poolImpl struct {
	cfg    *config.Config
	config PoolConfig

	mu     sync.Mutex
	idle   map[string][]string // image -> []sessionID (idle sessions)
	target map[string]int      // image -> target count
}

// New creates a new session pool. Returns nil if pool is disabled.
func New(cfg *config.Config, poolConfig PoolConfig) *poolImpl {
	if !cfg.Pool.Enabled || len(cfg.Pool.Images) == 0 {
		return nil
	}
	target := make(map[string]int)
	allowed := make(map[string]bool)
	for _, a := range cfg.AllowedImages {
		allowed[a] = true
	}
	for img, n := range cfg.Pool.Images {
		if n > 0 && (len(cfg.AllowedImages) == 0 || allowed[img]) {
			target[img] = n
		}
	}
	if len(target) == 0 {
		return nil
	}
	return &poolImpl{
		cfg:    cfg,
		config: poolConfig,
		idle:   make(map[string][]string),
		target: target,
	}
}

// Get acquires an idle session. workspaceID must be "" for pool to be used.
func (p *poolImpl) Get(ctx context.Context, image string, workspaceID string) (string, bool) {
	if workspaceID != "" {
		return "", false
	}
	target, ok := p.target[image]
	if !ok || target <= 0 {
		return "", false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ids := p.idle[image]
	if len(ids) == 0 {
		return "", false
	}

	sessionID := ids[len(ids)-1]
	p.idle[image] = ids[:len(ids)-1]
	return sessionID, true
}

// Put is a no-op for Phase 1. Sessions are destroyed on release; refill runs in background.
func (p *poolImpl) Put(ctx context.Context, sessionID string) error {
	return nil
}

// Refill creates sandboxes in background until pool reaches target for image.
func (p *poolImpl) Refill(ctx context.Context, image string, count int) error {
	target, ok := p.target[image]
	if !ok || target <= 0 {
		return nil
	}
	if count <= 0 {
		count = target
	}
	if count > target {
		count = target
	}

	p.mu.Lock()
	current := len(p.idle[image])
	needed := count - current
	p.mu.Unlock()

	if needed <= 0 {
		return nil
	}

	for i := 0; i < needed; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		sessionID := uuid.New().String()[:12]
		result, err := p.config.CreateFunc(ctx, sessionID, image)
		if err != nil {
			if p.config.Logger != nil {
				p.config.Logger.Warn("pool refill: create failed", "image", image, "error", err)
			}
			continue
		}

		now := time.Now().UTC()
		expiresAt := now.Add(p.config.PoolExpiry)
		sess := &storemod.Session{
			ID:           sessionID,
			Image:        image,
			InitPID:      result.InitPID,
			CgroupPath:   result.CgroupPath,
			Status:       storemod.StatusPoolIdle,
			Cwd:          "/workspace",
			WorkspaceID:  "",
			CreatedAt:    now,
			ExpiresAt:    expiresAt,
			LastActivity: now,
		}
		if err := p.config.Store.CreateSession(sess); err != nil {
			if p.config.Logger != nil {
				p.config.Logger.Warn("pool refill: store failed", "session_id", sessionID, "error", err)
			}
			continue
		}

		p.mu.Lock()
		p.idle[image] = append(p.idle[image], sessionID)
		p.mu.Unlock()
	}
	return nil
}

// RefillAll pre-warms the pool for all configured images (daemon startup).
func (p *poolImpl) RefillAll(ctx context.Context) {
	for image, count := range p.target {
		if err := p.Refill(ctx, image, count); err != nil && ctx.Err() == nil && p.config.Logger != nil {
			p.config.Logger.Warn("pool refill all: failed", "image", image, "error", err)
		}
	}
}
