package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/p-arndt/sandkasten/internal/session"
)

type createSessionRequest struct {
	Image       string `json:"image"`
	TTLSeconds  int    `json:"ttl_seconds"`
	WorkspaceID string `json:"workspace_id"` // optional persistent workspace
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	info, err := s.manager.Create(r.Context(), session.CreateOpts{
		Image:       req.Image,
		TTLSeconds:  req.TTLSeconds,
		WorkspaceID: req.WorkspaceID,
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

// handleExec handles blocking exec (returns when complete)
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

// handleExecStream handles streaming exec with Server-Sent Events
func (s *Server) handleExecStream(w http.ResponseWriter, r *http.Request) {
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

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Stream exec chunks
	chunkChan := make(chan session.ExecChunk, 10)
	errChan := make(chan error, 1)

	go func() {
		err := s.manager.ExecStream(r.Context(), id, req.Cmd, req.TimeoutMs, chunkChan)
		if err != nil {
			errChan <- err
		}
		close(chunkChan)
		close(errChan)
	}()

	// Send chunks as SSE
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				// Channel closed, we're done
				return
			}

			if chunk.Done {
				// For v0.1: send output as chunk event first (if present)
				// This handles the case where manager sends a single chunk with both output and Done=true
				if chunk.Output != "" {
					chunkJSON, _ := json.Marshal(map[string]interface{}{
						"chunk":     chunk.Output,
						"timestamp": chunk.Timestamp,
					})
					fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", chunkJSON)
					flusher.Flush()
				}

				// Then send completion event with metadata
				fmt.Fprintf(w, "event: done\ndata: {\"exit_code\":%d,\"cwd\":\"%s\",\"duration_ms\":%d}\n\n",
					chunk.ExitCode, chunk.Cwd, chunk.DurationMs)
				flusher.Flush()
				return
			}

			// Send chunk event (for future true streaming in v0.2)
			if chunk.Output != "" {
				chunkJSON, _ := json.Marshal(map[string]interface{}{
					"chunk":     chunk.Output,
					"timestamp": chunk.Timestamp,
				})
				fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", chunkJSON)
				flusher.Flush()
			}

		case err := <-errChan:
			if err != nil {
				errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", errJSON)
				flusher.Flush()
				return
			}

		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
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

// Workspace handlers

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.manager.ListWorkspaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.manager.DeleteWorkspace(r.Context(), id); err != nil {
		s.logger.Error("delete workspace", "workspace_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
