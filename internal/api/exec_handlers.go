package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/p-arndt/sandkasten/internal/session"
)

type execRequest struct {
	Cmd       string `json:"cmd"`
	TimeoutMs int    `json:"timeout_ms"`
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateSessionID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	var req execRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeValidationError(w, "invalid json: "+err.Error(), nil)
		return
	}
	if err := validateExecRequest(req); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	s.logger.Debug("exec", "session_id", id, "cmd", req.Cmd, "timeout_ms", req.TimeoutMs)
	result, err := s.manager.Exec(r.Context(), id, req.Cmd, req.TimeoutMs)
	if err != nil {
		s.logger.Error("exec", "session_id", id, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleExecStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateSessionID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	var req execRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeValidationError(w, "invalid json: "+err.Error(), nil)
		return
	}

	if err := validateExecRequest(req); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}

	if err := setupSSE(w); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	s.logger.Debug("exec stream", "session_id", id, "cmd", req.Cmd, "timeout_ms", req.TimeoutMs)
	flusher := w.(http.Flusher)
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

	streamSSEChunks(w, flusher, chunkChan, errChan, r)
}

// setupSSE configures headers for Server-Sent Events streaming.
func setupSSE(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if _, ok := w.(http.Flusher); !ok {
		return fmt.Errorf("streaming not supported")
	}

	return nil
}

// streamSSEChunks sends exec chunks as Server-Sent Events.
func streamSSEChunks(w http.ResponseWriter, flusher http.Flusher, chunkChan <-chan session.ExecChunk, errChan <-chan error, r *http.Request) {
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				return
			}

			if chunk.Done {
				sendDoneChunk(w, flusher, chunk)
				return
			}

			if chunk.Output != "" {
				sendOutputChunk(w, flusher, chunk)
			}

		case err := <-errChan:
			if err != nil {
				sendErrorEvent(w, flusher, err)
				return
			}

		case <-r.Context().Done():
			return
		}
	}
}

// sendDoneChunk sends the final chunk with execution metadata.
func sendDoneChunk(w http.ResponseWriter, flusher http.Flusher, chunk session.ExecChunk) {
	if chunk.Output != "" {
		chunkJSON, _ := json.Marshal(map[string]interface{}{
			"chunk":     chunk.Output,
			"timestamp": chunk.Timestamp,
		})
		fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", chunkJSON)
		flusher.Flush()
	}

	doneJSON, _ := json.Marshal(map[string]interface{}{
		"exit_code":   chunk.ExitCode,
		"cwd":         chunk.Cwd,
		"duration_ms": chunk.DurationMs,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneJSON)
	flusher.Flush()
}

// sendOutputChunk sends an output chunk event.
func sendOutputChunk(w http.ResponseWriter, flusher http.Flusher, chunk session.ExecChunk) {
	chunkJSON, _ := json.Marshal(map[string]interface{}{
		"chunk":     chunk.Output,
		"timestamp": chunk.Timestamp,
	})
	fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", chunkJSON)
	flusher.Flush()
}

// sendErrorEvent sends an error event.
func sendErrorEvent(w http.ResponseWriter, flusher http.Flusher, err error) {
	errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", errJSON)
	flusher.Flush()
}
