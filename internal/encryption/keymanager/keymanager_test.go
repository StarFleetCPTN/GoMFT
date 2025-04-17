package keymanager

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKeyManager(t *testing.T) {
	// Test with custom env var
	customEnvVar := "CUSTOM_KEY_ENV_VAR"
	km := NewKeyManager(customEnvVar)
	assert.Equal(t, customEnvVar, km.envVarName)

	// Test with empty env var (should use default)
	km = NewKeyManager("")
	assert.Equal(t, encryption.DefaultKeyEnvVar, km.envVarName)
}

func TestGenerateKey(t *testing.T) {
	// Test generating key with default size
	key, err := GenerateKey(encryption.AES256KeySize)
	require.NoError(t, err)
	assert.Len(t, key, encryption.AES256KeySize)

	// Test generating key with custom size
	customSize := 64
	key, err = GenerateKey(customSize)
	require.NoError(t, err)
	assert.Len(t, key, customSize)

	// Test generating key with size smaller than minimum (should use minimum)
	key, err = GenerateKey(16)
	require.NoError(t, err)
	assert.Len(t, key, encryption.MinKeyLength)
}

func TestGenerateKeyString(t *testing.T) {
	// Test generating key string
	keyStr, err := GenerateKeyString(encryption.AES256KeySize)
	require.NoError(t, err)
	assert.NotEmpty(t, keyStr)
}

func TestValidateKeyLength(t *testing.T) {
	// Test valid key length
	key := make([]byte, encryption.MinKeyLength)
	assert.True(t, ValidateKeyLength(key))

	// Test invalid key length
	key = make([]byte, encryption.MinKeyLength-1)
	assert.False(t, ValidateKeyLength(key))
}

func TestKeyManager_Initialize(t *testing.T) {
	// Setup test environment
	testEnvVar := "TEST_ENCRYPTION_KEY"
	validKey, err := GenerateKey(encryption.AES256KeySize)
	require.NoError(t, err)
	validKeyBase64 := encodeToBase64(validKey)

	t.Run("Valid key in environment", func(t *testing.T) {
		// Set a valid key in environment
		os.Setenv(testEnvVar, validKeyBase64)
		defer os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.Initialize()
		require.NoError(t, err)

		// Check that key is properly stored
		key, err := km.GetPrimaryKey()
		require.NoError(t, err)
		assert.Equal(t, validKey, key)
	})

	t.Run("Missing key in environment", func(t *testing.T) {
		os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.Initialize()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "encryption key not provided")
	})

	t.Run("Invalid key format", func(t *testing.T) {
		os.Setenv(testEnvVar, "not-a-valid-base64-or-hex-key")
		defer os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.Initialize()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("Key too short", func(t *testing.T) {
		shortKey := make([]byte, encryption.MinKeyLength-1)
		os.Setenv(testEnvVar, encodeToBase64(shortKey))
		defer os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.Initialize()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too short")
	})
}

func TestKeyManager_StoreKeyEnvironment(t *testing.T) {
	testEnvVar := "TEST_STORE_KEY"
	km := NewKeyManager(testEnvVar)

	// Generate a valid key
	key, err := GenerateKey(encryption.AES256KeySize)
	require.NoError(t, err)

	// Store the key
	err = km.StoreKeyEnvironment(key)
	require.NoError(t, err)

	// Verify key is stored in environment
	envValue := os.Getenv(testEnvVar)
	assert.NotEmpty(t, envValue)

	// Verify key is stored in KeyManager
	storedKey, err := km.GetPrimaryKey()
	require.NoError(t, err)
	assert.Equal(t, key, storedKey)

	// Clean up
	os.Unsetenv(testEnvVar)
}

func TestKeyManager_GetPrimaryKey_NotInitialized(t *testing.T) {
	km := NewKeyManager("NONEXISTENT_KEY")
	key, err := km.GetPrimaryKey()
	require.Error(t, err)
	assert.Nil(t, key)
	assert.Contains(t, err.Error(), "not initialized")
}

// Helper function to encode bytes to base64
func encodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
