package keymanager

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/starfleetcptn/gomft/internal/encryption"
)

// KeyManager handles the management of encryption keys
type KeyManager struct {
	// primaryKey is the main encryption key used for AES-256 encryption
	primaryKey []byte

	// envVarName is the name of the environment variable that stores the key
	envVarName string

	// mutex to protect key access
	mutex sync.RWMutex
}

// NewKeyManager creates a new KeyManager instance
func NewKeyManager(envVarName string) *KeyManager {
	if envVarName == "" {
		envVarName = encryption.DefaultKeyEnvVar
	}

	return &KeyManager{
		envVarName: envVarName,
	}
}

// Initialize loads the encryption key from the environment
// and validates it meets security requirements
func (km *KeyManager) Initialize() error {
	// Try loading .env file if exists
	_ = godotenv.Load()

	// Get key from environment variable
	keyStr := os.Getenv(km.envVarName)
	if keyStr == "" {
		return fmt.Errorf(encryption.ErrKeyNotProvided, km.envVarName)
	}

	// Attempt to decode the key - we support both hex and base64 formats
	var keyBytes []byte
	var err error

	// Try hex decoding first
	keyBytes, err = hex.DecodeString(keyStr)
	if err != nil {
		// If hex decoding fails, try base64
		keyBytes, err = base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return fmt.Errorf(encryption.ErrInvalidKey, "key must be valid hex or base64 encoded")
		}
	}

	// Validate key length
	if len(keyBytes) < encryption.MinKeyLength {
		return fmt.Errorf(encryption.ErrKeyTooShort, encryption.MinKeyLength)
	}

	// Store the key
	km.mutex.Lock()
	km.primaryKey = keyBytes
	km.mutex.Unlock()

	return nil
}

// GetPrimaryKey returns the primary encryption key
func (km *KeyManager) GetPrimaryKey() ([]byte, error) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if km.primaryKey == nil || len(km.primaryKey) == 0 {
		return nil, fmt.Errorf("encryption key not initialized")
	}

	// Return a copy of the key to prevent modification
	keyCopy := make([]byte, len(km.primaryKey))
	copy(keyCopy, km.primaryKey)

	return keyCopy, nil
}

// GenerateKey generates a new random encryption key of the specified size
func GenerateKey(size int) ([]byte, error) {
	if size < encryption.MinKeyLength {
		size = encryption.MinKeyLength
	}

	key := make([]byte, size)
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	return key, nil
}

// GenerateKeyString generates a new random encryption key and returns it as a base64 string
func GenerateKeyString(size int) (string, error) {
	key, err := GenerateKey(size)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(key), nil
}

// ValidateKeyLength checks if the provided key meets the minimum length requirement
func ValidateKeyLength(key []byte) bool {
	return len(key) >= encryption.MinKeyLength
}

// StoreKeyEnvironment stores the encryption key in the specified environment variable
// This is generally only used for development or testing purposes
func (km *KeyManager) StoreKeyEnvironment(key []byte) error {
	if !ValidateKeyLength(key) {
		return fmt.Errorf(encryption.ErrKeyTooShort, encryption.MinKeyLength)
	}

	keyStr := base64.StdEncoding.EncodeToString(key)
	err := os.Setenv(km.envVarName, keyStr)
	if err != nil {
		return fmt.Errorf("failed to set environment variable: %w", err)
	}

	// Update the stored key
	km.mutex.Lock()
	km.primaryKey = key
	km.mutex.Unlock()

	return nil
}

// GetEnvironmentVariableName returns the name of the environment variable used for the key
func (km *KeyManager) GetEnvironmentVariableName() string {
	return km.envVarName
}
