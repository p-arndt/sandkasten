package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/p-arndt/sandkasten/internal/session"
	"github.com/p-arndt/sandkasten/internal/store"
)

// Error codes returned in API responses
const (
	ErrCodeSessionNotFound   = "SESSION_NOT_FOUND"
	ErrCodeSessionExpired    = "SESSION_EXPIRED"
	ErrCodeInvalidImage      = "INVALID_IMAGE"
	ErrCodeInvalidWorkspace  = "INVALID_WORKSPACE"
	ErrCodeCommandTimeout    = "COMMAND_TIMEOUT"
	ErrCodeInvalidRequest    = "INVALID_REQUEST"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeUnauthorized      = "UNAUTHORIZED"
	ErrCodeWorkspaceNotFound = "WORKSPACE_NOT_FOUND"
)

// APIError represents a structured API error response
type APIError struct {
	Code    string                 `json:"error_code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// writeAPIError writes a structured error response with appropriate HTTP status
func writeAPIError(w http.ResponseWriter, err error) {
	var apiErr APIError
	statusCode := http.StatusInternalServerError

	// Map known errors to structured responses
	switch {
	case errors.Is(err, session.ErrNotFound), errors.Is(err, store.ErrNotFound):
		apiErr = APIError{
			Code:    ErrCodeSessionNotFound,
			Message: err.Error(),
		}
		statusCode = http.StatusNotFound

	case errors.Is(err, session.ErrExpired):
		apiErr = APIError{
			Code:    ErrCodeSessionExpired,
			Message: err.Error(),
		}
		statusCode = http.StatusGone

	case errors.Is(err, session.ErrInvalidImage):
		apiErr = APIError{
			Code:    ErrCodeInvalidImage,
			Message: err.Error(),
		}
		statusCode = http.StatusBadRequest

	case errors.Is(err, session.ErrTimeout):
		apiErr = APIError{
			Code:    ErrCodeCommandTimeout,
			Message: err.Error(),
		}
		statusCode = http.StatusGatewayTimeout

	default:
		// Generic internal error
		apiErr = APIError{
			Code:    ErrCodeInternalError,
			Message: err.Error(),
		}
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(apiErr)
}

// writeValidationError writes a 400 Bad Request with validation details
func writeValidationError(w http.ResponseWriter, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(APIError{
		Code:    ErrCodeInvalidRequest,
		Message: message,
		Details: details,
	})
}

// writeUnauthorizedError writes a 401 Unauthorized error
func writeUnauthorizedError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(APIError{
		Code:    ErrCodeUnauthorized,
		Message: message,
	})
}
