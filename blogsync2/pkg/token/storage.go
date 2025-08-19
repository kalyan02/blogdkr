package token

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

type SecureStorage struct {
	filePath string
	password string
}

type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func NewSecureStorage(filePath, password string) (*SecureStorage, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create token directory: %w", err)
	}

	return &SecureStorage{
		filePath: filePath,
		password: password,
	}, nil
}

func GetDefaultTokenPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./.dropbox_tokens"
	}
	return filepath.Join(homeDir, ".dropbox_tokens")
}

func (s *SecureStorage) SaveToken(accessToken, refreshToken string, expiresAt time.Time) error {
	tokenData := TokenData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}

	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	encryptedData, err := s.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt token data: %w", err)
	}

	if err := os.WriteFile(s.filePath, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

func (s *SecureStorage) LoadToken() (*TokenData, error) {
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("token file does not exist")
	}

	encryptedData, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	jsonData, err := s.decrypt(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token data: %w", err)
	}

	var tokenData TokenData
	if err := json.Unmarshal(jsonData, &tokenData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	return &tokenData, nil
}

func (s *SecureStorage) HasValidToken() bool {
	tokenData, err := s.LoadToken()
	if err != nil {
		return false
	}

	// Check if token expires within next 5 minutes
	return time.Now().Add(5 * time.Minute).Before(tokenData.ExpiresAt)
}

func (s *SecureStorage) encrypt(plaintext []byte) ([]byte, error) {
	// Derive key from password using PBKDF2
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	key := pbkdf2.Key([]byte(s.password), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Use GCM for authenticated encryption
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Combine salt + nonce + ciphertext
	result := make([]byte, len(salt)+len(nonce)+len(ciphertext))
	copy(result, salt)
	copy(result[len(salt):], nonce)
	copy(result[len(salt)+len(nonce):], ciphertext)

	// Base64 encode the result
	encoded := base64.StdEncoding.EncodeToString(result)
	return []byte(encoded), nil
}

func (s *SecureStorage) decrypt(ciphertext []byte) ([]byte, error) {
	// Base64 decode
	decoded, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return nil, err
	}

	if len(decoded) < 28 { // 16 (salt) + 12 (nonce) minimum
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract salt, nonce, and ciphertext
	salt := decoded[:16]
	nonce := decoded[16:28]
	encryptedData := decoded[28:]

	// Derive key from password using PBKDF2
	key := pbkdf2.Key([]byte(s.password), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}