package encryption

import (
	"bytes"
	"encoding/base64"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestKeyManager(t *testing.T) KeyManager {
	// Setup test environment
	testEnvVar := "TEST_ENCRYPTION_KEY"
	validKey := make([]byte, AES256KeySize)
	for i := range validKey {
		validKey[i] = byte(i % 256)
	}
	validKeyBase64 := base64.StdEncoding.EncodeToString(validKey)

	// Set a valid key in environment
	os.Setenv(testEnvVar, validKeyBase64)
	t.Cleanup(func() {
		os.Unsetenv(testEnvVar)
	})

	km := NewKeyManager(testEnvVar)
	err := km.(KeyManager).Initialize()
	require.NoError(t, err)

	return km
}

func setupEncryptionService(t *testing.T) *EncryptionService {
	km := setupTestKeyManager(t)
	service, err := NewEncryptionService(km)
	require.NoError(t, err)
	return service
}

func TestNewEncryptionService(t *testing.T) {
	t.Run("Valid key manager", func(t *testing.T) {
		km := setupTestKeyManager(t)
		service, err := NewEncryptionService(km)
		require.NoError(t, err)
		assert.NotNil(t, service)
	})

	t.Run("Nil key manager", func(t *testing.T) {
		service, err := NewEncryptionService(nil)
		require.Error(t, err)
		assert.Nil(t, service)
	})
}

func TestEncryptionService_Encrypt(t *testing.T) {
	service := setupEncryptionService(t)

	t.Run("Encrypt valid data", func(t *testing.T) {
		plaintext := []byte("This is a test message that needs to be encrypted")
		encrypted, err := service.Encrypt(plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)

		// Encrypted data should be base64 encoded
		_, err = base64.StdEncoding.DecodeString(encrypted)
		require.NoError(t, err)
	})

	t.Run("Encrypt empty data", func(t *testing.T) {
		encrypted, err := service.Encrypt([]byte{})
		require.Error(t, err)
		assert.Equal(t, ErrEmptyPlaintext, err)
		assert.Empty(t, encrypted)
	})

	t.Run("Same plaintext produces different ciphertexts", func(t *testing.T) {
		plaintext := []byte("This should encrypt to different ciphertexts each time")
		encrypted1, err := service.Encrypt(plaintext)
		require.NoError(t, err)

		encrypted2, err := service.Encrypt(plaintext)
		require.NoError(t, err)

		assert.NotEqual(t, encrypted1, encrypted2, "Same plaintext should encrypt to different ciphertexts due to random IV")
	})
}

func TestEncryptionService_Decrypt(t *testing.T) {
	service := setupEncryptionService(t)

	t.Run("Decrypt valid data", func(t *testing.T) {
		plaintext := []byte("This is a test message that needs to be encrypted and decrypted")
		encrypted, err := service.Encrypt(plaintext)
		require.NoError(t, err)

		decrypted, err := service.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("Decrypt empty data", func(t *testing.T) {
		decrypted, err := service.Decrypt("")
		require.Error(t, err)
		assert.Equal(t, ErrEmptyCiphertext, err)
		assert.Nil(t, decrypted)
	})

	t.Run("Decrypt invalid base64", func(t *testing.T) {
		decrypted, err := service.Decrypt("this-is-not-valid-base64!@#$%^")
		require.Error(t, err)
		assert.Nil(t, decrypted)
	})

	t.Run("Decrypt corrupted data - last byte modified", func(t *testing.T) {
		plaintext := []byte("This is a test message with proper length for padding")
		encrypted, err := service.Encrypt(plaintext)
		require.NoError(t, err)

		// Modify the last byte to corrupt the padding
		decoded, err := base64.StdEncoding.DecodeString(encrypted)
		require.NoError(t, err)
		decoded[len(decoded)-1] ^= 0x01 // Flip one bit in the last byte
		corrupted := base64.StdEncoding.EncodeToString(decoded)

		decrypted, err := service.Decrypt(corrupted)
		require.Error(t, err, "Decryption should fail with corrupted data")
		assert.Nil(t, decrypted)
	})

	t.Run("Decrypt with short data", func(t *testing.T) {
		// Create a short invalid encrypted string (not enough bytes for IV)
		shortData := base64.StdEncoding.EncodeToString([]byte("tooshort"))
		decrypted, err := service.Decrypt(shortData)
		require.Error(t, err)
		assert.Nil(t, decrypted)
	})
}

func TestEncryptionService_EncryptString(t *testing.T) {
	service := setupEncryptionService(t)

	t.Run("Encrypt valid string", func(t *testing.T) {
		plaintext := "This is a test string that needs to be encrypted"
		encrypted, err := service.EncryptString(plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)

		// Encrypted data should be base64 encoded
		_, err = base64.StdEncoding.DecodeString(encrypted)
		require.NoError(t, err)
	})

	t.Run("Encrypt empty string", func(t *testing.T) {
		encrypted, err := service.EncryptString("")
		require.Error(t, err)
		assert.Equal(t, ErrEmptyPlaintext, err)
		assert.Empty(t, encrypted)
	})
}

func TestEncryptionService_DecryptString(t *testing.T) {
	service := setupEncryptionService(t)

	t.Run("Decrypt valid string", func(t *testing.T) {
		plaintext := "This is a test string that needs to be encrypted and decrypted"
		encrypted, err := service.EncryptString(plaintext)
		require.NoError(t, err)

		decrypted, err := service.DecryptString(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("Decrypt empty string", func(t *testing.T) {
		decrypted, err := service.DecryptString("")
		require.Error(t, err)
		assert.Equal(t, ErrEmptyCiphertext, err)
		assert.Empty(t, decrypted)
	})
}

func TestPkcs7Padding(t *testing.T) {
	blockSize := 16

	t.Run("Pad and unpad", func(t *testing.T) {
		testCases := []struct {
			input    []byte
			expected int // expected padding size
		}{
			{[]byte("testing"), 9},                                    // 7 bytes + 9 padding = 16 bytes (multiple of blockSize)
			{[]byte("16 bytes exactly"), 16},                          // 16 bytes + 16 padding = 32 bytes (multiple of blockSize)
			{[]byte("this is a longer test string"), 4},               // 28 bytes + 4 padding = 32 bytes (multiple of blockSize)
			{[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}, 2}, // 14 bytes + 2 padding = 16 bytes (multiple of blockSize)
			{[]byte{}, 16}, // 0 bytes + 16 padding = 16 bytes (multiple of blockSize)
		}

		for _, tc := range testCases {
			padded := pkcs7Pad(tc.input, blockSize)
			// Check padding size
			assert.Equal(t, len(tc.input)+tc.expected, len(padded))
			// Check padding value
			for i := len(tc.input); i < len(padded); i++ {
				assert.Equal(t, byte(tc.expected), padded[i])
			}

			// Unpad and check
			unpadded, err := pkcs7Unpad(padded, blockSize)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(tc.input, unpadded))
		}
	})

	t.Run("Invalid padding", func(t *testing.T) {
		// Invalid padding value
		invalid := []byte("test data with invalid padding")
		paddedInvalid := pkcs7Pad(invalid, blockSize)
		paddedInvalid[len(paddedInvalid)-1] = 99 // Invalid padding value
		_, err := pkcs7Unpad(paddedInvalid, blockSize)
		require.Error(t, err)

		// Inconsistent padding
		inconsistent := []byte("test data with inconsistent padding")
		paddedInconsistent := pkcs7Pad(inconsistent, blockSize)
		paddedInconsistent[len(paddedInconsistent)-2] = 99 // Make padding inconsistent
		_, err = pkcs7Unpad(paddedInconsistent, blockSize)
		require.Error(t, err)

		// Empty data
		_, err = pkcs7Unpad([]byte{}, blockSize)
		require.Error(t, err)

		// Invalid block size
		invalidSize := []byte("invalid size")
		_, err = pkcs7Unpad(invalidSize, blockSize)
		require.Error(t, err)
	})
}

func TestGetGlobalEncryptionService(t *testing.T) {
	// Reset global key manager before test
	globalKeyManager = nil
	globalKeyManagerOnce = sync.Once{}

	// Setup test environment
	testEnvVar := DefaultKeyEnvVar
	validKey := make([]byte, AES256KeySize)
	for i := range validKey {
		validKey[i] = byte(i % 256)
	}
	validKeyBase64 := base64.StdEncoding.EncodeToString(validKey)

	// Set a valid key in environment
	os.Setenv(testEnvVar, validKeyBase64)
	t.Cleanup(func() {
		os.Unsetenv(testEnvVar)
	})

	// Get global encryption service
	service, err := GetGlobalEncryptionService()
	require.NoError(t, err)
	assert.NotNil(t, service)

	// Test with actual encryption/decryption
	plaintext := "Test with global encryption service"
	encrypted, err := service.EncryptString(plaintext)
	require.NoError(t, err)

	decrypted, err := service.DecryptString(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}
