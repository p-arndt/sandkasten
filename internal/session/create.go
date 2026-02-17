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
	if !m.isImageAllowed(image) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidImage, image)
	}

	ttl := m.resolveTTL(opts.TTLSeconds)
	workspaceID := opts.WorkspaceID

	if err := m.ensureWorkspace(ctx, workspaceID); err != nil {
		return nil, err
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
