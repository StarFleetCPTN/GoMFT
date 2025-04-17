package encryption

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
)

// SecureRandomReader is the reader used for generating random data
// It's a variable to allow for easier testing by replacing with a mock
var SecureRandomReader io.Reader = rand.Reader

// KeyManager is the interface for key management operations
type KeyManager interface {
	// Initialize initializes the key manager with a key from the environment
	Initialize() error

	// GetPrimaryKey returns the primary encryption key
	GetPrimaryKey() ([]byte, error)

	// GetEnvironmentVariableName returns the name of the environment variable used for the key
	GetEnvironmentVariableName() string

	// StoreKeyEnvironment stores the encryption key in the specified environment variable
	StoreKeyEnvironment(key []byte) error
}

// defaultKeyManager is the implementation of KeyManager
type defaultKeyManager struct {
	// primaryKey is the main encryption key used for AES-256 encryption
	primaryKey []byte

	// envVarName is the name of the environment variable that stores the key
	envVarName string

	// mutex to protect key access
	mutex sync.RWMutex
}

var (
	// Global key manager instance
	globalKeyManager     KeyManager
	globalKeyManagerOnce sync.Once
)

// InitializeKeyManager initializes the default key manager instance
// It retrieves the key from the environment variable GOMFT_ENCRYPTION_KEY by default.
// Only the first call to this function will actually initialize the key manager,
// subsequent calls will return the already initialized instance.
func InitializeKeyManager(envVar string) error {
	var initErr error

	globalKeyManagerOnce.Do(func() {
		// Create key manager
		globalKeyManager = NewKeyManager(envVar)

		// Initialize with key from environment
		initErr = globalKeyManager.Initialize()
	})

	return initErr
}

// GetKeyManager returns the global key manager instance
// If the key manager has not been initialized, this will return nil
func GetKeyManager() KeyManager {
	return globalKeyManager
}

// NewKeyManager creates a new KeyManager instance
func NewKeyManager(envVarName string) KeyManager {
	if envVarName == "" {
		envVarName = DefaultKeyEnvVar
	}

	return &defaultKeyManager{
		envVarName: envVarName,
	}
}

// decodeKey attempts to decode a key string from hex or base64 format
func decodeKey(keyStr string) ([]byte, error) {
	// Try hex decoding first
	keyBytes, err := hex.DecodeString(keyStr)
	if err == nil {
		return keyBytes, nil
	}

	// If hex decoding fails, try base64
	keyBytes, err = base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("key must be valid hex or base64 encoded: %w", err)
	}

	return keyBytes, nil
}

// Initialize loads the encryption key from the environment
// and validates it meets security requirements
func (km *defaultKeyManager) Initialize() error {
	// Get key from environment variable
	keyStr := os.Getenv(km.envVarName)
	if keyStr == "" {
		return fmt.Errorf(ErrKeyNotProvided, km.envVarName)
	}

	// Attempt to decode the key
	keyBytes, err := decodeKey(keyStr)
	if err != nil {
		return fmt.Errorf(ErrInvalidKey, err.Error())
	}

	// Validate key length
	if len(keyBytes) < MinKeyLength {
		return fmt.Errorf(ErrKeyTooShort, MinKeyLength)
	}

	// Store the key
	km.mutex.Lock()
	km.primaryKey = keyBytes
	km.mutex.Unlock()

	return nil
}

// GetPrimaryKey returns the primary encryption key
func (km *defaultKeyManager) GetPrimaryKey() ([]byte, error) {
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

// GetEnvironmentVariableName returns the name of the environment variable used for the key
func (km *defaultKeyManager) GetEnvironmentVariableName() string {
	return km.envVarName
}

// StoreKeyEnvironment stores the encryption key in the specified environment variable
// This is generally only used for development or testing purposes
func (km *defaultKeyManager) StoreKeyEnvironment(key []byte) error {
	if !ValidateKeyLength(key) {
		return fmt.Errorf(ErrKeyTooShort, MinKeyLength)
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

// GenerateKey generates a new random encryption key of the specified size
func GenerateKey(size int) ([]byte, error) {
	if size < MinKeyLength {
		size = MinKeyLength
	}

	key := make([]byte, size)
	_, err := SecureRandomReader.Read(key)
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
	return len(key) >= MinKeyLength
}
