package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/auth"
	"github.com/divmora/periscope/internal/store"
	"github.com/go-chi/chi/v5"
)

// GetProjects lists all projects
func GetProjects(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(auth.UserIDKey).(int)
	globalRole, _ := r.Context().Value(auth.GlobalRoleKey).(string)

	projects, err := store.ListProjects(r.Context(), userID, globalRole)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list projects", err.Error())
		return
	}

	// Ensure we return an empty array instead of null if no projects exist
	if projects == nil {
		projects = []store.Project{}
	}

	api.RespondJSON(w, http.StatusOK, projects, nil)
}

// GetProjectConfig gets a specific project's configuration
func GetProjectConfig(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	project, err := store.GetProject(r.Context(), projectID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch project", err.Error())
		return
	}
	if project == nil {
		api.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Project not found", nil)
		return
	}

	api.RespondJSON(w, http.StatusOK, project, nil)
}

// GetSystemStorage returns global default storage configuration
func GetSystemStorage(w http.ResponseWriter, r *http.Request) {
	defaultBucket := os.Getenv("S3_BUCKET")
	if defaultBucket == "" {
		defaultBucket = "periscope-telemetry"
	}
	defaultEndpoint := os.Getenv("S3_ENDPOINT")
	defaultRegion := os.Getenv("S3_REGION")
	if defaultRegion == "" {
		defaultRegion = "us-east-1"
	}

	api.RespondJSON(w, http.StatusOK, map[string]any{
		"storage_type":     "s3",
		"storage_bucket":   defaultBucket,
		"storage_endpoint": defaultEndpoint,
		"storage_region":   defaultRegion,
		"is_default_ready": true,
	}, nil)
}

// CreateProject creates a new project
func CreateProject(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(auth.UserIDKey).(int)

	var req store.Project
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.Name == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Project Name is required", nil)
		return
	}

	// Default Storage Target fallback if not specified by user
	if req.StorageBucket == "" || req.StorageBucket == "default" {
		defaultBucket := os.Getenv("S3_BUCKET")
		if defaultBucket == "" {
			defaultBucket = "periscope-telemetry"
		}
		req.StorageBucket = defaultBucket
		req.StorageEndpoint = os.Getenv("S3_ENDPOINT")
		req.StorageRegion = os.Getenv("S3_REGION")
		if req.StorageRegion == "" {
			req.StorageRegion = "us-east-1"
		}
		req.StorageAccessKey = os.Getenv("S3_ACCESS_KEY_ID")
		req.StorageSecretKey = os.Getenv("S3_SECRET_ACCESS_KEY")
		req.StorageType = "s3"
	}

	if req.StorageType == "" {
		req.StorageType = "s3"
	}

	if req.LakehouseRetentionDays <= 0 {
		req.LakehouseRetentionDays = 365
	}

	if err := store.CreateProject(r.Context(), userID, &req); err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to create project", err.Error())
		return
	}

	// Don't leak credentials back in the response
	req.SourceDSN = ""
	req.StorageAccessKey = ""
	req.StorageSecretKey = ""

	api.RespondJSON(w, http.StatusCreated, req, nil)
}

// UpdateProjectConfig updates an existing project
func UpdateProjectConfig(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	project, err := store.GetProject(r.Context(), projectID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch project", err.Error())
		return
	}
	if project == nil {
		api.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Project not found", nil)
		return
	}

	var req store.Project
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.Name != "" {
		project.Name = req.Name
	}
	if req.SourceDSN != "" {
		project.SourceDSN = req.SourceDSN
	}
	if req.StorageType != "" {
		project.StorageType = req.StorageType
	}
	if req.StorageBucket != "" {
		project.StorageBucket = req.StorageBucket
	}
	if req.StorageRegion != "" {
		project.StorageRegion = req.StorageRegion
	}
	if req.StorageEndpoint != "" {
		project.StorageEndpoint = req.StorageEndpoint
	}
	if req.StorageAccessKey != "" {
		project.StorageAccessKey = req.StorageAccessKey
	}
	if req.StorageSecretKey != "" {
		project.StorageSecretKey = req.StorageSecretKey
	}
	project.PruneEnabled = req.PruneEnabled
	if req.SourceRetentionDays >= 0 {
		project.SourceRetentionDays = req.SourceRetentionDays
	}
	if req.LakehouseRetentionDays > 0 {
		project.LakehouseRetentionDays = req.LakehouseRetentionDays
	}
	project.CompactionEnabled = req.CompactionEnabled

	if err := store.UpdateProject(r.Context(), project); err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to update project", err.Error())
		return
	}

	// Don't leak credentials back in the response
	project.SourceDSN = ""
	project.StorageAccessKey = ""
	project.StorageSecretKey = ""

	api.RespondJSON(w, http.StatusOK, project, nil)
}

// DeleteProject deletes a project
func DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	if projectID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Project ID is required", nil)
		return
	}

	if err := store.DeleteProject(r.Context(), projectID); err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			api.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Project not found", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to delete project", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Project deleted successfully"}, nil)
}
