package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testAPIServer(mgr SessionService) *Server {
	return &Server{
		cfg:     &config.Config{},
		manager: mgr,
		logger:  slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		mux:     http.NewServeMux(),
	}
}

func TestHandleCreateSession_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	now := time.Now().UTC()
	mockMgr.On("Create", mock.Anything, session.CreateOpts{
		Image:      "sandbox-runtime:python",
		TTLSeconds: 600,
	}).Return(&session.SessionInfo{
		ID:        "a1b2c3d4-e5f",
		Image:     "sandbox-runtime:python",
		Status:    "running",
		Cwd:       "/workspace",
		CreatedAt: now,
		ExpiresAt: now.Add(10 * time.Minute),
	}, nil)

	body := `{"image":"sandbox-runtime:python","ttl_seconds":600}`
	req := httptest.NewRequest("POST", "/v1/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleCreateSession(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var info session.SessionInfo
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&info))
	assert.Equal(t, "a1b2c3d4-e5f", info.ID)
	assert.Equal(t, "running", info.Status)
}

func TestHandleCreateSession_InvalidJSON(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	req := httptest.NewRequest("POST", "/v1/sessions", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleCreateSession(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateSession_ValidationError(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	body := `{"ttl_seconds":-1}`
	req := httptest.NewRequest("POST", "/v1/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleCreateSession(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateSession_ManagerError(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Create", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("%w: bad-image", session.ErrInvalidImage))

	body := `{"image":"bad-image"}`
	req := httptest.NewRequest("POST", "/v1/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handleCreateSession(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetSession_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	now := time.Now().UTC()
	mockMgr.On("Get", mock.Anything, "a1b2c3d4-e5f").Return(&session.SessionInfo{
		ID:     "a1b2c3d4-e5f",
		Image:  "sandbox-runtime:base",
		Status: "running",
	}, nil)

	req := httptest.NewRequest("GET", "/v1/sessions/a1b2c3d4-e5f", nil)
	req.SetPathValue("id", "a1b2c3d4-e5f")
	rec := httptest.NewRecorder()

	s.handleGetSession(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var info session.SessionInfo
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&info))
	assert.Equal(t, "a1b2c3d4-e5f", info.ID)
	_ = now
}

func TestHandleGetSession_NotFound(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Get", mock.Anything, "00000000-001").Return(nil, fmt.Errorf("%w: 00000000-001", session.ErrNotFound))

	req := httptest.NewRequest("GET", "/v1/sessions/00000000-001", nil)
	req.SetPathValue("id", "00000000-001")
	rec := httptest.NewRecorder()

	s.handleGetSession(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleListSessions(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("List", mock.Anything).Return([]session.SessionInfo{
		{ID: "a1b2c3d4-e5f", Status: "running"},
		{ID: "s2", Status: "destroyed"},
	}, nil)

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	rec := httptest.NewRecorder()

	s.handleListSessions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var sessions []session.SessionInfo
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&sessions))
	assert.Len(t, sessions, 2)
}

func TestHandleDestroy_Success(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Destroy", mock.Anything, "a1b2c3d4-e5f").Return(nil)

	req := httptest.NewRequest("DELETE", "/v1/sessions/a1b2c3d4-e5f", nil)
	req.SetPathValue("id", "a1b2c3d4-e5f")
	rec := httptest.NewRecorder()

	s.handleDestroy(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleDestroy_NotFound(t *testing.T) {
	mockMgr := &MockSessionService{}
	s := testAPIServer(mockMgr)

	mockMgr.On("Destroy", mock.Anything, "00000000-001").Return(fmt.Errorf("session not found: 00000000-001"))

	req := httptest.NewRequest("DELETE", "/v1/sessions/00000000-001", nil)
	req.SetPathValue("id", "00000000-001")
	rec := httptest.NewRecorder()

	s.handleDestroy(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
