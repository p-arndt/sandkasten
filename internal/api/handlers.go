package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/p-arndt/sandkasten/internal/session"
)

type createSessionRequest struct {
	Image      string `json:"image"`
	TTLSeconds int    `json:"ttl_seconds"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	info, err := s.manager.Create(r.Context(), session.CreateOpts{
		Image:      req.Image,
		TTLSeconds: req.TTLSeconds,
	})
	if err != nil {
		s.logger.Error("create session", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := s.manager.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.manager.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

type execRequest struct {
	Cmd       string `json:"cmd"`
	TimeoutMs int    `json:"timeout_ms"`
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req execRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	if req.Cmd == "" {
		writeError(w, http.StatusBadRequest, "cmd is required")
		return
	}

	result, err := s.manager.Exec(r.Context(), id, req.Cmd, req.TimeoutMs)
	if err != nil {
		s.logger.Error("exec", "session_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type writeRequest struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
	Text          string `json:"text"`
}

func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req writeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	var content []byte
	var isBase64 bool
	if req.ContentBase64 != "" {
		content = []byte(req.ContentBase64)
		isBase64 = true
	} else {
		content = []byte(req.Text)
	}

	if err := s.manager.Write(r.Context(), id, req.Path, content, isBase64); err != nil {
		s.logger.Error("write", "session_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	maxBytes := 0
	if v := r.URL.Query().Get("max_bytes"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxBytes = n
		}
	}

	contentBase64, truncated, err := s.manager.Read(r.Context(), id, path, maxBytes)
	if err != nil {
		s.logger.Error("read", "session_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":           path,
		"content_base64": contentBase64,
		"truncated":      truncated,
	})
}

func (s *Server) handleDestroy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.manager.Destroy(r.Context(), id); err != nil {
		s.logger.Error("destroy", "session_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
