package crypto

import (
	"os"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Set a valid 32-byte master key for testing
	os.Setenv("AES_ENCRYPTION_KEY", "12345678901234567890123456789012")
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	original := "mysql://periscope:secret@localhost:3306/laravel"

	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if encrypted == original {
		t.Fatalf("Encrypted string matches original: %s", encrypted)
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if decrypted != original {
		t.Fatalf("Expected '%s', got '%s'", original, decrypted)
	}
}

func TestEncryptEmptyString(t *testing.T) {
	encrypted, err := Encrypt("")
	if err != nil {
		t.Fatalf("Encrypting empty string should not fail: %v", err)
	}
	if encrypted != "" {
		t.Fatalf("Expected empty string, got: %s", encrypted)
	}
}

func TestDecryptEmptyString(t *testing.T) {
	decrypted, err := Decrypt("")
	if err != nil {
		t.Fatalf("Decrypting empty string should not fail: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("Expected empty string, got: %s", decrypted)
	}
}

func TestMissingKey(t *testing.T) {
	os.Unsetenv("AES_ENCRYPTION_KEY")
	_, err := Encrypt("test")
	if err == nil {
		t.Fatalf("Expected error when AES_ENCRYPTION_KEY is missing")
	}
}

func TestInvalidKeyLength(t *testing.T) {
	os.Setenv("AES_ENCRYPTION_KEY", "too_short")
	defer os.Unsetenv("AES_ENCRYPTION_KEY")

	_, err := Encrypt("test")
	if err == nil {
		t.Fatalf("Expected error when AES_ENCRYPTION_KEY is not 32 bytes")
	}
}
