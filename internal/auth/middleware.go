package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/store"
	"github.com/go-chi/chi/v5"
)

type contextKey string

const (
	UserIDKey         contextKey = "userID"
	GlobalRoleKey     contextKey = "globalRole"
	ProjectRoleKey    contextKey = "projectRole"
	ProjectIIDKey     contextKey = "projectIID"
	AllowedSourcesKey contextKey = "allowedSources"
)

// Project roles
const (
	RoleViewer = "viewer"
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// RequireAuth validates the JWT from the Authorization header (Bearer) or periscope_session cookie
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// 1. Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// 2. Fallback to HttpOnly cookie
		if tokenString == "" {
			if cookie, err := r.Cookie("periscope_session"); err == nil {
				tokenString = cookie.Value
			}
		}

		if tokenString == "" {
			api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authentication token or cookie", nil)
			return
		}

		claims, err := ValidateToken(tokenString)
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

// RequireProjectAccess ensures the user has access to the project specified in the URL.
// Injects ProjectRoleKey and ProjectIIDKey into the request context.
func RequireProjectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalRole, _ := r.Context().Value(GlobalRoleKey).(string)
		userID, ok := r.Context().Value(UserIDKey).(int)

		projectIID := chi.URLParam(r, "projectId")
		if projectIID == "" {
			api.RespondError(w, http.StatusBadRequest, "INVALID_PROJECT", "Project ID is required in URL", nil)
			return
		}

		// Super Admins automatically have full admin access across all projects
		if globalRole == "admin" {
			ctx := context.WithValue(r.Context(), ProjectRoleKey, RoleAdmin)
			ctx = context.WithValue(ctx, ProjectIIDKey, projectIID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if !ok {
			api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User ID not found in context", nil)
			return
		}

		role, allowedSources, err := store.GetUserProjectPermissions(r.Context(), userID, projectIID)
		if err != nil {
			api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to verify project access", nil)
			return
		}

		if role == "" {
			api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "You do not have access to this project", nil)
			return
		}

		ctx := context.WithValue(r.Context(), ProjectRoleKey, role)
		ctx = context.WithValue(ctx, ProjectIIDKey, projectIID)
		ctx = context.WithValue(ctx, AllowedSourcesKey, allowedSources)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserCanAccessSource checks if the user is authorized to access the given sourceIID.
func UserCanAccessSource(ctx context.Context, sourceIID string) bool {
	if sourceIID == "" {
		return true
	}
	globalRole, _ := ctx.Value(GlobalRoleKey).(string)
	if globalRole == "admin" {
		return true
	}
	projectRole, _ := ctx.Value(ProjectRoleKey).(string)
	if projectRole == RoleAdmin {
		return true
	}
	allowed, ok := ctx.Value(AllowedSourcesKey).([]string)
	if !ok || len(allowed) == 0 {
		// Empty/nil allowed_sources indicates unrestricted access to all sources
		return true
	}
	for _, id := range allowed {
		if id == sourceIID {
			return true
		}
	}
	return false
}

// RequireProjectMember ensures the user has at least 'member' or 'admin' role on the scoped project
func RequireProjectMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ProjectRoleKey).(string)
		if role != RoleMember && role != RoleAdmin {
			api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Member or Admin privileges required for this action", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireProjectAdmin ensures the user has 'admin' role on the scoped project
func RequireProjectAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ProjectRoleKey).(string)
		if role != RoleAdmin {
			api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Admin privileges required for this action", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
