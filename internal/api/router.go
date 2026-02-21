package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/store"
)

type Server struct {
	cfg     *config.Config
	manager SessionService
	logger  *slog.Logger
	mux     *http.ServeMux
}

func NewServer(cfg *config.Config, mgr SessionService, st *store.Store, configPath string, logger *slog.Logger) *Server {
	s := &Server{
		cfg:     cfg,
		manager: mgr,
		logger:  logger,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.authMiddleware(s.requestIDMiddleware(s.debugLogMiddleware(s.mux)))
}

func (s *Server) routes() {
	// API routes (with auth)
	s.mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	s.mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /v1/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /v1/sessions/{id}/stats", s.handleGetSessionStats)
	s.mux.HandleFunc("POST /v1/sessions/{id}/exec", s.handleExec)
	s.mux.HandleFunc("POST /v1/sessions/{id}/exec/stream", s.handleExecStream)
	s.mux.HandleFunc("POST /v1/sessions/{id}/fs/write", s.handleWrite)
	s.mux.HandleFunc("POST /v1/sessions/{id}/fs/upload", s.handleUpload)
	s.mux.HandleFunc("GET /v1/sessions/{id}/fs/read", s.handleRead)
	s.mux.HandleFunc("DELETE /v1/sessions/{id}", s.handleDestroy)

	// Workspace routes (with auth)
	s.mux.HandleFunc("GET /v1/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("DELETE /v1/workspaces/{id}", s.handleDeleteWorkspace)
	s.mux.HandleFunc("POST /v1/workspaces/{id}/fs/write", s.handleWriteWorkspaceFile)
	s.mux.HandleFunc("POST /v1/workspaces/{id}/fs/upload", s.handleUploadWorkspaceFile)
	s.mux.HandleFunc("GET /v1/workspaces/{id}/fs", s.handleListWorkspaceFiles)
	s.mux.HandleFunc("GET /v1/workspaces/{id}/fs/read", s.handleReadWorkspaceFile)

	// Health check (no auth)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
