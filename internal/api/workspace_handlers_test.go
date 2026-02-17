package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	mockMgr.On("DeleteWorkspace", mock.Anything, "nonexistent").Return(fmt.Errorf("not found"))

	req := httptest.NewRequest("DELETE", "/v1/workspaces/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	rec := httptest.NewRecorder()

	s.handleDeleteWorkspace(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
