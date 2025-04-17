package encryption

import (
	"encoding/base64"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKeyManager(t *testing.T) {
	// Test with custom env var
	customEnvVar := "CUSTOM_KEY_ENV_VAR"
	km := NewKeyManager(customEnvVar)
	assert.Equal(t, customEnvVar, km.GetEnvironmentVariableName())

	// Test with empty env var (should use default)
	km = NewKeyManager("")
	assert.Equal(t, DefaultKeyEnvVar, km.GetEnvironmentVariableName())
}

func TestKeyManager_Initialize(t *testing.T) {
	// Setup test environment
	testEnvVar := "TEST_ENCRYPTION_KEY"
	validKey, err := GenerateKey(AES256KeySize)
	require.NoError(t, err)
	validKeyBase64 := base64.StdEncoding.EncodeToString(validKey)

	t.Run("Valid key in environment", func(t *testing.T) {
		// Set a valid key in environment
		os.Setenv(testEnvVar, validKeyBase64)
		defer os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.(KeyManager).Initialize()
		require.NoError(t, err)

		// Check that key is properly stored
		key, err := km.GetPrimaryKey()
		require.NoError(t, err)
		assert.Equal(t, validKey, key)
	})

	t.Run("Missing key in environment", func(t *testing.T) {
		os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.(KeyManager).Initialize()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "encryption key not provided")
	})

	t.Run("Invalid key format", func(t *testing.T) {
		os.Setenv(testEnvVar, "not-a-valid-base64-or-hex-key")
		defer os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.(KeyManager).Initialize()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("Key too short", func(t *testing.T) {
		shortKey := make([]byte, MinKeyLength-1)
		os.Setenv(testEnvVar, base64.StdEncoding.EncodeToString(shortKey))
		defer os.Unsetenv(testEnvVar)

		km := NewKeyManager(testEnvVar)
		err := km.(KeyManager).Initialize()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too short")
	})
}

func TestGlobalKeyManager(t *testing.T) {
	// Reset global key manager
	globalKeyManager = nil
	globalKeyManagerOnce = sync.Once{}

	// Set a valid key in environment
	testEnvVar := DefaultKeyEnvVar
	validKey, err := GenerateKey(AES256KeySize)
	require.NoError(t, err)
	validKeyBase64 := base64.StdEncoding.EncodeToString(validKey)
	os.Setenv(testEnvVar, validKeyBase64)
	defer os.Unsetenv(testEnvVar)

	// Initialize global key manager
	err = InitializeKeyManager("")
	require.NoError(t, err)

	// Get global key manager
	km := GetKeyManager()
	require.NotNil(t, km)

	// Check that key is properly stored
	key, err := km.GetPrimaryKey()
	require.NoError(t, err)
	assert.Equal(t, validKey, key)

	// Test that subsequent calls to InitializeKeyManager do nothing
	// Set a different key
	differentKey, err := GenerateKey(AES256KeySize)
	require.NoError(t, err)
	os.Setenv(testEnvVar, base64.StdEncoding.EncodeToString(differentKey))

	// Try to initialize again
	err = InitializeKeyManager("")
	require.NoError(t, err)

	// Key should still be the original one
	key, err = km.GetPrimaryKey()
	require.NoError(t, err)
	assert.Equal(t, validKey, key)
}

func TestGenerateKey(t *testing.T) {
	// Test generating key with default size
	key, err := GenerateKey(AES256KeySize)
	require.NoError(t, err)
	assert.Len(t, key, AES256KeySize)

	// Test generating key with custom size
	customSize := 64
	key, err = GenerateKey(customSize)
	require.NoError(t, err)
	assert.Len(t, key, customSize)

	// Test generating key with size smaller than minimum (should use minimum)
	key, err = GenerateKey(16)
	require.NoError(t, err)
	assert.Len(t, key, MinKeyLength)
}

func TestGenerateKeyString(t *testing.T) {
	// Test generating key string
	keyStr, err := GenerateKeyString(AES256KeySize)
	require.NoError(t, err)
	assert.NotEmpty(t, keyStr)

	// Test that the key string decodes to a valid key
	decodedKey, err := base64.StdEncoding.DecodeString(keyStr)
	require.NoError(t, err)
	assert.Len(t, decodedKey, AES256KeySize)
}

func TestDecodeKey(t *testing.T) {
	// Test decoding a hex key
	originalKey := []byte("this is a test key that is long enough")
	hexKey := encodeToHex(originalKey)
	decodedKey, err := decodeKey(hexKey)
	require.NoError(t, err)
	assert.Equal(t, originalKey, decodedKey)

	// Test decoding a base64 key
	base64Key := base64.StdEncoding.EncodeToString(originalKey)
	decodedKey, err = decodeKey(base64Key)
	require.NoError(t, err)
	assert.Equal(t, originalKey, decodedKey)

	// Test decoding an invalid key
	_, err = decodeKey("not a valid key")
	require.Error(t, err)
}

// Helper function to encode bytes to hex
func encodeToHex(data []byte) string {
	hexChars := []byte("0123456789abcdef")
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0F]
	}
	return string(result)
}
