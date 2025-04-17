package encryption

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCredentialEncryptor(t *testing.T) *CredentialEncryptor {
	encService := setupEncryptionService(t)
	credEncryptor, err := NewCredentialEncryptor(encService)
	require.NoError(t, err)
	return credEncryptor
}

func TestNewCredentialEncryptor(t *testing.T) {
	t.Run("Valid encryption service", func(t *testing.T) {
		encService := setupEncryptionService(t)
		credEncryptor, err := NewCredentialEncryptor(encService)
		require.NoError(t, err)
		assert.NotNil(t, credEncryptor)
	})

	t.Run("Nil encryption service", func(t *testing.T) {
		credEncryptor, err := NewCredentialEncryptor(nil)
		require.Error(t, err)
		assert.Nil(t, credEncryptor)
	})
}

func TestCredentialEncryptor_Encrypt(t *testing.T) {
	credEncryptor := setupCredentialEncryptor(t)

	t.Run("Encrypt password", func(t *testing.T) {
		password := "securePassword123"
		encrypted, err := credEncryptor.EncryptPassword(password)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(encrypted, EncryptedPrefix))

		// Check that we can decrypt it
		decrypted, err := credEncryptor.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, password, decrypted)
	})

	t.Run("Encrypt API key", func(t *testing.T) {
		apiKey := "api_12345678901234567890abcdef"
		encrypted, err := credEncryptor.EncryptAPIKey(apiKey)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(encrypted, EncryptedPrefix))

		// Check that we can decrypt it
		decrypted, err := credEncryptor.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, apiKey, decrypted)
	})

	t.Run("Encrypt empty value", func(t *testing.T) {
		encrypted, err := credEncryptor.Encrypt("", TypePassword)
		require.Error(t, err)
		assert.Equal(t, ErrEmptyCredential, err)
		assert.Empty(t, encrypted)
	})

	t.Run("Encrypt value with invalid type", func(t *testing.T) {
		encrypted, err := credEncryptor.Encrypt("somevalue", "invalid_type")
		require.Error(t, err)
		assert.Contains(t, err.Error(), ErrUnsupportedType.Error())
		assert.Empty(t, encrypted)
	})

	t.Run("Password validation", func(t *testing.T) {
		shortPassword := "short"
		encrypted, err := credEncryptor.EncryptPassword(shortPassword)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "password too short")
		assert.Empty(t, encrypted)
	})

	t.Run("API key validation", func(t *testing.T) {
		shortAPIKey := "short"
		encrypted, err := credEncryptor.EncryptAPIKey(shortAPIKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too short")
		assert.Empty(t, encrypted)
	})

	t.Run("Already encrypted value", func(t *testing.T) {
		password := "securePassword123"
		encrypted, err := credEncryptor.EncryptPassword(password)
		require.NoError(t, err)

		// Try to encrypt again
		doubleEncrypted, err := credEncryptor.Encrypt(encrypted, TypePassword)
		require.Error(t, err)
		assert.Equal(t, ErrAlreadyEncrypted, err)
		assert.Empty(t, doubleEncrypted)
	})
}

func TestCredentialEncryptor_Decrypt(t *testing.T) {
	credEncryptor := setupCredentialEncryptor(t)

	t.Run("Decrypt encrypted value", func(t *testing.T) {
		original := "securePassword123"
		encrypted, err := credEncryptor.EncryptPassword(original)
		require.NoError(t, err)

		decrypted, err := credEncryptor.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, original, decrypted)
	})

	t.Run("Decrypt empty value", func(t *testing.T) {
		decrypted, err := credEncryptor.Decrypt("")
		require.Error(t, err)
		assert.Equal(t, ErrEmptyCredential, err)
		assert.Empty(t, decrypted)
	})

	t.Run("Decrypt non-encrypted value", func(t *testing.T) {
		decrypted, err := credEncryptor.Decrypt("notEncrypted")
		require.Error(t, err)
		assert.Equal(t, ErrNotEncrypted, err)
		assert.Empty(t, decrypted)
	})

	t.Run("Decrypt corrupted value", func(t *testing.T) {
		original := "securePassword123"
		encrypted, err := credEncryptor.EncryptPassword(original)
		require.NoError(t, err)

		// Remove the prefix for manipulation
		encryptedWithoutPrefix := strings.TrimPrefix(encrypted, EncryptedPrefix)

		// Base64 decode the encrypted content
		decoded, err := base64.StdEncoding.DecodeString(encryptedWithoutPrefix)
		require.NoError(t, err)

		// Find position in the actual ciphertext (after the IV)
		if len(decoded) > 20 {
			// Corrupt a byte in the ciphertext portion (not in the IV)
			decoded[20] ^= 0xFF // Flip all bits in this byte

			// Re-encode to base64
			corrupted := EncryptedPrefix + base64.StdEncoding.EncodeToString(decoded)

			// This should fail to decrypt
			decrypted, err := credEncryptor.Decrypt(corrupted)
			require.Error(t, err, "Decryption should fail with corrupted data")
			assert.Empty(t, decrypted)
		} else {
			t.Skip("Encrypted data too short to corrupt properly")
		}
	})
}

func TestCredentialEncryptor_EncryptField(t *testing.T) {
	credEncryptor := setupCredentialEncryptor(t)

	t.Run("Encrypt non-encrypted field", func(t *testing.T) {
		field := "securePassword123"
		encrypted, err := credEncryptor.EncryptField(field, TypePassword)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(encrypted, EncryptedPrefix))
	})

	t.Run("Already encrypted field", func(t *testing.T) {
		original := "securePassword123"
		encrypted, err := credEncryptor.EncryptPassword(original)
		require.NoError(t, err)

		// Try to encrypt again using EncryptField
		result, err := credEncryptor.EncryptField(encrypted, TypePassword)
		require.NoError(t, err)
		assert.Equal(t, encrypted, result, "EncryptField should return the already encrypted value")
	})

	t.Run("Empty field", func(t *testing.T) {
		result, err := credEncryptor.EncryptField("", TypePassword)
		require.NoError(t, err)
		assert.Empty(t, result, "EncryptField should return empty for empty input")
	})
}

func TestCredentialEncryptor_DecryptField(t *testing.T) {
	credEncryptor := setupCredentialEncryptor(t)

	t.Run("Decrypt encrypted field", func(t *testing.T) {
		original := "securePassword123"
		encrypted, err := credEncryptor.EncryptPassword(original)
		require.NoError(t, err)

		decrypted, err := credEncryptor.DecryptField(encrypted)
		require.NoError(t, err)
		assert.Equal(t, original, decrypted)
	})

	t.Run("Non-encrypted field", func(t *testing.T) {
		field := "plaintext"
		result, err := credEncryptor.DecryptField(field)
		require.NoError(t, err)
		assert.Equal(t, field, result, "DecryptField should return non-encrypted value as is")
	})

	t.Run("Empty field", func(t *testing.T) {
		result, err := credEncryptor.DecryptField("")
		require.NoError(t, err)
		assert.Empty(t, result, "DecryptField should return empty for empty input")
	})
}

func TestSanitizeCredential(t *testing.T) {
	t.Run("Sanitize plaintext", func(t *testing.T) {
		original := "plainTextPassword123"
		sanitized := SanitizeCredential(original)
		assert.NotEqual(t, original, sanitized)
		assert.True(t, len(sanitized) < len(original))
		assert.Contains(t, sanitized, "...")
	})

	t.Run("Sanitize encrypted value", func(t *testing.T) {
		credEncryptor := setupCredentialEncryptor(t)
		original := "securePassword123"
		encrypted, err := credEncryptor.EncryptPassword(original)
		require.NoError(t, err)

		sanitized := SanitizeCredential(encrypted)
		assert.NotEqual(t, encrypted, sanitized)
		assert.True(t, strings.HasPrefix(sanitized, EncryptedPrefix))
		assert.Contains(t, sanitized, "...")
	})

	t.Run("Sanitize empty value", func(t *testing.T) {
		sanitized := SanitizeCredential("")
		assert.Empty(t, sanitized)
	})

	t.Run("Sanitize short value", func(t *testing.T) {
		sanitized := SanitizeCredential("short")
		assert.Equal(t, "****", sanitized)
	})
}

func TestRequiresEncryption(t *testing.T) {
	testCases := []struct {
		fieldName          string
		requiresEncryption bool
		expectedType       CredentialType
	}{
		{"password", true, TypePassword},
		{"userPassword", true, TypePassword},
		{"passwd", true, TypePassword},
		{"pwd", true, TypePassword},
		{"apiKey", true, TypeAPIKey},
		{"api_key", true, TypeAPIKey},
		{"secretKey", true, TypeSecretKey},
		{"secret_key", true, TypeSecretKey},
		{"accessToken", true, TypeAccessToken},
		{"access_token", true, TypeAccessToken},
		{"refreshToken", true, TypeRefreshToken},
		{"refresh_token", true, TypeRefreshToken},
		{"oauthToken", true, TypeOAuthToken},
		{"oauth_refresh_token", true, TypeOAuthToken},
		{"sshKey", true, TypeSSHKey},
		{"ssh_key", true, TypeSSHKey},
		{"privateKey", true, TypeSSHKey},
		{"private_key", true, TypeSSHKey},
		{"authToken", true, TypeGeneric},
		{"secret", true, TypeGeneric},
		{"key", true, TypeGeneric},
		{"token", true, TypeGeneric},
		{"username", false, ""},
		{"email", false, ""},
		{"address", false, ""},
		{"name", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.fieldName, func(t *testing.T) {
			requires, credType := RequiresEncryption(tc.fieldName)
			assert.Equal(t, tc.requiresEncryption, requires)
			if tc.requiresEncryption {
				assert.Equal(t, tc.expectedType, credType)
			}
		})
	}
}

func TestGetGlobalCredentialEncryptor(t *testing.T) {
	// Setup environment for global encryption service
	testEnvVar := DefaultKeyEnvVar
	validKey := make([]byte, AES256KeySize)
	for i := range validKey {
		validKey[i] = byte(i % 256)
	}
	validKeyBase64 := encodeBase64(validKey)

	// Set a valid key in environment
	setenv(t, testEnvVar, validKeyBase64)

	// Get global credential encryptor
	credEncryptor, err := GetGlobalCredentialEncryptor()
	require.NoError(t, err)
	assert.NotNil(t, credEncryptor)

	// Test that it works
	testValue := "testPassword123"
	encrypted, err := credEncryptor.EncryptPassword(testValue)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(encrypted, EncryptedPrefix))

	decrypted, err := credEncryptor.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, testValue, decrypted)
}

// Utility functions for testing

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func setenv(t *testing.T, key, value string) {
	t.Setenv(key, value)
}
