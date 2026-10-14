package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/auth"
	"github.com/divmora/periscope/internal/olap"
	"github.com/divmora/periscope/internal/store"
	"github.com/go-chi/chi/v5"
)

type DASResponseMeta struct {
	Total       int    `json:"total"`
	PerPage     int    `json:"per_page"`
	CurrentPage int    `json:"current_page"`
	LastPage    int    `json:"last_page"`
	Engine      string `json:"engine"`
	SourceIID   string `json:"source_iid,omitempty"`
}

// GetDASActivity returns database activity records using either Direct S3 Parquet (DuckDB) or Athena.
func GetDASActivity(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	// 1. Parse pagination and filter parameters
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	filter := olap.DASFilter{
		Search:    strings.TrimSpace(q.Get("search")),
		DBUser:    strings.TrimSpace(q.Get("user")),
		ClientIP:  strings.TrimSpace(q.Get("ip")),
		Database:  strings.TrimSpace(q.Get("database")),
		Command:   strings.TrimSpace(q.Get("command")),
		Status:    strings.TrimSpace(q.Get("status")),
		StartDate: strings.TrimSpace(q.Get("start_date")),
		EndDate:   strings.TrimSpace(q.Get("end_date")),
		Limit:     perPage,
		Offset:    (page - 1) * perPage,
	}

	// 2. Fetch project sources to identify the Aurora DAS configuration
	sources, err := store.ListProjectSources(r.Context(), projectIID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "SOURCE_FETCH_ERROR", "Failed to retrieve project sources", err.Error())
		return
	}

	var dasSource *store.ProjectSource
	targetSourceIID := q.Get("source_iid")
	if targetSourceIID != "" && !auth.UserCanAccessSource(r.Context(), targetSourceIID) {
		api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this data source", nil)
		return
	}

	for i := range sources {
		s := &sources[i]
		if s.SourceType == "aurora_das" && auth.UserCanAccessSource(r.Context(), s.IID) {
			if targetSourceIID == "" || s.IID == targetSourceIID {
				dasSource = s
				break
			}
		}
	}

	// Determine engine mode ("direct" vs "athena")
	engineMode := strings.ToLower(q.Get("engine"))
	var sourceConfig map[string]any
	var sourceIID string
	if dasSource != nil {
		sourceConfig = dasSource.Config
		sourceIID = dasSource.IID
		if engineMode == "" {
			if cfgEngine, ok := sourceConfig["engine"].(string); ok && cfgEngine != "" {
				engineMode = strings.ToLower(cfgEngine)
			}
		}
	}

	if engineMode != "athena" {
		engineMode = "direct"
	}
	if sourceConfig == nil {
		sourceConfig = make(map[string]any)
	}

	// 3. Dispatch to selected OLAP Engine
	var records []olap.DASRecord
	var total int

	if engineMode == "athena" {
		records, total, err = olap.QueryDASAthena(r.Context(), sourceConfig, filter)
	} else {
		records, total, err = olap.QueryDASDirect(r.Context(), sourceConfig, filter)
	}

	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "DAS_QUERY_FAILED", err.Error(), nil)
		return
	}

	lastPage := 1
	if total > 0 && perPage > 0 {
		lastPage = (total + perPage - 1) / perPage
	}

	meta := DASResponseMeta{
		Total:       total,
		PerPage:     perPage,
		CurrentPage: page,
		LastPage:    lastPage,
		Engine:      engineMode,
		SourceIID:   sourceIID,
	}

	api.RespondJSON(w, http.StatusOK, records, meta)
}

// GetDASStats aggregates activity KPI metrics across database commands, users, and errors.
func GetDASStats(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	targetSourceIID := r.URL.Query().Get("source_iid")
	if targetSourceIID != "" && !auth.UserCanAccessSource(r.Context(), targetSourceIID) {
		api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this data source", nil)
		return
	}

	var sourceConfig map[string]any
	sources, err := store.ListProjectSources(r.Context(), projectIID)
	if err == nil {
		for i := range sources {
			s := &sources[i]
			if s.SourceType == "aurora_das" && auth.UserCanAccessSource(r.Context(), s.IID) {
				if targetSourceIID == "" || s.IID == targetSourceIID {
					sourceConfig = s.Config
					break
				}
			}
		}
	}
	if sourceConfig == nil {
		sourceConfig = make(map[string]any)
	}

	stats, err := olap.GetDASStatsDirect(r.Context(), sourceConfig)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "STATS_CALCULATION_FAILED", err.Error(), nil)
		return
	}

	api.RespondJSON(w, http.StatusOK, stats, nil)
}
