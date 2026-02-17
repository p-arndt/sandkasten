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
	store                *store.Store
	pool                 *pool.Pool
	configPath           string
	playgroundConfigPath string
	startTime            time.Time
	distFS               fs.FS
}

func NewHandler(store *store.Store, poolMgr *pool.Pool, configPath, playgroundConfigPath string) *Handler {
	// Extract the dist subdirectory from the embed.FS
	distSubFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Fallback to the root FS if dist doesn't exist yet
		distSubFS = distFS
	}

	return &Handler{
		store:                store,
		pool:                 poolMgr,
		configPath:           configPath,
		playgroundConfigPath: playgroundConfigPath,
		startTime:            time.Now(),
		distFS:               distSubFS,
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

// PlaygroundConfig is the JSON structure for /config/config.json (or playground_config_path).
type PlaygroundConfig struct {
	Playground *struct {
		Provider           string `json:"provider"`
		Model              string `json:"model"`
		OpenAIApiKey       string `json:"openaiApiKey"`
		GoogleApiKey       string `json:"googleApiKey"`
		GoogleVertexApiKey string `json:"googleVertexApiKey"`
		VertexProject      string `json:"vertexProject"`
		VertexLocation     string `json:"vertexLocation"`
	} `json:"playground"`
}

// GetPlaygroundConfig returns provider and API keys from the backend config file.
// Requires auth (same API key as rest of API). Frontend stores keys in memory only.
func (h *Handler) GetPlaygroundConfig(w http.ResponseWriter, r *http.Request) {
	if h.playgroundConfigPath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "playground config not configured (set playground_config_path or SANDKASTEN_PLAYGROUND_CONFIG_PATH)"}`))
		return
	}

	data, err := os.ReadFile(h.playgroundConfigPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "playground config file not found"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to read playground config"}`))
		return
	}

	var cfg PlaygroundConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "invalid playground config JSON"}`))
		return
	}

	if cfg.Playground == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
		return
	}

	out := map[string]interface{}{
		"provider":           cfg.Playground.Provider,
		"model":              cfg.Playground.Model,
		"openaiApiKey":       cfg.Playground.OpenAIApiKey,
		"googleApiKey":       cfg.Playground.GoogleApiKey,
		"googleVertexApiKey": cfg.Playground.GoogleVertexApiKey,
		"vertexProject":      cfg.Playground.VertexProject,
		"vertexLocation":     cfg.Playground.VertexLocation,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// PutPlaygroundConfig writes the playground config to the backend JSON file.
// Creates the file and parent directory if they do not exist.
func (h *Handler) PutPlaygroundConfig(w http.ResponseWriter, r *http.Request) {
	if h.playgroundConfigPath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "playground config not configured (set playground_config_path or SANDKASTEN_PLAYGROUND_CONFIG_PATH)"}`))
		return
	}

	var body struct {
		Provider           string `json:"provider"`
		Model              string `json:"model"`
		OpenAIApiKey       string `json:"openaiApiKey"`
		GoogleApiKey       string `json:"googleApiKey"`
		GoogleVertexApiKey string `json:"googleVertexApiKey"`
		VertexProject      string `json:"vertexProject"`
		VertexLocation     string `json:"vertexLocation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid JSON"}`))
		return
	}

	// Load existing file to preserve top-level keys other than "playground"
	var root map[string]interface{}
	if data, err := os.ReadFile(h.playgroundConfigPath); err == nil {
		_ = json.Unmarshal(data, &root)
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	root["playground"] = map[string]interface{}{
		"provider":            body.Provider,
		"model":               body.Model,
		"openaiApiKey":        body.OpenAIApiKey,
		"googleApiKey":        body.GoogleApiKey,
		"googleVertexApiKey":  body.GoogleVertexApiKey,
		"vertexProject":       body.VertexProject,
		"vertexLocation":      body.VertexLocation,
	}

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to encode config"}`))
		return
	}

	dir := path.Dir(h.playgroundConfigPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "failed to create config directory"}`))
			return
		}
	}

	if err := os.WriteFile(h.playgroundConfigPath, data, 0600); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "failed to write config file"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
