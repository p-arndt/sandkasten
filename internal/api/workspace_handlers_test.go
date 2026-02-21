package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleListWorkspaces_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("ListWorkspaces", mock.Anything).Return([]*session.WorkspaceInfo{
		{ID: "ws-1"},
		{ID: "ws-2"},
	}, nil)

	req := httptest.NewRequest("GET", "/v1/workspaces", nil)
	rec := httptest.NewRecorder()

	s.handleListWorkspaces(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	workspaces := result["workspaces"].([]any)
	assert.Len(t, workspaces, 2)
}

func TestHandleListWorkspaces_Error(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("ListWorkspaces", mock.Anything).Return(nil, fmt.Errorf("workspaces not enabled"))

	req := httptest.NewRequest("GET", "/v1/workspaces", nil)
	rec := httptest.NewRecorder()

	s.handleListWorkspaces(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleDeleteWorkspace_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("DeleteWorkspace", mock.Anything, "my-ws").Return(nil)

	req := httptest.NewRequest("DELETE", "/v1/workspaces/my-ws", nil)
	req.SetPathValue("id", "my-ws")
	rec := httptest.NewRecorder()

	s.handleDeleteWorkspace(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleDeleteWorkspace_Error(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("DeleteWorkspace", mock.Anything, "zz-nonexistent-ws").Return(fmt.Errorf("not found"))

	req := httptest.NewRequest("DELETE", "/v1/workspaces/nonexistent", nil)
	req.SetPathValue("id", "zz-nonexistent-ws")
	rec := httptest.NewRecorder()

	s.handleDeleteWorkspace(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleWriteWorkspaceFile_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("WriteWorkspaceFile", mock.Anything, "my-ws", "code.py", []byte("print(42)"), false).Return(nil)

	body := `{"path":"code.py","text":"print(42)"}`
	req := httptest.NewRequest("POST", "/v1/workspaces/my-ws/fs/write", strings.NewReader(body))
	req.SetPathValue("id", "my-ws")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleWriteWorkspaceFile(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.True(t, result["ok"].(bool))
}

func TestHandleWriteWorkspaceFile_MissingPath(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	body := `{"text":"hello"}`
	req := httptest.NewRequest("POST", "/v1/workspaces/my-ws/fs/write", strings.NewReader(body))
	req.SetPathValue("id", "my-ws")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleWriteWorkspaceFile(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleUploadWorkspaceFile_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "data.csv")
	require.NoError(t, err)
	_, _ = part.Write([]byte("a,b,c\n1,2,3"))
	require.NoError(t, w.Close())

	req := httptest.NewRequest("POST", "/v1/workspaces/my-project/fs/upload", &buf)
	req.SetPathValue("id", "my-project")
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	mockMgr.On("WriteWorkspaceFile", mock.Anything, "my-project", "data.csv", []byte("a,b,c\n1,2,3"), false).Return(nil)

	s.handleUploadWorkspaceFile(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var result map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.True(t, result["ok"].(bool))
	assert.Equal(t, []any{"data.csv"}, result["paths"])
}

func TestHandleUploadWorkspaceFile_NoFile(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.Close())

	req := httptest.NewRequest("POST", "/v1/workspaces/my-ws/fs/upload", &buf)
	req.SetPathValue("id", "my-ws")
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	s.handleUploadWorkspaceFile(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
