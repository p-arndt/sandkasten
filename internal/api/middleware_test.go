package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/p-arndt/sandkasten/internal/config"
	"github.com/stretchr/testify/assert"
)

func testServer(apiKey string) *Server {
	return &Server{
		cfg: &config.Config{
			APIKey: apiKey,
		},
	}
}

func TestAuthMiddleware_NoAPIKey(t *testing.T) {
	s := testServer("")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No API key configured = open access
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	s := testServer("sk-test-key")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer sk-test-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	s := testServer("sk-test-key")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	s := testServer("sk-test-key")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_NoBearerPrefix(t *testing.T) {
	s := testServer("sk-test-key")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	req.Header.Set("Authorization", "sk-test-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_SkipPaths(t *testing.T) {
	s := testServer("sk-test-key")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	skipPaths := []string{
		"/healthz",
		"/",
		"/api/status",
		"/_app/immutable/something.js",
		"/sessions",
		"/workspaces",
		"/settings",
	}

	for _, path := range skipPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			// No auth header
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "path %s should skip auth", path)
		})
	}
}

func TestAuthMiddleware_SkipStaticFiles(t *testing.T) {
	s := testServer("sk-test-key")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	staticFiles := []string{
		"/app.js",
		"/styles.css",
		"/favicon.svg",
		"/logo.png",
		"/favicon.ico",
		"/font.woff",
		"/font.woff2",
		"/font.ttf",
	}

	for _, path := range staticFiles {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "path %s should skip auth", path)
		})
	}
}

func TestAuthMiddleware_ReadOnlyConfig(t *testing.T) {
	s := testServer("sk-test-key")
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET /api/config should not require auth
	req := httptest.NewRequest("GET", "/api/config", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// PUT /api/config should require auth
	req = httptest.NewRequest("PUT", "/api/config", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	s := testServer("")
	var gotID string
	handler := s.requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Context().Value(requestIDKey).(string)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.NotEmpty(t, gotID)
	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func TestRequestIDMiddleware_PreservesID(t *testing.T) {
	s := testServer("")
	var gotID string
	handler := s.requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Context().Value(requestIDKey).(string)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/sessions", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "my-custom-id", gotID)
	assert.Equal(t, "my-custom-id", rec.Header().Get("X-Request-ID"))
}
