package handlers

import (
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/go-chi/chi/v5"
)

// RequestItem represents a simplified Telescope request for the UI
type RequestItem struct {
	ID       string `json:"id"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	Duration string `json:"duration"`
	Time     string `json:"time"`
}

// GetRequests handles the fetching of paginated Telescope requests
func GetRequests(w http.ResponseWriter, r *http.Request) {
	// Extract the project ID from the URL using chi
	projectID := chi.URLParam(r, "projectId")
	if projectID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	// TODO: Replace this mock data with an actual query to the DuckDB Parquet engine using the projectID
	mockData := []RequestItem{
		{ID: "1", Method: "GET", Path: "/api/users", Status: 200, Duration: "124ms", Time: "21 Jul 2026 10:45 AM"},
		{ID: "2", Method: "POST", Path: "/api/auth/login", Status: 200, Duration: "340ms", Time: "21 Jul 2026 10:42 AM"},
		{ID: "3", Method: "GET", Path: "/api/projects/1/sync", Status: 404, Duration: "12ms", Time: "21 Jul 2026 10:30 AM"},
		{ID: "4", Method: "PUT", Path: "/api/settings", Status: 500, Duration: "1205ms", Time: "21 Jul 2026 09:15 AM"},
		{ID: "5", Method: "GET", Path: "/health", Status: 200, Duration: "5ms", Time: "21 Jul 2026 08:00 AM"},
	}

	// Construct the pagination meta block
	meta := api.APIMeta{
		Total:       5,
		PerPage:     10,
		CurrentPage: 1,
		LastPage:    1,
	}

	// Respond using our standard JSON envelope
	api.RespondJSON(w, http.StatusOK, mockData, meta)
}
