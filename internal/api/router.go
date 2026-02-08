package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/p-arndt/sandkasten/internal/web"
)

type Server struct {
	cfg        *config.Config
	manager    *session.Manager
	logger     *slog.Logger
	mux        *http.ServeMux
	webHandler *web.Handler
}

func NewServer(cfg *config.Config, mgr *session.Manager, st *store.Store, configPath string, logger *slog.Logger) *Server {
	s := &Server{
		cfg:        cfg,
		manager:    mgr,
		logger:     logger,
		mux:        http.NewServeMux(),
		webHandler: web.NewHandler(st, configPath),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.authMiddleware(s.requestIDMiddleware(s.mux))
}

func (s *Server) routes() {
	// API routes (with auth)
	s.mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	s.mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /v1/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("POST /v1/sessions/{id}/exec", s.handleExec)
	s.mux.HandleFunc("POST /v1/sessions/{id}/exec/stream", s.handleExecStream)
	s.mux.HandleFunc("POST /v1/sessions/{id}/fs/write", s.handleWrite)
	s.mux.HandleFunc("GET /v1/sessions/{id}/fs/read", s.handleRead)
	s.mux.HandleFunc("DELETE /v1/sessions/{id}", s.handleDestroy)

	// Workspace routes (with auth)
	s.mux.HandleFunc("GET /v1/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("DELETE /v1/workspaces/{id}", s.handleDeleteWorkspace)

	// Web UI routes (no auth for viewing)
	s.mux.HandleFunc("GET /", s.webHandler.ServeStatusPage)
	s.mux.HandleFunc("GET /settings", s.webHandler.ServeSettingsPage)

	// Web API routes (read-only: no auth, modifications: requires auth if API key set)
	s.mux.HandleFunc("GET /api/status", s.webHandler.GetStatus)
	s.mux.HandleFunc("GET /api/config", s.webHandler.GetConfig)
	s.mux.HandleFunc("PUT /api/config", s.webHandler.UpdateConfig)       // Protected
	s.mux.HandleFunc("POST /api/config/validate", s.webHandler.ValidateConfig) // Protected

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
