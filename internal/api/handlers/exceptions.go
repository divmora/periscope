package handlers

import (
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/go-chi/chi/v5"
)

// ExceptionItem represents a simplified Telescope exception for the UI
type ExceptionItem struct {
	ID      string `json:"id"`
	Class   string `json:"class"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

// GetExceptions handles the fetching of paginated Telescope exceptions
func GetExceptions(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	if projectID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	// Mock Data for Exceptions
	mockData := []ExceptionItem{
		{ID: "1", Class: "TypeError", Message: "Cannot read properties of null (reading 'id')", Time: "21 Jul 2026 11:45 AM"},
		{ID: "2", Class: "Illuminate\\Database\\QueryException", Message: "SQLSTATE[42S02]: Base table or view not found", Time: "21 Jul 2026 10:42 AM"},
		{ID: "3", Class: "Symfony\\Component\\HttpKernel\\Exception\\NotFoundHttpException", Message: "Route not found.", Time: "21 Jul 2026 09:30 AM"},
	}

	meta := api.APIMeta{
		Total:       3,
		PerPage:     10,
		CurrentPage: 1,
		LastPage:    1,
	}

	api.RespondJSON(w, http.StatusOK, mockData, meta)
}
