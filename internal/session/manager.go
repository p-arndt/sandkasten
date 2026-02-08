package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/pool"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/internal/workspace"
	"github.com/p-arndt/sandkasten/protocol"
)

type Manager struct {
	cfg       *config.Config
	store     *store.Store
	docker    *docker.Client
	pool      *pool.Pool
	workspace *workspace.Manager

	// Per-session mutexes to serialize exec calls.
	locks   map[string]*sync.Mutex
	locksMu sync.Mutex
}

func NewManager(cfg *config.Config, st *store.Store, dc *docker.Client, p *pool.Pool, ws *workspace.Manager) *Manager {
	return &Manager{
		cfg:       cfg,
		store:     st,
		docker:    dc,
		pool:      p,
		workspace: ws,
		locks:     make(map[string]*sync.Mutex),
	}
}

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

func (m *Manager) removeSessionLock(id string) {
	m.locksMu.Lock()
	defer m.locksMu.Unlock()
	delete(m.locks, id)
}

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

func (m *Manager) Create(ctx context.Context, opts CreateOpts) (*SessionInfo, error) {
	image := opts.Image
	if image == "" {
		image = m.cfg.DefaultImage
	}

	if !m.isImageAllowed(image) {
		return nil, fmt.Errorf("image not allowed: %s", image)
	}

	ttl := opts.TTLSeconds
	if ttl <= 0 {
		ttl = m.cfg.SessionTTLSeconds
	}

	workspaceID := opts.WorkspaceID

	// Create persistent workspace if needed
	if workspaceID != "" && m.cfg.Workspace.Enabled {
		exists, err := m.workspace.Exists(ctx, "sandkasten-ws-"+workspaceID)
		if err != nil {
			return nil, fmt.Errorf("check workspace: %w", err)
		}
		if !exists {
			if err := m.workspace.Create(ctx, "sandkasten-ws-"+workspaceID, map[string]string{
				"sandkasten.workspace_id": workspaceID,
			}); err != nil {
				return nil, fmt.Errorf("create workspace: %w", err)
			}
		}
	}

	sessionID := uuid.New().String()[:12]
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)

	var containerID string
	var err error
	fromPool := false

	// Try to get from pool first
	if m.pool != nil {
		containerID, fromPool = m.pool.Get(ctx, image)
	}

	if !fromPool {
		// Create new container
		containerID, err = m.docker.CreateContainer(ctx, docker.CreateOpts{
			SessionID:   sessionID,
			Image:       image,
			Defaults:    m.cfg.Defaults,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return nil, fmt.Errorf("create container: %w", err)
		}
	}

	sess := &store.Session{
		ID:           sessionID,
		Image:        image,
		ContainerID:  containerID,
		Status:       "running",
		Cwd:          "/workspace",
		WorkspaceID:  workspaceID,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastActivity: now,
	}

	if err := m.store.CreateSession(sess); err != nil {
		m.docker.RemoveContainer(ctx, containerID, sessionID)
		return nil, fmt.Errorf("store session: %w", err)
	}

	// Give the runner a moment to start (skip if from pool)
	if !fromPool {
		time.Sleep(500 * time.Millisecond)
	}

	return &SessionInfo{
		ID:          sessionID,
		Image:       image,
		Status:      "running",
		Cwd:         "/workspace",
		WorkspaceID: workspaceID,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}, nil
}

func (m *Manager) Get(ctx context.Context, id string) (*SessionInfo, error) {
	sess, err := m.store.GetSession(id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return &SessionInfo{
		ID:          sess.ID,
		Image:       sess.Image,
		Status:      sess.Status,
		Cwd:         sess.Cwd,
		WorkspaceID: sess.WorkspaceID,
		CreatedAt:   sess.CreatedAt,
		ExpiresAt:   sess.ExpiresAt,
	}, nil
}

func (m *Manager) List(ctx context.Context) ([]SessionInfo, error) {
	sessions, err := m.store.ListSessions()
	if err != nil {
		return nil, err
	}
	result := make([]SessionInfo, len(sessions))
	for i, s := range sessions {
		result[i] = SessionInfo{
			ID:          s.ID,
			Image:       s.Image,
			Status:      s.Status,
			Cwd:         s.Cwd,
			WorkspaceID: s.WorkspaceID,
			CreatedAt:   s.CreatedAt,
			ExpiresAt:   s.ExpiresAt,
		}
	}
	return result, nil
}

func (m *Manager) Exec(ctx context.Context, sessionID, cmd string, timeoutMs int) (*ExecResult, error) {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if sess.Status != "running" {
		return nil, fmt.Errorf("session not running: %s (status=%s)", sessionID, sess.Status)
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, fmt.Errorf("session expired: %s", sessionID)
	}

	// Enforce max timeout.
	if timeoutMs <= 0 || timeoutMs > m.cfg.Defaults.MaxExecTimeoutMs {
		timeoutMs = m.cfg.Defaults.MaxExecTimeoutMs
	}

	// Serialize exec per session.
	mu := m.sessionLock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	execID := uuid.New().String()[:8]

	resp, err := m.docker.ExecRunner(ctx, sess.ContainerID, protocol.Request{
		ID:        execID,
		Type:      protocol.RequestExec,
		Cmd:       cmd,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		return nil, fmt.Errorf("exec: %w", err)
	}

	if resp.Type == protocol.ResponseError {
		return nil, fmt.Errorf("runner error: %s", resp.Error)
	}

	// Update session activity + extend lease.
	newExpiry := time.Now().UTC().Add(time.Duration(m.cfg.SessionTTLSeconds) * time.Second)
	cwd := resp.Cwd
	if cwd == "" {
		cwd = sess.Cwd
	}
	m.store.UpdateSessionActivity(sessionID, cwd, newExpiry)

	return &ExecResult{
		ExitCode:   resp.ExitCode,
		Cwd:        cwd,
		Output:     resp.Output,
		Truncated:  resp.Truncated,
		DurationMs: resp.DurationMs,
	}, nil
}

// ExecStream executes a command and streams output chunks via channel.
// The channel is closed when execution completes or errors.
// The final chunk has Done=true and includes exit code, cwd, duration.
func (m *Manager) ExecStream(ctx context.Context, sessionID, cmd string, timeoutMs int, chunkChan chan<- ExecChunk) error {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return err
	}
	if sess == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if sess.Status != "running" {
		return fmt.Errorf("session not running: %s (status=%s)", sessionID, sess.Status)
	}
	if time.Now().After(sess.ExpiresAt) {
		return fmt.Errorf("session expired: %s", sessionID)
	}

	// Enforce max timeout
	if timeoutMs <= 0 || timeoutMs > m.cfg.Defaults.MaxExecTimeoutMs {
		timeoutMs = m.cfg.Defaults.MaxExecTimeoutMs
	}

	// Serialize exec per session
	mu := m.sessionLock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	execID := uuid.New().String()[:8]
	startTime := time.Now()

	// For now, use blocking exec and send as single chunk
	// TODO: Implement true streaming at runner level
	resp, err := m.docker.ExecRunner(ctx, sess.ContainerID, protocol.Request{
		ID:        execID,
		Type:      protocol.RequestExec,
		Cmd:       cmd,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	if resp.Type == protocol.ResponseError {
		return fmt.Errorf("runner error: %s", resp.Error)
	}

	// Update session activity + extend lease
	newExpiry := time.Now().UTC().Add(time.Duration(m.cfg.SessionTTLSeconds) * time.Second)
	cwd := resp.Cwd
	if cwd == "" {
		cwd = sess.Cwd
	}
	m.store.UpdateSessionActivity(sessionID, cwd, newExpiry)

	// Send final chunk with complete output
	chunkChan <- ExecChunk{
		Output:     resp.Output,
		Timestamp:  startTime.UnixMilli(),
		ExitCode:   resp.ExitCode,
		Cwd:        cwd,
		DurationMs: resp.DurationMs,
		Done:       true,
	}

	return nil
}

func (m *Manager) Write(ctx context.Context, sessionID, path string, content []byte, isBase64 bool) error {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return err
	}
	if sess == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if sess.Status != "running" {
		return fmt.Errorf("session not running: %s", sessionID)
	}

	req := protocol.Request{
		ID:   uuid.New().String()[:8],
		Type: protocol.RequestWrite,
		Path: path,
	}
	if isBase64 {
		req.ContentBase64 = string(content)
	} else {
		req.Text = string(content)
	}

	resp, err := m.docker.ExecRunner(ctx, sess.ContainerID, req)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if resp.Type == protocol.ResponseError {
		return fmt.Errorf("runner error: %s", resp.Error)
	}

	// Extend lease.
	newExpiry := time.Now().UTC().Add(time.Duration(m.cfg.SessionTTLSeconds) * time.Second)
	m.store.UpdateSessionActivity(sessionID, sess.Cwd, newExpiry)

	return nil
}

func (m *Manager) Read(ctx context.Context, sessionID, path string, maxBytes int) (string, bool, error) {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return "", false, err
	}
	if sess == nil {
		return "", false, fmt.Errorf("session not found: %s", sessionID)
	}
	if sess.Status != "running" {
		return "", false, fmt.Errorf("session not running: %s", sessionID)
	}

	resp, err := m.docker.ExecRunner(ctx, sess.ContainerID, protocol.Request{
		ID:       uuid.New().String()[:8],
		Type:     protocol.RequestRead,
		Path:     path,
		MaxBytes: maxBytes,
	})
	if err != nil {
		return "", false, fmt.Errorf("read: %w", err)
	}
	if resp.Type == protocol.ResponseError {
		return "", false, fmt.Errorf("runner error: %s", resp.Error)
	}

	// Extend lease.
	newExpiry := time.Now().UTC().Add(time.Duration(m.cfg.SessionTTLSeconds) * time.Second)
	m.store.UpdateSessionActivity(sessionID, sess.Cwd, newExpiry)

	return resp.ContentBase64, resp.Truncated, nil
}

func (m *Manager) Destroy(ctx context.Context, sessionID string) error {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return err
	}
	if sess == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	m.docker.RemoveContainer(ctx, sess.ContainerID, sessionID)
	m.store.UpdateSessionStatus(sessionID, "destroyed")
	m.removeSessionLock(sessionID)

	return nil
}

// Store returns the underlying store (used by reaper).
func (m *Manager) Store() *store.Store {
	return m.store
}

// Docker returns the underlying docker client (used by reaper).
func (m *Manager) Docker() *docker.Client {
	return m.docker
}

func (m *Manager) isImageAllowed(image string) bool {
	if len(m.cfg.AllowedImages) == 0 {
		return true
	}
	for _, allowed := range m.cfg.AllowedImages {
		if allowed == image {
			return true
		}
	}
	return false
}

// Workspace management methods

func (m *Manager) ListWorkspaces(ctx context.Context) ([]*workspace.Workspace, error) {
	if !m.cfg.Workspace.Enabled {
		return nil, fmt.Errorf("workspaces not enabled")
	}
	return m.workspace.List(ctx)
}

func (m *Manager) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	if !m.cfg.Workspace.Enabled {
		return fmt.Errorf("workspaces not enabled")
	}
	return m.workspace.Delete(ctx, "sandkasten-ws-"+workspaceID)
}
