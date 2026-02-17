package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	storemod "github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/protocol"
)

func (m *Manager) Exec(ctx context.Context, sessionID, cmd string, timeoutMs int) (*ExecResult, error) {
	sess, err := m.validateSession(sessionID)
	if err != nil {
		return nil, err
	}

	timeoutMs = m.enforceMaxTimeout(timeoutMs)

	// Serialize exec per session
	mu := m.sessionLock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	execID := uuid.New().String()[:8]

	resp, err := m.runtime.Exec(ctx, sess.ID, protocol.Request{
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

	cwd := m.resolveCwd(resp.Cwd, sess.Cwd)
	m.extendSessionLease(sessionID, cwd)

	return &ExecResult{
		ExitCode:   resp.ExitCode,
		Cwd:        cwd,
		Output:     resp.Output,
		Truncated:  resp.Truncated,
		DurationMs: resp.DurationMs,
	}, nil
}

func (m *Manager) ExecStream(ctx context.Context, sessionID, cmd string, timeoutMs int, chunkChan chan<- ExecChunk) error {
	sess, err := m.validateSession(sessionID)
	if err != nil {
		return err
	}

	timeoutMs = m.enforceMaxTimeout(timeoutMs)

	// Serialize exec per session
	mu := m.sessionLock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	execID := uuid.New().String()[:8]
	startTime := time.Now()

	resp, err := m.runtime.Exec(ctx, sess.ID, protocol.Request{
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

	cwd := m.resolveCwd(resp.Cwd, sess.Cwd)
	m.extendSessionLease(sessionID, cwd)

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

// validateSession checks if a session exists and is valid for execution.
func (m *Manager) validateSession(sessionID string) (*storemod.Session, error) {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, sessionID)
	}
	if sess.Status != "running" {
		return nil, fmt.Errorf("%w: %s (status=%s)", ErrNotRunning, sessionID, sess.Status)
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, fmt.Errorf("%w: %s", ErrExpired, sessionID)
	}
	return sess, nil
}

// enforceMaxTimeout ensures timeout doesn't exceed configured maximum.
func (m *Manager) enforceMaxTimeout(timeoutMs int) int {
	if timeoutMs <= 0 || timeoutMs > m.cfg.Defaults.MaxExecTimeoutMs {
		return m.cfg.Defaults.MaxExecTimeoutMs
	}
	return timeoutMs
}

// resolveCwd returns new cwd if present, otherwise falls back to current.
func (m *Manager) resolveCwd(newCwd, currentCwd string) string {
	if newCwd != "" {
		return newCwd
	}
	return currentCwd
}

// extendSessionLease updates session activity and extends TTL.
func (m *Manager) extendSessionLease(sessionID, cwd string) {
	newExpiry := time.Now().UTC().Add(time.Duration(m.cfg.SessionTTLSeconds) * time.Second)
	m.store.UpdateSessionActivity(sessionID, cwd, newExpiry)
}
