package api

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/p-arndt/sandkasten/protocol"
)

var (
	// sessionIDPattern matches valid session IDs: first 12 chars of UUID (8 hex + "-" + 3 hex, as in session.Manager Create).
	// Prevents path traversal when id is used in filepath.Join(dataDir, "sessions", id).
	sessionIDPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{3}$`)

	// workspaceIDPattern matches valid workspace IDs: lowercase letters, numbers, hyphens
	// Prevents path traversal when id is used in filepath.Join(dataDir, "workspaces", id).
	workspaceIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
)

// ValidateSessionID returns an error if id is not a valid session ID format.
func ValidateSessionID(id string) error {
	if id == "" {
		return fmt.Errorf("session id is required")
	}
	if len(id) > 64 {
		return fmt.Errorf("session id too long")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid session id")
	}
	if !sessionIDPattern.MatchString(id) {
		return fmt.Errorf("invalid session id format")
	}
	return nil
}

// ValidateWorkspaceID returns an error if id is not a valid workspace ID format.
// Accepts both short form (e.g. "my-ws") and legacy prefix form (e.g. "sandkasten-ws-my-ws").
func ValidateWorkspaceID(id string) error {
	if id == "" {
		return fmt.Errorf("workspace id is required")
	}
	short := strings.TrimPrefix(id, protocol.WorkspaceVolumePrefix)
	if short != "" {
		id = short
	}
	if len(id) < 2 {
		return fmt.Errorf("workspace id must be at least 2 characters")
	}
	if len(id) > 64 {
		return fmt.Errorf("workspace id must not exceed 64 characters")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid workspace id")
	}
	if !workspaceIDPattern.MatchString(id) {
		return fmt.Errorf("workspace id must contain only lowercase letters, numbers, and hyphens, and cannot start or end with a hyphen")
	}
	return nil
}

// validateCreateSessionRequest validates session creation parameters
func validateCreateSessionRequest(req createSessionRequest) error {
	// Validate TTL
	if req.TTLSeconds < 0 {
		return fmt.Errorf("ttl_seconds must be non-negative")
	}
	if req.TTLSeconds > 86400 {
		return fmt.Errorf("ttl_seconds must not exceed 86400 (24 hours)")
	}

	// Validate workspace ID format if provided
	if req.WorkspaceID != "" {
		if len(req.WorkspaceID) < 2 {
			return fmt.Errorf("workspace_id must be at least 2 characters")
		}
		if len(req.WorkspaceID) > 64 {
			return fmt.Errorf("workspace_id must not exceed 64 characters")
		}
		if !workspaceIDPattern.MatchString(req.WorkspaceID) {
			return fmt.Errorf("workspace_id must contain only lowercase letters, numbers, and hyphens, and cannot start or end with a hyphen")
		}
	}

	return nil
}

// validateExecRequest validates command execution parameters
func validateExecRequest(req execRequest) error {
	if req.Cmd == "" {
		return fmt.Errorf("cmd is required")
	}

	if req.TimeoutMs < 0 {
		return fmt.Errorf("timeout_ms must be non-negative")
	}
	if req.TimeoutMs > 600000 {
		return fmt.Errorf("timeout_ms must not exceed 600000 (10 minutes)")
	}

	return nil
}

// validateWriteRequest validates file write parameters
func validateWriteRequest(req writeRequest) error {
	if req.Path == "" {
		return fmt.Errorf("path is required")
	}

	// Check that either text or content_base64 is provided, but not both
	if req.Text != "" && req.ContentBase64 != "" {
		return fmt.Errorf("provide either 'text' or 'content_base64', not both")
	}

	if req.Text == "" && req.ContentBase64 == "" {
		return fmt.Errorf("either 'text' or 'content_base64' must be provided")
	}

	return nil
}

// validateReadRequest validates file read parameters
func validateReadRequest(path string, maxBytes int) error {
	if path == "" {
		return fmt.Errorf("path query parameter is required")
	}

	if maxBytes < 0 {
		return fmt.Errorf("max_bytes must be non-negative")
	}
	if maxBytes > 100*1024*1024 {
		return fmt.Errorf("max_bytes must not exceed 104857600 (100MB)")
	}

	return nil
}
