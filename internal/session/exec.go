package session

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	storemod "github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/protocol"
)

func (m *Manager) Exec(ctx context.Context, sessionID, cmd string, timeoutMs int, rawOutput bool) (*ExecResult, error) {
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

	execReq, err := m.prepareExecRequest(ctx, sess.ID, execID, cmd, timeoutMs, rawOutput)
	if err != nil {
		return nil, err
	}

	resp, err := m.runtime.Exec(ctx, sess.ID, execReq)
	if err != nil {
		return nil, fmt.Errorf("exec: %w", err)
	}

	if resp.Type == protocol.ResponseError {
		return nil, fmt.Errorf("runner error: %s", resp.Error)
	}
	if resp.ExitCode == -1 && strings.HasPrefix(resp.Output, "timeout:") {
		return nil, fmt.Errorf("%w: %s", ErrTimeout, resp.Output)
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

func (m *Manager) ExecStream(ctx context.Context, sessionID, cmd string, timeoutMs int, rawOutput bool, chunkChan chan<- ExecChunk) error {
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

	execReq, err := m.prepareExecRequest(ctx, sess.ID, execID, cmd, timeoutMs, rawOutput)
	if err != nil {
		return err
	}

	resp, err := m.runtime.Exec(ctx, sess.ID, execReq)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	if resp.Type == protocol.ResponseError {
		return fmt.Errorf("runner error: %s", resp.Error)
	}
	if resp.ExitCode == -1 && strings.HasPrefix(resp.Output, "timeout:") {
		return fmt.Errorf("%w: %s", ErrTimeout, resp.Output)
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

func (m *Manager) prepareExecRequest(ctx context.Context, sessionID, execID, cmd string, timeoutMs int, rawOutput bool) (protocol.Request, error) {
	if len(cmd) <= protocol.MaxExecInlineCmdBytes {
		return protocol.Request{
			ID:        execID,
			Type:      protocol.RequestExec,
			Cmd:       cmd,
			TimeoutMs: timeoutMs,
			RawOutput: rawOutput,
		}, nil
	}

	scriptPath := fmt.Sprintf(".sandkasten/exec-%s.sh", execID)
	encoded := base64.StdEncoding.EncodeToString([]byte(cmd))
	writeReq := buildWriteRequest(scriptPath, []byte(encoded), true)
	writeResp, err := m.runtime.Exec(ctx, sessionID, writeReq)
	if err != nil {
		return protocol.Request{}, fmt.Errorf("stage exec script: %w", err)
	}
	if writeResp.Type == protocol.ResponseError {
		return protocol.Request{}, fmt.Errorf("stage exec script: runner error: %s", writeResp.Error)
	}

	absScriptPath := "/workspace/" + scriptPath
	quotedPath := shellSingleQuote(absScriptPath)
	stagedCmd := fmt.Sprintf("bash %s; __sandkasten_rc=$?; rm -f %s; exit $__sandkasten_rc", quotedPath, quotedPath)

	return protocol.Request{
		ID:        execID,
		Type:      protocol.RequestExec,
		Cmd:       stagedCmd,
		TimeoutMs: timeoutMs,
		RawOutput: rawOutput,
	}, nil
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
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
