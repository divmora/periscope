package handlers

import (
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/go-chi/chi/v5"
)

type LogItem struct {
	ID      string `json:"id"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

func GetLogs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	if projectID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	mockData := []LogItem{
		{ID: "1", Level: "info", Message: "User 1 successfully authenticated via API token.", Time: "21 Jul 2026 11:45 AM"},
		{ID: "2", Level: "warning", Message: "Deprecation notice: Method legacy_sync() is deprecated.", Time: "21 Jul 2026 10:42 AM"},
		{ID: "3", Level: "error", Message: "Failed to connect to external payment gateway.", Time: "21 Jul 2026 09:30 AM"},
	}

	meta := api.APIMeta{
		Total:       3,
		PerPage:     10,
		CurrentPage: 1,
		LastPage:    1,
	}

	api.RespondJSON(w, http.StatusOK, mockData, meta)
}
