package session

import (
	"context"
	"fmt"

	"github.com/p-arndt/sandkasten/protocol"
)

func (m *Manager) Get(ctx context.Context, id string) (*SessionInfo, error) {
	sess, err := m.store.GetSession(id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
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

func (m *Manager) GetStats(ctx context.Context, id string) (*protocol.SessionStats, error) {
	sess, err := m.store.GetSession(id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m.runtime.Stats(ctx, sess.ID)
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

func (m *Manager) Destroy(ctx context.Context, sessionID string) error {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return err
	}
	if sess == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	_ = m.store.UpdateSessionStatus(sessionID, "destroying")
	_ = m.runtime.Destroy(ctx, sessionID)
	_ = m.store.UpdateSessionStatus(sessionID, "destroyed")
	m.removeSessionLock(sessionID)

	return nil
}
