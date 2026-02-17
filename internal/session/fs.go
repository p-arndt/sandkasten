package session

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/p-arndt/sandkasten/protocol"
)

func (m *Manager) Write(ctx context.Context, sessionID, path string, content []byte, isBase64 bool) error {
	sess, err := m.validateSession(sessionID)
	if err != nil {
		return err
	}

	req := buildWriteRequest(path, content, isBase64)

	resp, err := m.runtime.Exec(ctx, sess.ID, req)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if resp.Type == protocol.ResponseError {
		return fmt.Errorf("runner error: %s", resp.Error)
	}

	m.extendSessionLease(sessionID, sess.Cwd)
	return nil
}

func (m *Manager) Read(ctx context.Context, sessionID, path string, maxBytes int) (string, bool, error) {
	sess, err := m.validateSession(sessionID)
	if err != nil {
		return "", false, err
	}

	req := buildReadRequest(path, maxBytes)

	resp, err := m.runtime.Exec(ctx, sess.ID, req)
	if err != nil {
		return "", false, fmt.Errorf("read: %w", err)
	}
	if resp.Type == protocol.ResponseError {
		return "", false, fmt.Errorf("runner error: %s", resp.Error)
	}

	m.extendSessionLease(sessionID, sess.Cwd)
	return resp.ContentBase64, resp.Truncated, nil
}

// buildWriteRequest creates a write request with content in the correct format.
func buildWriteRequest(path string, content []byte, isBase64 bool) protocol.Request {
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

	return req
}

// buildReadRequest creates a read request.
func buildReadRequest(path string, maxBytes int) protocol.Request {
	return protocol.Request{
		ID:       uuid.New().String()[:8],
		Type:     protocol.RequestRead,
		Path:     path,
		MaxBytes: maxBytes,
	}
}
