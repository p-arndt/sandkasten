package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAPIError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "session not found",
			err:        fmt.Errorf("%w: abc123", session.ErrNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   ErrCodeSessionNotFound,
		},
		{
			name:       "store not found",
			err:        fmt.Errorf("wrap: %w", store.ErrNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   ErrCodeSessionNotFound,
		},
		{
			name:       "session expired",
			err:        fmt.Errorf("%w: abc123", session.ErrExpired),
			wantStatus: http.StatusGone,
			wantCode:   ErrCodeSessionExpired,
		},
		{
			name:       "invalid image",
			err:        fmt.Errorf("%w: bad-image", session.ErrInvalidImage),
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeInvalidImage,
		},
		{
			name:       "command timeout",
			err:        fmt.Errorf("%w", session.ErrTimeout),
			wantStatus: http.StatusGatewayTimeout,
			wantCode:   ErrCodeCommandTimeout,
		},
		{
			name:       "generic error",
			err:        fmt.Errorf("something went wrong"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   ErrCodeInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeAPIError(rec, tt.err)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var apiErr APIError
			require.NoError(t, decodeBody(rec, &apiErr))
			assert.Equal(t, tt.wantCode, apiErr.Code)
			assert.NotEmpty(t, apiErr.Message)
		})
	}
}

func TestWriteValidationError(t *testing.T) {
	rec := httptest.NewRecorder()
	details := map[string]interface{}{"field": "cmd"}
	writeValidationError(rec, "cmd is required", details)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var apiErr APIError
	require.NoError(t, decodeBody(rec, &apiErr))
	assert.Equal(t, ErrCodeInvalidRequest, apiErr.Code)
	assert.Equal(t, "cmd is required", apiErr.Message)
	assert.Equal(t, "cmd", apiErr.Details["field"])
}

func TestWriteUnauthorizedError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeUnauthorizedError(rec, "invalid api key")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var apiErr APIError
	require.NoError(t, decodeBody(rec, &apiErr))
	assert.Equal(t, ErrCodeUnauthorized, apiErr.Code)
	assert.Equal(t, "invalid api key", apiErr.Message)
}

func decodeBody(rec *httptest.ResponseRecorder, v any) error {
	return json.NewDecoder(rec.Body).Decode(v)
}
