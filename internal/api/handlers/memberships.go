package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/store"
	"github.com/go-chi/chi/v5"
)

type AddMemberRequest struct {
	UserIdentifier string   `json:"user_identifier"`
	Role           string   `json:"role"`
	AllowedSources []string `json:"allowed_sources,omitempty"`
}

type UpdateMemberRequest struct {
	Role           string   `json:"role"`
	AllowedSources []string `json:"allowed_sources,omitempty"`
}

// ListProjectMembers returns the list of members and their roles for a project
func ListProjectMembers(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	members, err := store.ListProjectMembers(r.Context(), projectIID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list project members", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, members, nil)
}

// AddProjectMember adds or updates a member to a project
func AddProjectMember(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.UserIdentifier == "" || req.Role == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "user_identifier and role are required", nil)
		return
	}

	member, err := store.AddProjectMember(r.Context(), projectIID, req.UserIdentifier, req.Role, req.AllowedSources)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			api.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found with provided username/email", nil)
			return
		}
		if errors.Is(err, store.ErrProjectNotFound) {
			api.RespondError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
			return
		}
		if errors.Is(err, store.ErrInvalidRole) {
			api.RespondError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be viewer, member, or admin", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to add project member", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusCreated, member, nil)
}

// UpdateProjectMember updates an existing project member's role
func UpdateProjectMember(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	userIID := chi.URLParam(r, "userIid")

	if projectIID == "" || userIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Project ID and User IID are required", nil)
		return
	}

	var req UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.Role == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "role is required", nil)
		return
	}

	err := store.UpdateProjectMemberRole(r.Context(), projectIID, userIID, req.Role, req.AllowedSources)
	if err != nil {
		if errors.Is(err, store.ErrMemberNotFound) {
			api.RespondError(w, http.StatusNotFound, "MEMBER_NOT_FOUND", "Member not found in this project", nil)
			return
		}
		if errors.Is(err, store.ErrInvalidRole) {
			api.RespondError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be viewer, member, or admin", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to update project member role", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Project member role updated successfully"}, nil)
}

// RemoveProjectMember removes a member from a project
func RemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	userIID := chi.URLParam(r, "userIid")

	if projectIID == "" || userIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Project ID and User IID are required", nil)
		return
	}

	err := store.RemoveProjectMember(r.Context(), projectIID, userIID)
	if err != nil {
		if errors.Is(err, store.ErrMemberNotFound) {
			api.RespondError(w, http.StatusNotFound, "MEMBER_NOT_FOUND", "Member not found in this project", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to remove member from project", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Member removed from project successfully"}, nil)
}
