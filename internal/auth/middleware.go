package auth

import (
	"context"
	"net/http"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/store"
	"github.com/go-chi/chi/v5"
)

type contextKey string

const (
	UserIDKey     contextKey = "userID"
	GlobalRoleKey contextKey = "globalRole"
)

// RequireAuth validates the JWT from the periscope_session cookie
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("periscope_session")
		if err != nil {
			api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authentication cookie", nil)
			return
		}

		claims, err := ValidateToken(cookie.Value)
		if err != nil {
			api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token", nil)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, GlobalRoleKey, claims.GlobalRole)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireGlobalAdmin ensures the user has the "admin" global role
func RequireGlobalAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(GlobalRoleKey).(string)
		if !ok || role != "admin" {
			api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Super Admin privileges required", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireProjectAccess ensures the user is either a Global Admin or a member of the requested project
func RequireProjectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(GlobalRoleKey).(string)
		userID, ok := r.Context().Value(UserIDKey).(int)

		// Super Admins bypass project-level checks
		if role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		if !ok {
			api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User ID not found in context", nil)
			return
		}

		projectIID := chi.URLParam(r, "projectId")
		if projectIID == "" {
			api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required", nil)
			return
		}

		// Check project_memberships table
		hasAccess, err := store.CheckProjectAccess(r.Context(), userID, projectIID)
		if err != nil {
			api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to verify project access", nil)
			return
		}

		if !hasAccess {
			api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "You do not have access to this project", nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}
