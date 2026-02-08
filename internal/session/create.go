package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/p-arndt/sandkasten/internal/docker"
	"github.com/p-arndt/sandkasten/internal/store"
)

func (m *Manager) Create(ctx context.Context, opts CreateOpts) (*SessionInfo, error) {
	image := m.resolveImage(opts.Image)
	if !m.isImageAllowed(image) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidImage, image)
	}

	ttl := m.resolveTTL(opts.TTLSeconds)
	workspaceID := opts.WorkspaceID

	// Create persistent workspace if requested
	if err := m.ensureWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Generate session ID and timestamps
	sessionID := uuid.New().String()[:12]
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)

	// Create or acquire container
	containerID, fromPool, err := m.acquireContainer(ctx, sessionID, image, workspaceID)
	if err != nil {
		return nil, err
	}

	// Persist session to store
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

	// Wait for runner to be ready (skip if from pool)
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

// resolveImage returns the image to use, applying default if needed.
func (m *Manager) resolveImage(image string) string {
	if image == "" {
		return m.cfg.DefaultImage
	}
	return image
}

// resolveTTL returns the TTL to use, applying default if needed.
func (m *Manager) resolveTTL(ttl int) int {
	if ttl <= 0 {
		return m.cfg.SessionTTLSeconds
	}
	return ttl
}

// ensureWorkspace creates a persistent workspace if needed.
func (m *Manager) ensureWorkspace(ctx context.Context, workspaceID string) error {
	if workspaceID == "" || !m.cfg.Workspace.Enabled {
		return nil
	}

	volumeName := "sandkasten-ws-" + workspaceID
	exists, err := m.workspace.Exists(ctx, volumeName)
	if err != nil {
		return fmt.Errorf("check workspace: %w", err)
	}

	if !exists {
		if err := m.workspace.Create(ctx, volumeName, map[string]string{
			"sandkasten.workspace_id": workspaceID,
		}); err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}
	}

	return nil
}

// acquireContainer gets a container from pool or creates a new one.
func (m *Manager) acquireContainer(ctx context.Context, sessionID, image, workspaceID string) (containerID string, fromPool bool, err error) {
	// Try to get from pool first
	if m.pool != nil {
		containerID, fromPool = m.pool.Get(ctx, image)
		if fromPool {
			return containerID, true, nil
		}
	}

	// Create new container
	containerID, err = m.docker.CreateContainer(ctx, docker.CreateOpts{
		SessionID:   sessionID,
		Image:       image,
		Defaults:    m.cfg.Defaults,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return "", false, fmt.Errorf("create container: %w", err)
	}

	return containerID, false, nil
}
