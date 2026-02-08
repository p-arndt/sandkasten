package session

import (
	"errors"
	"sync"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
)

// Sentinel errors for structured error handling
var (
	ErrNotFound     = errors.New("session not found")
	ErrExpired      = errors.New("session expired")
	ErrInvalidImage = errors.New("image not allowed")
	ErrTimeout      = errors.New("command timeout")
	ErrNotRunning   = errors.New("session not running")
)

type Manager struct {
	cfg       *config.Config
	store     SessionStore
	docker    DockerClient
	pool      ContainerPool
	workspace WorkspaceManager

	// Per-session mutexes to serialize exec calls
	locks   map[string]*sync.Mutex
	locksMu sync.Mutex
}

func NewManager(cfg *config.Config, st SessionStore, dc DockerClient, p ContainerPool, ws WorkspaceManager) *Manager {
	return &Manager{
		cfg:       cfg,
		store:     st,
		docker:    dc,
		pool:      p,
		workspace: ws,
		locks:     make(map[string]*sync.Mutex),
	}
}

// sessionLock returns or creates a mutex for the given session ID.
func (m *Manager) sessionLock(id string) *sync.Mutex {
	m.locksMu.Lock()
	defer m.locksMu.Unlock()
	mu, ok := m.locks[id]
	if !ok {
		mu = &sync.Mutex{}
		m.locks[id] = mu
	}
	return mu
}

// removeSessionLock removes the mutex for a destroyed session.
func (m *Manager) removeSessionLock(id string) {
	m.locksMu.Lock()
	defer m.locksMu.Unlock()
	delete(m.locks, id)
}

// CleanupSessionLock removes the mutex for a session (used by reaper).
func (m *Manager) CleanupSessionLock(id string) {
	m.removeSessionLock(id)
}

// isImageAllowed checks if an image is in the allowed list.
func (m *Manager) isImageAllowed(image string) bool {
	if len(m.cfg.AllowedImages) == 0 {
		return true // No restrictions
	}
	for _, allowed := range m.cfg.AllowedImages {
		if allowed == image {
			return true
		}
	}
	return false
}

// Store returns the underlying store.
func (m *Manager) Store() SessionStore {
	return m.store
}

// Docker returns the underlying docker client.
func (m *Manager) Docker() DockerClient {
	return m.docker
}

// Types

type CreateOpts struct {
	Image       string
	TTLSeconds  int
	WorkspaceID string // optional persistent workspace
}

type SessionInfo struct {
	ID          string    `json:"id"`
	Image       string    `json:"image"`
	Status      string    `json:"status"`
	Cwd         string    `json:"cwd"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type ExecResult struct {
	ExitCode   int    `json:"exit_code"`
	Cwd        string `json:"cwd"`
	Output     string `json:"output"`
	Truncated  bool   `json:"truncated"`
	DurationMs int64  `json:"duration_ms"`
}

type ExecChunk struct {
	Output     string `json:"output"`      // output chunk
	Timestamp  int64  `json:"timestamp"`   // unix timestamp ms
	ExitCode   int    `json:"exit_code"`   // only set on final chunk
	Cwd        string `json:"cwd"`         // only set on final chunk
	DurationMs int64  `json:"duration_ms"` // only set on final chunk
	Done       bool   `json:"done"`        // true on final chunk
}
