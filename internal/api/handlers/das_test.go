package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divmora/periscope/internal/olap"
	"github.com/go-chi/chi/v5"
)

func TestGetDASStats_MissingProject(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/projects//das/stats", nil)
	rr := httptest.NewRecorder()

	GetDASStats(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request, got %d", rr.Code)
	}
}

func TestGetDASStats_Success(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/projects/{projectId}/das/stats", GetDASStats)

	req := httptest.NewRequest("GET", "/api/projects/proj-1234/das/stats", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool          `json:"success"`
		Data    olap.DASStats `json:"data"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success true, got false")
	}

	if resp.Data.TotalQueries <= 0 {
		t.Errorf("expected positive total queries, got %d", resp.Data.TotalQueries)
	}
}

func TestGetDASActivity_Success(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/projects/{projectId}/das/activity", GetDASActivity)

	req := httptest.NewRequest("GET", "/api/projects/proj-1234/das/activity?engine=direct&per_page=5", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Verify standard envelope format is returned
	var resp struct {
		Success bool             `json:"success"`
		Data    []olap.DASRecord `json:"data"`
		Error   any              `json:"error"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response envelope: %v", err)
	}
}
