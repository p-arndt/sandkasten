package api

import (
	"net/http"
)

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.manager.ListWorkspaces(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateWorkspaceID(id); err != nil {
		writeValidationError(w, err.Error(), nil)
		return
	}
	if err := s.manager.DeleteWorkspace(r.Context(), id); err != nil {
		s.logger.Error("delete workspace", "workspace_id", id, "error", err)
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
