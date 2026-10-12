package handlers

import (
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/go-chi/chi/v5"
)

type QueryItem struct {
	ID       string `json:"id"`
	SQL      string `json:"sql"`
	Duration string `json:"duration"`
	Time     string `json:"time"`
}

func GetQueries(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	if projectID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	mockData := []QueryItem{
		{ID: "1", SQL: "select * from \"users\" where \"id\" = 1 limit 1", Duration: "2.40ms", Time: "21 Jul 2026 11:45 AM"},
		{ID: "2", SQL: "update \"sessions\" set \"last_activity\" = 1689940120 where \"id\" = 'abc'", Duration: "1.10ms", Time: "21 Jul 2026 10:42 AM"},
		{ID: "3", SQL: "select count(*) as aggregate from \"telescope_entries\" where \"type\" = 'request'", Duration: "45.00ms", Time: "21 Jul 2026 09:30 AM"},
	}

	meta := api.APIMeta{
		Total:       3,
		PerPage:     10,
		CurrentPage: 1,
		LastPage:    1,
	}

	api.RespondJSON(w, http.StatusOK, mockData, meta)
}
