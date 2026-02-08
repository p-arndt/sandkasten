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

func TestHandleWrite_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Write", mock.Anything, "s1", "/workspace/test.py", []byte("print('hello')"), false).Return(nil)

	body := `{"path":"/workspace/test.py","text":"print('hello')"}`
	req := httptest.NewRequest("POST", "/v1/sessions/s1/fs/write", strings.NewReader(body))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleWrite(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleWrite_MissingPath(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	body := `{"text":"hello"}`
	req := httptest.NewRequest("POST", "/v1/sessions/s1/fs/write", strings.NewReader(body))
	req.SetPathValue("id", "s1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleWrite(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleWrite_NotFound(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Write", mock.Anything, "nonexistent", "/test", mock.Anything, false).Return(fmt.Errorf("%w: nonexistent", session.ErrNotFound))

	body := `{"path":"/test","text":"hello"}`
	req := httptest.NewRequest("POST", "/v1/sessions/nonexistent/fs/write", strings.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleWrite(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleRead_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Read", mock.Anything, "s1", "/workspace/test.py", 0).Return("cHJpbnQoJ2hlbGxvJyk=", false, nil)

	req := httptest.NewRequest("GET", "/v1/sessions/s1/fs/read?path=/workspace/test.py", nil)
	req.SetPathValue("id", "s1")
	rec := httptest.NewRecorder()

	s.handleRead(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Equal(t, "cHJpbnQoJ2hlbGxvJyk=", result["content_base64"])
	assert.Equal(t, false, result["truncated"])
}

func TestHandleRead_MissingPath(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	req := httptest.NewRequest("GET", "/v1/sessions/s1/fs/read", nil)
	req.SetPathValue("id", "s1")
	rec := httptest.NewRecorder()

	s.handleRead(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleRead_WithMaxBytes(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Read", mock.Anything, "s1", "/test", 1024).Return("data", false, nil)

	req := httptest.NewRequest("GET", "/v1/sessions/s1/fs/read?path=/test&max_bytes=1024", nil)
	req.SetPathValue("id", "s1")
	rec := httptest.NewRecorder()

	s.handleRead(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleRead_InvalidMaxBytes(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	req := httptest.NewRequest("GET", "/v1/sessions/s1/fs/read?path=/test&max_bytes=abc", nil)
	req.SetPathValue("id", "s1")
	rec := httptest.NewRecorder()

	s.handleRead(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
