package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleExec_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Exec", mock.Anything, "s1", "echo hello", 5000).Return(&session.ExecResult{
		ExitCode:   0,
		Cwd:        "/workspace",
		Output:     "hello\n",
		DurationMs: 42,
	}, nil)

	body := `{"cmd":"echo hello","timeout_ms":5000}`
	req := httptest.NewRequest("POST", "/v1/sessions/s1/exec", strings.NewReader(body))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleExec(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result session.ExecResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "hello\n", result.Output)
}

func TestHandleExec_EmptyCmd(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	body := `{"cmd":""}`
	req := httptest.NewRequest("POST", "/v1/sessions/s1/exec", strings.NewReader(body))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleExec(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleExec_NotFound(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Exec", mock.Anything, "nonexistent", "ls", 0).Return(nil, fmt.Errorf("%w: nonexistent", session.ErrNotFound))

	body := `{"cmd":"ls"}`
	req := httptest.NewRequest("POST", "/v1/sessions/nonexistent/exec", strings.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleExec(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleExec_InvalidJSON(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	req := httptest.NewRequest("POST", "/v1/sessions/s1/exec", strings.NewReader("{bad"))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleExec(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleExecStream_InvalidJSON(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	req := httptest.NewRequest("POST", "/v1/sessions/s1/exec/stream", strings.NewReader("{bad"))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleExecStream(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleExecStream_EmptyCmd(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	body := `{"cmd":""}`
	req := httptest.NewRequest("POST", "/v1/sessions/s1/exec/stream", strings.NewReader(body))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleExecStream(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
