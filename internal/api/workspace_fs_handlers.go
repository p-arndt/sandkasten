package api

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListWorkspaceFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pathParam := r.URL.Query().Get("path")
	if pathParam == "" {
		pathParam = "."
	}

	entries, err := s.manager.ListWorkspaceFiles(r.Context(), id, pathParam)
	if err != nil {
		s.logger.Error("list workspace files", "workspace_id", id, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleReadWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pathParam := r.URL.Query().Get("path")
	if pathParam == "" {
		writeValidationError(w, "path is required", nil)
		return
	}

	maxBytes := 0
	if v := r.URL.Query().Get("max_bytes"); v != "" {
		var err error
		maxBytes, err = strconv.Atoi(v)
		if err != nil {
			writeValidationError(w, "invalid max_bytes", nil)
			return
		}
	}

	contentBase64, truncated, err := s.manager.ReadWorkspaceFile(r.Context(), id, pathParam, maxBytes)
	if err != nil {
		s.logger.Error("read workspace file", "workspace_id", id, "path", pathParam, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":           pathParam,
		"content_base64": contentBase64,
		"truncated":      truncated,
	})
}
