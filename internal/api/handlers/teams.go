package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/store"
	"github.com/go-chi/chi/v5"
)

type CreateTeamRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Role           string   `json:"role"`
	AllowedSources []string `json:"allowed_sources"`
}

type UpdateTeamRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Role           string   `json:"role"`
	AllowedSources []string `json:"allowed_sources"`
}

type AddTeamMemberRequest struct {
	UserIdentifier string `json:"user_identifier"`
}

// ListProjectTeams returns all teams in a project with their member counts
func ListProjectTeams(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	teams, err := store.ListProjectTeams(r.Context(), projectIID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list project teams", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, teams, nil)
}

// CreateProjectTeam creates a new team with role and source permissions
func CreateProjectTeam(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	if projectIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
		return
	}

	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.Name == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Team name is required", nil)
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}

	team := store.ProjectTeam{
		Name:           req.Name,
		Description:    req.Description,
		Role:           req.Role,
		AllowedSources: req.AllowedSources,
	}

	if err := store.CreateProjectTeam(r.Context(), projectIID, &team); err != nil {
		if errors.Is(err, store.ErrInvalidRole) {
			api.RespondError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be viewer, member, or admin", nil)
			return
		}
		if errors.Is(err, store.ErrProjectNotFound) {
			api.RespondError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to create team", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusCreated, team, nil)
}

// UpdateProjectTeam updates team role, name, description, and source permissions
func UpdateProjectTeam(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	teamIID := chi.URLParam(r, "teamId")

	if projectIID == "" || teamIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Project ID and Team ID are required", nil)
		return
	}

	var req UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.Name == "" || req.Role == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Team name and role are required", nil)
		return
	}

	err := store.UpdateProjectTeam(r.Context(), projectIID, teamIID, req.Name, req.Description, req.Role, req.AllowedSources)
	if err != nil {
		if errors.Is(err, store.ErrTeamNotFound) {
			api.RespondError(w, http.StatusNotFound, "TEAM_NOT_FOUND", "Team not found in this project", nil)
			return
		}
		if errors.Is(err, store.ErrInvalidRole) {
			api.RespondError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be viewer, member, or admin", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to update team", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Team updated successfully"}, nil)
}

// DeleteProjectTeam removes a team from a project
func DeleteProjectTeam(w http.ResponseWriter, r *http.Request) {
	projectIID := chi.URLParam(r, "projectId")
	teamIID := chi.URLParam(r, "teamId")

	if projectIID == "" || teamIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Project ID and Team ID are required", nil)
		return
	}

	err := store.DeleteProjectTeam(r.Context(), projectIID, teamIID)
	if err != nil {
		if errors.Is(err, store.ErrTeamNotFound) {
			api.RespondError(w, http.StatusNotFound, "TEAM_NOT_FOUND", "Team not found in this project", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to delete team", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Team deleted successfully"}, nil)
}

// ListTeamMembers returns all members assigned to a team
func ListTeamMembers(w http.ResponseWriter, r *http.Request) {
	teamIID := chi.URLParam(r, "teamId")
	if teamIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Team ID is required", nil)
		return
	}

	members, err := store.ListTeamMembers(r.Context(), teamIID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to list team members", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, members, nil)
}

// AddTeamMember adds a user to a team
func AddTeamMember(w http.ResponseWriter, r *http.Request) {
	teamIID := chi.URLParam(r, "teamId")
	if teamIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Team ID is required", nil)
		return
	}

	var req AddTeamMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", err.Error())
		return
	}

	if req.UserIdentifier == "" {
		api.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "user_identifier is required", nil)
		return
	}

	if err := store.AddTeamMember(r.Context(), teamIID, req.UserIdentifier); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			api.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found with provided username/email", nil)
			return
		}
		if errors.Is(err, store.ErrTeamNotFound) {
			api.RespondError(w, http.StatusNotFound, "TEAM_NOT_FOUND", "Team not found", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to add member to team", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusCreated, map[string]string{"message": "User added to team successfully"}, nil)
}

// RemoveTeamMember removes a user from a team
func RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	teamIID := chi.URLParam(r, "teamId")
	userIID := chi.URLParam(r, "userIid")

	if teamIID == "" || userIID == "" {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PARAMS", "Team ID and User ID are required", nil)
		return
	}

	if err := store.RemoveTeamMember(r.Context(), teamIID, userIID); err != nil {
		if errors.Is(err, store.ErrMemberNotFound) {
			api.RespondError(w, http.StatusNotFound, "MEMBER_NOT_FOUND", "User is not a member of this team", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to remove member from team", err.Error())
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "User removed from team successfully"}, nil)
}
