package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/olap"
	"github.com/go-chi/chi/v5"
)

type ExecuteQueryRequest struct {
	Query string `json:"query"`
}

// ExecuteSQLQuery runs an ad-hoc analytical SQL query via embedded DuckDB
func ExecuteSQLQuery(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	var req ExecuteQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		api.RespondError(w, http.StatusBadRequest, "EMPTY_QUERY", "SQL query cannot be empty", nil)
		return
	}

	engine, err := olap.GetEngine()
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "OLAP_ERROR", "Failed to initialize query engine", err.Error())
		return
	}

	result, err := engine.ExecuteQuery(r.Context(), query)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "QUERY_ERROR", err.Error(), nil)
		return
	}

	api.RespondJSON(w, http.StatusOK, result, nil)
}
