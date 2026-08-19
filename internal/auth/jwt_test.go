package auth

import (
	"os"
	"testing"
)

func TestGenerateAndValidateToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-jwt")
	defer os.Unsetenv("JWT_SECRET")

	userID := 42
	globalRole := "admin"

	// 1. Generate Token
	token, err := GenerateToken(userID, globalRole)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Fatal("Generated token is empty")
	}

	// 2. Validate Token
	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, claims.UserID)
	}

	if claims.GlobalRole != globalRole {
		t.Errorf("Expected GlobalRole %s, got %s", globalRole, claims.GlobalRole)
	}
}

func TestValidateInvalidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-jwt")
	defer os.Unsetenv("JWT_SECRET")

	_, err := ValidateToken("this.is.not.a.valid.jwt")
	if err == nil {
		t.Fatal("Expected error for invalid token string")
	}
}

func TestValidateTokenWithWrongSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-jwt")
	token, _ := GenerateToken(1, "admin")

	// Change secret
	os.Setenv("JWT_SECRET", "a-completely-different-secret")
	defer os.Unsetenv("JWT_SECRET")

	_, err := ValidateToken(token)
	if err == nil {
		t.Fatal("Expected error when validating token with wrong secret")
	}
}
