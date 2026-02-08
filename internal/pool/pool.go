package pool

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
)

// Pool maintains pre-warmed containers ready for instant use.
type Pool struct {
	cfg     *config.Config
	docker  *docker.Client
	store   *store.Store
	logger  *slog.Logger
	pools   map[string]chan string // image -> channel of ready container IDs
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
}

// PoolConfig defines per-image pool sizes.
type PoolConfig struct {
	Image string
	Size  int
}

func New(cfg *config.Config, dc *docker.Client, st *store.Store, logger *slog.Logger) *Pool {
	return &Pool{
		cfg:    cfg,
		docker: dc,
		store:  st,
		logger: logger,
		pools:  make(map[string]chan string),
		stopCh: make(chan struct{}),
	}
}

// Start begins pre-warming containers in the background.
func (p *Pool) Start(ctx context.Context, poolConfigs []PoolConfig) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true

	// Initialize pools
	for _, pc := range poolConfigs {
		p.pools[pc.Image] = make(chan string, pc.Size)
	}
	p.mu.Unlock()

	p.logger.Info("starting container pool", "configs", poolConfigs)

	// Start refill workers for each image
	for _, pc := range poolConfigs {
		go p.refillWorker(ctx, pc.Image, pc.Size)
	}
}

// Stop shuts down the pool and cleans up pre-warmed containers.
func (p *Pool) Stop(ctx context.Context) {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)

	// Copy channel references while holding lock to avoid race with refill workers
	poolCopy := make(map[string]chan string)
	for image, ch := range p.pools {
		poolCopy[image] = ch
	}
	p.mu.Unlock()

	p.logger.Info("stopping container pool")

	// Drain and cleanup all pools (using our safe copy)
	for image, ch := range poolCopy {
		close(ch)
		for containerID := range ch {
			p.logger.Info("cleaning up pooled container", "image", image, "container", containerID[:12])
			p.docker.RemoveContainer(ctx, containerID, "pool-cleanup")
		}
	}
}

// Get retrieves a pre-warmed container from the pool, or creates a new one if pool is empty.
func (p *Pool) Get(ctx context.Context, image string) (string, bool) {
	p.mu.RLock()
	ch, exists := p.pools[image]
	p.mu.RUnlock()

	if !exists {
		return "", false // Pool not configured for this image
	}

	select {
	case containerID := <-ch:
		p.logger.Info("using pooled container", "image", image, "container", containerID[:12])
		return containerID, true
	default:
		// Pool empty - caller should create normally
		return "", false
	}
}

// Return returns a container to the pool (if there's space), or removes it.
func (p *Pool) Return(ctx context.Context, image, containerID string) {
	p.mu.RLock()
	ch, exists := p.pools[image]
	p.mu.RUnlock()

	if !exists {
		// Pool not configured - remove container
		p.docker.RemoveContainer(ctx, containerID, "pool-return")
		return
	}

	select {
	case ch <- containerID:
		p.logger.Info("returned container to pool", "image", image, "container", containerID[:12])
	default:
		// Pool full - remove excess container
		p.logger.Info("pool full, removing container", "image", image, "container", containerID[:12])
		p.docker.RemoveContainer(ctx, containerID, "pool-overflow")
	}
}

// refillWorker maintains the pool at target size.
func (p *Pool) refillWorker(ctx context.Context, image string, targetSize int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial fill
	p.refill(ctx, image, targetSize)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.refill(ctx, image, targetSize)
		}
	}
}

func (p *Pool) refill(ctx context.Context, image string, targetSize int) {
	p.mu.RLock()
	ch := p.pools[image]
	p.mu.RUnlock()

	current := len(ch)
	needed := targetSize - current

	if needed <= 0 {
		return
	}

	p.logger.Info("refilling pool", "image", image, "current", current, "target", targetSize, "creating", needed)

	for i := 0; i < needed; i++ {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
		}

		// Create container with pool labels
		sessionID := generatePoolSessionID()
		containerID, err := p.docker.CreateContainer(ctx, docker.CreateOpts{
			SessionID: sessionID,
			Image:     image,
			Defaults:  p.cfg.Defaults,
			Labels: map[string]string{
				"sandkasten.pool":      "true",
				"sandkasten.image":     image,
				"sandkasten.pooled_at": time.Now().Format(time.RFC3339),
			},
		})
		if err != nil {
			p.logger.Error("failed to create pooled container", "image", image, "error", err)
			time.Sleep(2 * time.Second) // Back off
			continue
		}

		// Add to pool (non-blocking)
		select {
		case ch <- containerID:
			p.logger.Info("created pooled container", "image", image, "container", containerID[:12])
		default:
			// Pool filled while we were creating - remove this one
			p.docker.RemoveContainer(ctx, containerID, "pool-excess")
		}
	}
}

func generatePoolSessionID() string {
	return "pool-" + time.Now().Format("20060102-150405")
}
