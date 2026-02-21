package api

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

type writeWorkspaceRequest struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
	Text          string `json:"text"`
}

func extractWorkspaceContent(req writeWorkspaceRequest) ([]byte, bool) {
	if req.ContentBase64 != "" {
		return []byte(req.ContentBase64), true
	}
	return []byte(req.Text), false
}

func (s *Server) handleWriteWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateWorkspaceID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}

	var req writeWorkspaceRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeValidationError(w, "invalid json: "+err.Error(), nil)
		return
	}

	if err := validateWriteWorkspaceRequest(req); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}

	content, isBase64 := extractWorkspaceContent(req)
	s.logger.Debug("workspace fs write", "workspace_id", id, "path", req.Path, "content_len", len(content))
	if err := s.manager.WriteWorkspaceFile(r.Context(), id, req.Path, content, isBase64); err != nil {
		s.logger.Error("write workspace file", "workspace_id", id, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUploadWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateWorkspaceID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(MaxUploadBytes))
	if err := r.ParseMultipartForm(int64(MaxUploadBytes)); err != nil {
		if err == io.EOF || strings.Contains(err.Error(), "request body too large") {
			writeValidationError(w, "request body too large", map[string]any{"max_bytes": MaxUploadBytes})
			return
		}
		writeValidationError(w, "invalid multipart form: "+err.Error(), nil)
		return
	}

	basePath := strings.Trim(filepath.Clean(r.FormValue("path")), "/")
	if strings.Contains(basePath, "..") {
		writeValidationError(w, "path must not contain '..'", nil)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		files = r.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		writeValidationError(w, "no file provided: use form field 'file' or 'files'", nil)
		return
	}

	var uploaded []string
	for _, fh := range files {
		name := filepath.Base(fh.Filename)
		if name == "" || name == "." || strings.Contains(name, "..") {
			writeValidationError(w, "invalid filename: "+fh.Filename, nil)
			return
		}
		var relPath string
		if basePath != "" {
			relPath = filepath.Join(basePath, name)
		} else {
			relPath = name
		}
		relPath = filepath.ToSlash(filepath.Clean(relPath))
		if strings.Contains(relPath, "..") {
			writeValidationError(w, "invalid destination for "+fh.Filename, nil)
			return
		}

		f, err := fh.Open()
		if err != nil {
			s.logger.Error("workspace upload open file", "workspace_id", id, "filename", fh.Filename, "error", err)
			writeAPIError(w, err)
			return
		}
		content, err := io.ReadAll(io.LimitReader(f, int64(MaxUploadBytes)+1))
		_ = f.Close()
		if err != nil {
			s.logger.Error("workspace upload read file", "workspace_id", id, "filename", fh.Filename, "error", err)
			writeAPIError(w, err)
			return
		}
		if len(content) > MaxUploadBytes {
			writeValidationError(w, "file too large", map[string]any{"filename": fh.Filename, "max_bytes": MaxUploadBytes})
			return
		}

		if err := s.manager.WriteWorkspaceFile(r.Context(), id, relPath, content, false); err != nil {
			s.logger.Error("workspace upload write", "workspace_id", id, "path", relPath, "error", err)
			writeAPIError(w, err)
			return
		}
		uploaded = append(uploaded, relPath)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "paths": uploaded})
}

func (s *Server) handleListWorkspaceFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateWorkspaceID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
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
	if err := ValidateWorkspaceID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
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
