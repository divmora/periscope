package api

import (
	"encoding/json"
	"net/http"
)

// Response represents the standard JSON envelope for all API responses.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   *APIError   `json:"error"`
	Meta    interface{} `json:"meta,omitempty"`
}

// APIError represents a structured error payload.
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// APIMeta represents standard pagination metadata
type APIMeta struct {
	Total       int `json:"total"`
	PerPage     int `json:"per_page"`
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
}

// RespondJSON writes a standard successful JSON response.
func RespondJSON(w http.ResponseWriter, status int, data interface{}, meta interface{}) {
	resp := Response{
		Success: true,
		Data:    data,
		Error:   nil,
		Meta:    meta,
	}

	writeJSON(w, status, resp)
}

// RespondError writes a standard error JSON response.
func RespondError(w http.ResponseWriter, status int, code, message string, details interface{}) {
	resp := Response{
		Success: false,
		Data:    nil,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}

	writeJSON(w, status, resp)
}

// writeJSON is a helper to serialize and write the response payload.
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Fallback error if encoding fails
		http.Error(w, `{"success":false,"error":{"code":"INTERNAL_ENCODING_ERROR","message":"Failed to encode response"}}`, http.StatusInternalServerError)
	}
}
