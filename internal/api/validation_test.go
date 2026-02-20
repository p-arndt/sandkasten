package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateSessionRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     createSessionRequest
		wantErr string
	}{
		{
			name: "valid empty request",
			req:  createSessionRequest{},
		},
		{
			name: "valid with all fields",
			req: createSessionRequest{
				Image:       "sandbox-runtime:python",
				TTLSeconds:  600,
				WorkspaceID: "my-workspace",
			},
		},
		{
			name:    "negative TTL",
			req:     createSessionRequest{TTLSeconds: -1},
			wantErr: "ttl_seconds must be non-negative",
		},
		{
			name:    "TTL too large",
			req:     createSessionRequest{TTLSeconds: 86401},
			wantErr: "ttl_seconds must not exceed 86400",
		},
		{
			name:    "workspace ID too short",
			req:     createSessionRequest{WorkspaceID: "a"},
			wantErr: "workspace_id must be at least 2 characters",
		},
		{
			name:    "workspace ID too long",
			req:     createSessionRequest{WorkspaceID: "a" + string(make([]byte, 64))},
			wantErr: "workspace_id must not exceed 64 characters",
		},
		{
			name:    "workspace ID with uppercase",
			req:     createSessionRequest{WorkspaceID: "MyWorkspace"},
			wantErr: "workspace_id must contain only lowercase",
		},
		{
			name:    "workspace ID starts with hyphen",
			req:     createSessionRequest{WorkspaceID: "-workspace"},
			wantErr: "workspace_id must contain only lowercase",
		},
		{
			name:    "workspace ID ends with hyphen",
			req:     createSessionRequest{WorkspaceID: "workspace-"},
			wantErr: "workspace_id must contain only lowercase",
		},
		{
			name: "valid workspace with hyphens",
			req:  createSessionRequest{WorkspaceID: "my-cool-workspace-123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateSessionRequest(tt.req)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateExecRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     execRequest
		wantErr string
	}{
		{
			name: "valid",
			req:  execRequest{Cmd: "echo hello"},
		},
		{
			name: "valid with timeout",
			req:  execRequest{Cmd: "sleep 5", TimeoutMs: 10000},
		},
		{
			name:    "empty cmd",
			req:     execRequest{},
			wantErr: "cmd is required",
		},
		{
			name:    "negative timeout",
			req:     execRequest{Cmd: "ls", TimeoutMs: -1},
			wantErr: "timeout_ms must be non-negative",
		},
		{
			name:    "timeout too large",
			req:     execRequest{Cmd: "ls", TimeoutMs: 600001},
			wantErr: "timeout_ms must not exceed 600000",
		},
		{
			name:    "cmd too large",
			req:     execRequest{Cmd: strings.Repeat("x", 1024*1024+1)},
			wantErr: "cmd is too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExecRequest(tt.req)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWriteRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     writeRequest
		wantErr string
	}{
		{
			name: "valid text",
			req:  writeRequest{Path: "/workspace/test.py", Text: "print('hello')"},
		},
		{
			name: "valid base64",
			req:  writeRequest{Path: "/workspace/test.py", ContentBase64: "cHJpbnQoJ2hlbGxvJyk="},
		},
		{
			name:    "missing path",
			req:     writeRequest{Text: "hello"},
			wantErr: "path is required",
		},
		{
			name:    "both text and base64",
			req:     writeRequest{Path: "/test", Text: "hello", ContentBase64: "aGVsbG8="},
			wantErr: "provide either 'text' or 'content_base64', not both",
		},
		{
			name:    "neither text nor base64",
			req:     writeRequest{Path: "/test"},
			wantErr: "either 'text' or 'content_base64' must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWriteRequest(tt.req)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateReadRequest(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		maxBytes int
		wantErr  string
	}{
		{
			name: "valid",
			path: "/workspace/test.py",
		},
		{
			name:     "valid with max_bytes",
			path:     "/workspace/test.py",
			maxBytes: 1024,
		},
		{
			name:    "missing path",
			path:    "",
			wantErr: "path query parameter is required",
		},
		{
			name:     "negative max_bytes",
			path:     "/test",
			maxBytes: -1,
			wantErr:  "max_bytes must be non-negative",
		},
		{
			name:     "max_bytes too large",
			path:     "/test",
			maxBytes: 100*1024*1024 + 1,
			wantErr:  "max_bytes must not exceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReadRequest(tt.path, tt.maxBytes)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
