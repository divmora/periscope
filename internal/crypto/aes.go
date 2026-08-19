package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

// getMasterKey retrieves the AES encryption key from the environment.
func getMasterKey() ([]byte, error) {
	keyStr := os.Getenv("AES_ENCRYPTION_KEY")
	if keyStr == "" {
		return nil, errors.New("AES_ENCRYPTION_KEY environment variable is not set")
	}
	key := []byte(keyStr)
	if len(key) != 32 {
		return nil, errors.New("AES_ENCRYPTION_KEY must be exactly 32 bytes for AES-256")
	}
	return key, nil
}

// Encrypt encrypts a plaintext string using AES-GCM and returns a base64 encoded string.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := getMasterKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 encoded AES-GCM string.
func Decrypt(encryptedBase64 string) (string, error) {
	if encryptedBase64 == "" {
		return "", nil
	}

	key, err := getMasterKey()
	if err != nil {
		return "", err
	}

	enc, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(enc) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
