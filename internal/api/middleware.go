package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

const dashboardCookieName = "sandkasten_dashboard"

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth for public endpoints and static assets only.
		if isPublicPath(path, r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if s.cfg.APIKey == "" {
			// No API key configured — open access (dev mode).
			next.ServeHTTP(w, r)
			return
		}

		// Accept Bearer token
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != auth && token == s.cfg.APIKey {
			next.ServeHTTP(w, r)
			return
		}

		// Accept dashboard cookie (for browser form posts and dashboard pages)
		if c, _ := r.Cookie(dashboardCookieName); c != nil && c.Value == s.cfg.APIKey {
			next.ServeHTTP(w, r)
			return
		}

		// Login flow: ?api_key=xxx sets cookie and redirects
		if r.Method == http.MethodGet && r.URL.Query().Get("api_key") == s.cfg.APIKey {
			http.SetCookie(w, &http.Cookie{
				Name:     dashboardCookieName,
				Value:    s.cfg.APIKey,
				Path:     "/",
				MaxAge:   86400 * 7, // 7 days
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			q := r.URL.Query()
			q.Del("api_key")
			redirectURL := r.URL.Path
			if q.Encode() != "" {
				redirectURL += "?" + q.Encode()
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		writeUnauthorizedError(w, "missing or invalid authorization")
	})
}

func isPublicPath(path, method string) bool {
	if path == "/healthz" || path == "/" || strings.HasPrefix(path, "/_app/") {
		return true
	}
	if path == "/dashboard/login" && method == http.MethodPost {
		return true
	}

	if strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".woff") ||
		strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".ttf") {
		return true
	}

	if method != http.MethodGet && method != http.MethodHead {
		return false
	}

	if path == "/sessions" || strings.HasPrefix(path, "/sessions/") {
		return true
	}
	if path == "/workspaces" || strings.HasPrefix(path, "/workspaces/") {
		return true
	}
	if path == "/settings" || strings.HasPrefix(path, "/settings/") {
		return true
	}

	return false
}

func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()[:8]
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) debugLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, _ := r.Context().Value(requestIDKey).(string)
		s.logger.Debug("request", "method", r.Method, "path", r.URL.Path, "request_id", reqID)
		next.ServeHTTP(w, r)
	})
}
