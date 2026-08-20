package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRequireAuth(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-jwt")
	defer os.Unsetenv("JWT_SECRET")

	token, err := GenerateToken(10, "member")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(UserIDKey)
		globalRole := r.Context().Value(GlobalRoleKey)
		if userID != 10 || globalRole != "member" {
			t.Errorf("Unexpected context values: userID=%v, globalRole=%v", userID, globalRole)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuth(nextHandler)

	// 1. Test with Bearer Header
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 with Bearer token, got %d", rr.Code)
	}

	// 2. Test with HttpOnly Cookie
	req = httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "periscope_session", Value: token})
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 with cookie, got %d", rr.Code)
	}

	// 3. Test missing auth
	req = httptest.NewRequest("GET", "/api/test", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for missing auth, got %d", rr.Code)
	}

	// 4. Test invalid token
	req = httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for invalid token, got %d", rr.Code)
	}
}

func TestRequireGlobalAdmin(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireGlobalAdmin(nextHandler)

	// 1. Admin allowed
	req := httptest.NewRequest("GET", "/api/admin", nil)
	ctx := context.WithValue(req.Context(), GlobalRoleKey, "admin")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for admin, got %d", rr.Code)
	}

	// 2. Member forbidden
	req = httptest.NewRequest("GET", "/api/admin", nil)
	ctx = context.WithValue(req.Context(), GlobalRoleKey, "member")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for non-admin, got %d", rr.Code)
	}
}

func TestRequireProjectMember(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireProjectMember(nextHandler)

	// 1. Project Admin allowed
	req := httptest.NewRequest("GET", "/api/project/action", nil)
	ctx := context.WithValue(req.Context(), ProjectRoleKey, RoleAdmin)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for admin, got %d", rr.Code)
	}

	// 2. Project Member allowed
	req = httptest.NewRequest("GET", "/api/project/action", nil)
	ctx = context.WithValue(req.Context(), ProjectRoleKey, RoleMember)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for member, got %d", rr.Code)
	}

	// 3. Project Viewer forbidden
	req = httptest.NewRequest("GET", "/api/project/action", nil)
	ctx = context.WithValue(req.Context(), ProjectRoleKey, RoleViewer)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for viewer, got %d", rr.Code)
	}
}

func TestRequireProjectAdmin(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireProjectAdmin(nextHandler)

	// 1. Project Admin allowed
	req := httptest.NewRequest("GET", "/api/project/config", nil)
	ctx := context.WithValue(req.Context(), ProjectRoleKey, RoleAdmin)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for project admin, got %d", rr.Code)
	}

	// 2. Project Member forbidden
	req = httptest.NewRequest("GET", "/api/project/config", nil)
	ctx = context.WithValue(req.Context(), ProjectRoleKey, RoleMember)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for project member, got %d", rr.Code)
	}

	// 3. Project Viewer forbidden
	req = httptest.NewRequest("GET", "/api/project/config", nil)
	ctx = context.WithValue(req.Context(), ProjectRoleKey, RoleViewer)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for project viewer, got %d", rr.Code)
	}
}

func TestRequireProjectAccessSuperAdmin(t *testing.T) {
	// Setup Chi routing context with projectId param
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), GlobalRoleKey, "admin")
			ctx = context.WithValue(ctx, UserIDKey, 1)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Route("/projects/{projectId}", func(r chi.Router) {
		r.Use(RequireProjectAccess)
		r.Get("/test", func(w http.ResponseWriter, req *http.Request) {
			role := req.Context().Value(ProjectRoleKey)
			projectIID := req.Context().Value(ProjectIIDKey)
			if role != RoleAdmin || projectIID != "test-proj-uuid" {
				t.Errorf("Expected superadmin to get RoleAdmin on project, got role=%v, projectIID=%v", role, projectIID)
			}
			w.WriteHeader(http.StatusOK)
		})
	})

	req := httptest.NewRequest("GET", "/projects/test-proj-uuid/test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for super admin project access, got %d", rr.Code)
	}
}
