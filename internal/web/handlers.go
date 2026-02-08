package web

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/p-arndt/sandkasten/internal/pool"
	"github.com/p-arndt/sandkasten/internal/store"

	"gopkg.in/yaml.v3"
)

//go:embed all:dist
var distFS embed.FS

type Handler struct {
	store      *store.Store
	pool       *pool.Pool
	configPath string
	startTime  time.Time
	distFS     fs.FS
}

func NewHandler(store *store.Store, poolMgr *pool.Pool, configPath string) *Handler {
	// Extract the dist subdirectory from the embed.FS
	distSubFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Fallback to the root FS if dist doesn't exist yet
		distSubFS = distFS
	}

	return &Handler{
		store:      store,
		pool:       poolMgr,
		configPath: configPath,
		startTime:  time.Now(),
		distFS:     distSubFS,
	}
}

// ServeSPA serves the SvelteKit static files with SPA fallback
func (h *Handler) ServeSPA(w http.ResponseWriter, r *http.Request) {
	// Clean the path
	p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if p == "" || p == "." {
		p = "index.html"
	}

	// Try to open the requested file
	file, err := h.distFS.Open(p)
	if err != nil {
		// If file not found and not an asset, serve index.html for SPA routing
		if !strings.HasPrefix(p, "_app/") && !strings.Contains(p, ".") {
			file, err = h.distFS.Open("index.html")
			if err != nil {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}
	defer file.Close()

	// Get file info for serving
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Serve the file
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file.(io.ReadSeeker))
}

// GetStatus returns status JSON for the dashboard
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.store.ListSessions()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to list sessions"}`))
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

	// Add pool status if pool is available
	if h.pool != nil {
		poolStatus := h.pool.Status()
		if poolStatus.Enabled || len(poolStatus.Images) > 0 {
			response["pool"] = poolStatus
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetConfig returns the current YAML config file content
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if h.configPath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "no config file path set"}`))
		return
	}

	content, err := os.ReadFile(h.configPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to read config file"}`))
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "no config file path set"}`))
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid JSON"}`))
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

	// Write to file with restricted permissions (config may contain API keys)
	if err := os.WriteFile(h.configPath, []byte(req.Content), 0600); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to write config file"}`))
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid JSON"}`))
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
