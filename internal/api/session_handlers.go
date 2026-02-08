package api

import (
	"encoding/json"
	"net/http"

	"github.com/p-arndt/sandkasten/internal/session"
)

type createSessionRequest struct {
	Image       string `json:"image"`
	TTLSeconds  int    `json:"ttl_seconds"`
	WorkspaceID string `json:"workspace_id"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, "invalid json: "+err.Error(), nil)
		return
	}

	if err := validateCreateSessionRequest(req); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}

	info, err := s.manager.Create(r.Context(), session.CreateOpts{
		Image:       req.Image,
		TTLSeconds:  req.TTLSeconds,
		WorkspaceID: req.WorkspaceID,
	})
	if err != nil {
		s.logger.Error("create session", "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := s.manager.Get(r.Context(), id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.manager.List(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleDestroy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.manager.Destroy(r.Context(), id); err != nil {
		s.logger.Error("destroy", "session_id", id, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
