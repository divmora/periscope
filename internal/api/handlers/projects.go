package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

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

// CreateProject creates a new project
func CreateProject(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(auth.UserIDKey).(int)

	var req store.Project
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.Name == "" || req.SourceDSN == "" || req.StorageBucket == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Name, DSN, and Bucket are required", nil)
		return
	}

	if req.StorageType == "" {
		req.StorageType = "s3"
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

	// Merge updates
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
	project.StorageRegion = req.StorageRegion
	project.StorageEndpoint = req.StorageEndpoint
	project.StorageAccessKey = req.StorageAccessKey
	project.StorageSecretKey = req.StorageSecretKey
	project.PruneEnabled = req.PruneEnabled

	if err := store.UpdateProject(r.Context(), project); err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to update project", err.Error())
		return
	}

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
