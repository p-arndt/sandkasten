package web

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/store"

	"gopkg.in/yaml.v3"
)

//go:embed templates/status.html
var statusHTML string

//go:embed templates/settings.html
var settingsHTML string

type Handler struct {
	store      *store.Store
	configPath string
	startTime  time.Time
}

func NewHandler(store *store.Store, configPath string) *Handler {
	return &Handler{
		store:      store,
		configPath: configPath,
		startTime:  time.Now(),
	}
}

// ServeStatusPage serves the status page HTML
func (h *Handler) ServeStatusPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(statusHTML))
}

// ServeSettingsPage serves the settings page HTML
func (h *Handler) ServeSettingsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(settingsHTML))
}

// GetStatus returns status JSON for the dashboard
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.store.ListSessions()
	if err != nil {
		http.Error(w, `{"error": "failed to list sessions"}`, http.StatusInternalServerError)
		return
	}

	// Count by status
	total := len(sessions)
	active := 0
	expired := 0

	for _, s := range sessions {
		switch s.Status {
		case "running":
			active++
		case "expired":
			expired++
		}
	}

	response := map[string]interface{}{
		"total_sessions":   total,
		"active_sessions":  active,
		"expired_sessions": expired,
		"sessions":         sessions,
		"uptime_seconds":   int(time.Since(h.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetConfig returns the current YAML config file content
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if h.configPath == "" {
		http.Error(w, `{"error": "no config file path set"}`, http.StatusNotFound)
		return
	}

	content, err := os.ReadFile(h.configPath)
	if err != nil {
		http.Error(w, `{"error": "failed to read config file"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"content": string(content),
		"path":    h.configPath,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateConfig saves the YAML config file
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if h.configPath == "" {
		http.Error(w, `{"error": "no config file path set"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Validate YAML before saving
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(req.Content), &cfg); err != nil {
		resp := map[string]interface{}{
			"error": "invalid YAML: " + err.Error(),
			"valid": false,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Write to file
	if err := os.WriteFile(h.configPath, []byte(req.Content), 0644); err != nil {
		http.Error(w, `{"error": "failed to write config file"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ValidateConfig validates YAML without saving
func (h *Handler) ValidateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	var cfg config.Config
	err := yaml.Unmarshal([]byte(req.Content), &cfg)

	response := map[string]interface{}{
		"valid": err == nil,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
