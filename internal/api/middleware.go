package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth for static assets and SPA routes only (not /api/*)
		if path == "/healthz" ||
			path == "/" ||
			path == "/api/status" ||
			strings.HasPrefix(path, "/_app/") ||
			strings.HasPrefix(path, "/sessions") ||
			strings.HasPrefix(path, "/workspaces") ||
			strings.HasPrefix(path, "/settings") ||
			(strings.HasSuffix(path, ".js") ||
				strings.HasSuffix(path, ".css") ||
				strings.HasSuffix(path, ".svg") ||
				strings.HasSuffix(path, ".png") ||
				strings.HasSuffix(path, ".ico") ||
				strings.HasSuffix(path, ".woff") ||
				strings.HasSuffix(path, ".woff2") ||
				strings.HasSuffix(path, ".ttf")) {
			next.ServeHTTP(w, r)
			return
		}
		// /api/config (GET, PUT) and /api/config/validate (POST) require auth when API key is set (see below)

		if s.cfg.APIKey == "" {
			// No API key configured — open access (dev mode).
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeUnauthorizedError(w, "missing authorization header")
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth || token != s.cfg.APIKey {
			writeUnauthorizedError(w, "invalid api key")
			return
		}

		next.ServeHTTP(w, r)
	})
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
