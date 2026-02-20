package api

import (
	"net/http"
	"strconv"
)

type writeRequest struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
	Text          string `json:"text"`
}

func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateSessionID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	var req writeRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeValidationError(w, "invalid json: "+err.Error(), nil)
		return
	}

	if err := validateWriteRequest(req); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}

	content, isBase64 := extractContent(req)
	s.logger.Debug("fs write", "session_id", id, "path", req.Path, "content_len", len(content), "is_base64", isBase64)
	if err := s.manager.Write(r.Context(), id, req.Path, content, isBase64); err != nil {
		s.logger.Error("write", "session_id", id, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateSessionID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	path := r.URL.Query().Get("path")

	maxBytes, err := parseMaxBytes(r)
	if err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}

	if err := validateReadRequest(path, maxBytes); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	s.logger.Debug("fs read", "session_id", id, "path", path, "max_bytes", maxBytes)
	contentBase64, truncated, err := s.manager.Read(r.Context(), id, path, maxBytes)
	if err != nil {
		s.logger.Error("read", "session_id", id, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":           path,
		"content_base64": contentBase64,
		"truncated":      truncated,
	})
}

// extractContent returns content and whether it's base64 encoded.
func extractContent(req writeRequest) ([]byte, bool) {
	if req.ContentBase64 != "" {
		return []byte(req.ContentBase64), true
	}
	return []byte(req.Text), false
}

// parseMaxBytes parses max_bytes query parameter.
func parseMaxBytes(r *http.Request) (int, error) {
	maxBytes := 0
	if v := r.URL.Query().Get("max_bytes"); v != "" {
		var err error
		maxBytes, err = strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
	}
	return maxBytes, nil
}
