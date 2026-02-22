package api

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/p-arndt/sandkasten/internal/session"
)

//go:embed templates/dashboard/*.html
var dashboardTemplates embed.FS

type dashboardPage struct {
	Title      string
	Sessions   []session.SessionInfo
	Images     []string
	DefaultImg string
	Flash      string
	FlashErr   string
}

type playgroundPage struct {
	Title    string
	Session  session.SessionInfo
	Flash    string
	FlashErr string
}

func (s *Server) parseDashboardTemplates() (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"timeFormat": func(t time.Time) string { return t.Format("2006-01-02 15:04:05") },
		"json":       func(v any) (string, error) { b, err := json.Marshal(v); return string(b), err },
		"len":        func(s []session.SessionInfo) int { return len(s) },
		// jsQuote outputs a JS string literal without html/template escaping it (avoids \" in URLs)
		"jsQuote": func(s string) template.JS {
			b, _ := json.Marshal(s)
			return template.JS(b)
		},
	}).ParseFS(dashboardTemplates, "templates/dashboard/*.html")
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := s.parseDashboardTemplates()
	if err != nil {
		s.logger.Error("parse dashboard templates", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessions, err := s.manager.List(r.Context())
	if err != nil {
		s.logger.Error("list sessions for dashboard", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	images := s.dashboardImages()
	page := dashboardPage{
		Title:      "Sandkasten Dashboard",
		Sessions:   sessions,
		Images:     images,
		DefaultImg: s.cfg.DefaultImage,
		Flash:      r.URL.Query().Get("flash"),
		FlashErr:   r.URL.Query().Get("flash_err"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", page); err != nil {
		s.logger.Error("execute dashboard template", "error", err)
	}
}

func (s *Server) handlePlayground(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := ValidateSessionID(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := s.manager.Get(r.Context(), id)
	if err != nil {
		s.logger.Error("get session for playground", "session_id", id, "error", err)
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	tmpl, err := s.parseDashboardTemplates()
	if err != nil {
		s.logger.Error("parse dashboard templates", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	page := playgroundPage{
		Title:   "Playground · " + id[:8],
		Session: *info,
		Flash:   r.URL.Query().Get("flash"),
		FlashErr: r.URL.Query().Get("flash_err"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "playground.html", page); err != nil {
		s.logger.Error("execute playground template", "error", err)
	}
}

func (s *Server) handleDashboardCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	image := r.FormValue("image")
	if image == "" {
		image = s.cfg.DefaultImage
	}
	ttl := 1800
	if v := r.FormValue("ttl_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 86400 {
			ttl = n
		}
	}
	workspaceID := r.FormValue("workspace_id")

	info, err := s.manager.Create(r.Context(), session.CreateOpts{
		Image:       image,
		TTLSeconds:  ttl,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		s.logger.Error("dashboard create session", "error", err)
		http.Redirect(w, r, "/dashboard?flash_err="+encodeQuery(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard?flash=Session "+info.ID+" created", http.StatusSeeOther)
}

func (s *Server) handleDashboardDestroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if err := ValidateSessionID(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.manager.Destroy(r.Context(), id); err != nil {
		s.logger.Error("dashboard destroy session", "session_id", id, "error", err)
		http.Redirect(w, r, "/dashboard?flash_err="+encodeQuery(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard?flash=Session "+id+" destroyed", http.StatusSeeOther)
}

func (s *Server) handleDashboardBulkDestroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ids := r.Form["ids"]
	if len(ids) == 0 {
		http.Redirect(w, r, "/dashboard?flash_err="+encodeQuery("No sessions selected"), http.StatusSeeOther)
		return
	}
	var destroyed, failed int
	for _, id := range ids {
		if err := ValidateSessionID(id); err != nil {
			failed++
			continue
		}
		if err := s.manager.Destroy(r.Context(), id); err != nil {
			s.logger.Error("dashboard bulk destroy", "session_id", id, "error", err)
			failed++
		} else {
			destroyed++
		}
	}
	var q string
	if destroyed > 0 {
		msg := "Destroyed " + strconv.Itoa(destroyed) + " session(s)"
		if failed > 0 {
			msg += "; " + strconv.Itoa(failed) + " failed"
		}
		q = "flash=" + encodeQuery(msg)
	} else if failed > 0 {
		q = "flash_err=" + encodeQuery("Failed to destroy sessions")
	}
	if q != "" {
		q = "?" + q
	}
	http.Redirect(w, r, "/dashboard"+q, http.StatusSeeOther)
}

func (s *Server) dashboardImages() []string {
	if len(s.cfg.AllowedImages) > 0 {
		return s.cfg.AllowedImages
	}
	if len(s.cfg.Pool.Images) > 0 {
		imgs := make([]string, 0, len(s.cfg.Pool.Images))
		for k := range s.cfg.Pool.Images {
			imgs = append(imgs, k)
		}
		sort.Strings(imgs)
		return imgs
	}
	return []string{s.cfg.DefaultImage}
}

func encodeQuery(s string) string {
	return url.QueryEscape(s)
}

func (s *Server) handleDashboardLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg.APIKey == "" {
		http.Redirect(w, r, "/dashboard?flash=No API key configured — auth disabled", http.StatusSeeOther)
		return
	}
	apiKey := r.FormValue("api_key")
	if apiKey != s.cfg.APIKey {
		http.Redirect(w, r, "/dashboard?flash_err="+encodeQuery("Invalid API key"), http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     dashboardCookieName,
		Value:    s.cfg.APIKey,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/dashboard?flash=Logged in", http.StatusSeeOther)
}
