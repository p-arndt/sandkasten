package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/session"
)

type Server struct {
	cfg     *config.Config
	manager *session.Manager
	logger  *slog.Logger
	mux     *http.ServeMux
}

func NewServer(cfg *config.Config, mgr *session.Manager, logger *slog.Logger) *Server {
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
	return s.authMiddleware(s.requestIDMiddleware(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	s.mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /v1/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("POST /v1/sessions/{id}/exec", s.handleExec)
	s.mux.HandleFunc("POST /v1/sessions/{id}/fs/write", s.handleWrite)
	s.mux.HandleFunc("GET /v1/sessions/{id}/fs/read", s.handleRead)
	s.mux.HandleFunc("DELETE /v1/sessions/{id}", s.handleDestroy)

	// Health check (no auth).
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
