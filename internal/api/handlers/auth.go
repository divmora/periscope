package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/divmora/periscope/internal/api"
	"github.com/divmora/periscope/internal/auth"
	"github.com/divmora/periscope/internal/store"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserResponse struct {
	IID        string `json:"iid"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	GlobalRole string `json:"global_role"`
	Status     string `json:"status"`
}

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

func Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.RespondError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid request body", nil)
		return
	}

	var user UserResponse
	var passwordHash string
	var internalID int

	// 1. Fetch user from database
	err := store.Pool.QueryRow(r.Context(), `
		SELECT id, iid, username, email, password_hash, global_role, status 
		FROM users 
		WHERE username = $1 OR email = $1
	`, req.Username).Scan(&internalID, &user.IID, &user.Username, &user.Email, &passwordHash, &user.GlobalRole, &user.Status)

	if err != nil {
		if err == pgx.ErrNoRows {
			api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", nil)
			return
		}
		api.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to fetch user", nil)
		return
	}

	if user.Status != StatusActive {
		api.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Account is inactive or suspended", nil)
		return
	}

	// 2. Compare passwords
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials", nil)
		return
	}

	// 3. Generate JWT
	tokenString, err := auth.GenerateToken(internalID, user.GlobalRole)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "AUTH_ERROR", "Failed to generate token", nil)
		return
	}

	// 4. Set HttpOnly Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "periscope_session",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	// 5. Return user metadata (no token in body)
	api.RespondJSON(w, http.StatusOK, user, nil)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	// Clear the cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "periscope_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"}, nil)
}

func Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(auth.UserIDKey).(int)
	if !ok {
		api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated", nil)
		return
	}

	var user UserResponse
	err := store.Pool.QueryRow(r.Context(), `
		SELECT iid, username, email, global_role 
		FROM users 
		WHERE id = $1
	`, userID).Scan(&user.IID, &user.Username, &user.Email, &user.GlobalRole)

	if err != nil {
		api.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not found", nil)
		return
	}

	api.RespondJSON(w, http.StatusOK, user, nil)
}
