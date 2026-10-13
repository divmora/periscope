package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/store"
	"github.com/go-chi/chi/v5"
)

// ListSources lists all data sources for a project
func ListSources(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	sources, err := store.ListProjectSources(r.Context(), projectIID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list project sources", err.Error())
		return
	}

	if sources == nil {
		sources = []store.ProjectSource{}
	}

	api.RespondJSON(w, http.StatusOK, sources, nil)
}

// CreateSource adds a new data source to a project
func CreateSource(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	var req store.ProjectSource
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.Name == "" || req.SourceType == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Source name and source type are required", nil)
		return
	}

	if err := store.CreateProjectSource(r.Context(), projectIID, &req); err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to create source", err.Error())
		return
	}

	// Mask sensitive credentials
	req.DSN = ""

	api.RespondJSON(w, http.StatusCreated, req, nil)
}

// DeleteSource removes a data source from a project
func DeleteSource(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	sourceIID := chi.URLParam(r, "sourceIid")

	if projectIID == "" || sourceIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Project ID and Source ID are required", nil)
		return
	}

	if err := store.DeleteProjectSource(r.Context(), projectIID, sourceIID); err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to delete project source", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Source deleted successfully"}, nil)
}
