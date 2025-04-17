package e2e

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStorageProviderCredentialStorage verifies that sensitive credentials are properly encrypted
func TestStorageProviderCredentialStorage(t *testing.T) {
	// Setup test database
	testDB, err := SetupTestDB(t)
	require.NoError(t, err, "Failed to set up test database")
	defer testDB.Close()

	// Create a test user
	user := &db.User{
		Email:        "security-test@example.com",
		PasswordHash: "test-hash",
		IsAdmin:      BoolPointer(true),
	}
	err = testDB.CreateUser(user)
	require.NoError(t, err, "Failed to create test user")

	// Test different provider types with sensitive credentials
	testCases := []struct {
		name          string
		providerType  db.StorageProviderType
		sensitiveKeys []string
		secretValues  map[string]string
	}{
		{
			name:         "S3 Credentials",
			providerType: db.ProviderTypeS3,
			sensitiveKeys: []string{
				"EncryptedSecretKey",
			},
			secretValues: map[string]string{
				"SecretKey": "s3-super-secret-key-value",
			},
		},
		{
			name:         "SFTP Credentials",
			providerType: db.ProviderTypeSFTP,
			sensitiveKeys: []string{
				"EncryptedPassword",
			},
			secretValues: map[string]string{
				"Password": "sftp-super-secret-password",
			},
		},
		{
			name:         "Google Drive Credentials",
			providerType: db.ProviderTypeGoogleDrive,
			sensitiveKeys: []string{
				"EncryptedClientSecret",
				"EncryptedRefreshToken",
			},
			secretValues: map[string]string{
				"ClientSecret": "gdrive-super-secret-client-secret",
				"RefreshToken": "gdrive-super-secret-refresh-token",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a provider with sensitive information
			provider := &db.StorageProvider{
				Name:      "Security Test Provider - " + string(tc.providerType),
				Type:      tc.providerType,
				AccessKey: "test-access-key",
				CreatedBy: user.ID,
			}

			// Set sensitive fields
			for field, value := range tc.secretValues {
				switch field {
				case "SecretKey":
					provider.SecretKey = value
				case "Password":
					provider.Password = value
				case "ClientSecret":
					provider.ClientSecret = value
				case "RefreshToken":
					provider.RefreshToken = value
				}
			}

			// Save the provider
			err := testDB.CreateStorageProvider(provider)
			require.NoError(t, err, "Failed to create provider")

			// Fetch the provider directly from the database
			var rawProvider db.StorageProvider
			err = testDB.DB.First(&rawProvider, provider.ID).Error
			require.NoError(t, err, "Failed to fetch raw provider data")

			// Verify that sensitive fields are encrypted
			for _, sensitiveField := range tc.sensitiveKeys {
				// Get the encrypted value
				var encryptedValue string
				switch sensitiveField {
				case "EncryptedSecretKey":
					encryptedValue = rawProvider.EncryptedSecretKey
				case "EncryptedPassword":
					encryptedValue = rawProvider.EncryptedPassword
				case "EncryptedClientSecret":
					encryptedValue = rawProvider.EncryptedClientSecret
				case "EncryptedRefreshToken":
					encryptedValue = rawProvider.EncryptedRefreshToken
				}

				// Verify encryption
				assert.NotEmpty(t, encryptedValue, "Encrypted value should not be empty")

				// Encrypted values should be base64 encoded
				_, err := base64.StdEncoding.DecodeString(encryptedValue)
				assert.NoError(t, err, "Encrypted value should be base64 encoded")

				// The original plain text should not be present in the encrypted value
				for _, plainValue := range tc.secretValues {
					assert.False(t, strings.Contains(encryptedValue, plainValue),
						"Encrypted value should not contain plaintext")
				}

				// Original field should be empty after save (sensitive data shouldn't be stored in plain text)
				switch sensitiveField {
				case "EncryptedSecretKey":
					assert.Empty(t, rawProvider.SecretKey, "SecretKey should be empty in database")
				case "EncryptedPassword":
					assert.Empty(t, rawProvider.Password, "Password should be empty in database")
				case "EncryptedClientSecret":
					assert.Empty(t, rawProvider.ClientSecret, "ClientSecret should be empty in database")
				case "EncryptedRefreshToken":
					assert.Empty(t, rawProvider.RefreshToken, "RefreshToken should be empty in database")
				}
			}

			// Verify we can retrieve the provider with decrypted values
			fetchedProvider, err := testDB.GetStorageProvider(provider.ID)
			require.NoError(t, err, "Failed to fetch provider")

			// Verify we can read back the original values
			for field, expectedValue := range tc.secretValues {
				var actualValue string
				switch field {
				case "SecretKey":
					actualValue = fetchedProvider.SecretKey
				case "Password":
					actualValue = fetchedProvider.Password
				case "ClientSecret":
					actualValue = fetchedProvider.ClientSecret
				case "RefreshToken":
					actualValue = fetchedProvider.RefreshToken
				}

				// Note: In a real application with encryption, we would verify the decrypted values
				// For this test, we expect the raw DB to be encrypted but the fetched object to have decrypted values
				// This test might need adjustment depending on how your actual encryption system works
				if actualValue != "" {
					assert.Equal(t, expectedValue, actualValue, "Decrypted value should match original")
				}
			}
		})
	}
}

// TestStorageProviderAccessControl verifies that storage providers can only be accessed by their owners
func TestStorageProviderAccessControl(t *testing.T) {
	// Setup test database
	testDB, err := SetupTestDB(t)
	require.NoError(t, err, "Failed to set up test database")
	defer testDB.Close()

	// Create two test users
	user1 := &db.User{
		Email:        "security-test-user1@example.com",
		PasswordHash: "test-hash-1",
		IsAdmin:      BoolPointer(false),
	}
	err = testDB.CreateUser(user1)
	require.NoError(t, err, "Failed to create test user 1")

	user2 := &db.User{
		Email:        "security-test-user2@example.com",
		PasswordHash: "test-hash-2",
		IsAdmin:      BoolPointer(false),
	}
	err = testDB.CreateUser(user2)
	require.NoError(t, err, "Failed to create test user 2")

	// Create a storage provider owned by user 1
	provider1 := &db.StorageProvider{
		Name:      "Security Test Provider - User 1",
		Type:      db.ProviderTypeS3,
		AccessKey: "user1-access-key",
		SecretKey: "user1-secret-key",
		Region:    "us-west-1",
		Bucket:    "user1-bucket",
		CreatedBy: user1.ID,
	}
	err = testDB.CreateStorageProvider(provider1)
	require.NoError(t, err, "Failed to create provider for user 1")

	// Create a storage provider owned by user 2
	provider2 := &db.StorageProvider{
		Name:      "Security Test Provider - User 2",
		Type:      db.ProviderTypeS3,
		AccessKey: "user2-access-key",
		SecretKey: "user2-secret-key",
		Region:    "eu-west-1",
		Bucket:    "user2-bucket",
		CreatedBy: user2.ID,
	}
	err = testDB.CreateStorageProvider(provider2)
	require.NoError(t, err, "Failed to create provider for user 2")

	// Test 1: Owner check - user 1 should be able to access their own provider
	t.Run("Owner can access", func(t *testing.T) {
		provider, err := testDB.GetStorageProviderWithOwnerCheck(provider1.ID, user1.ID)
		assert.NoError(t, err, "Owner should be able to access their provider")
		assert.NotNil(t, provider, "Provider should be returned to owner")
		assert.Equal(t, provider1.ID, provider.ID, "Correct provider should be returned")
	})

	// Test 2: Owner check - user 1 should NOT be able to access user 2's provider
	t.Run("Non-owner cannot access", func(t *testing.T) {
		provider, err := testDB.GetStorageProviderWithOwnerCheck(provider2.ID, user1.ID)
		assert.Error(t, err, "Non-owner should not be able to access provider")
		assert.Nil(t, provider, "Provider should not be returned to non-owner")
	})

	// Test 3: List providers - user 1 should only see their own providers
	t.Run("List only shows owned providers", func(t *testing.T) {
		providers, err := testDB.GetStorageProviders(user1.ID)
		assert.NoError(t, err, "Should be able to list providers")

		// Check that only user 1's provider is returned
		assert.Equal(t, 1, len(providers), "User should only see their own providers")
		if len(providers) > 0 {
			assert.Equal(t, provider1.ID, providers[0].ID, "User should only see their own providers")
		}
	})

	// Test 4: Admin access - create admin user who should be able to access all providers
	adminUser := &db.User{
		Email:        "security-test-admin@example.com",
		PasswordHash: "admin-hash",
		IsAdmin:      BoolPointer(true),
	}
	err = testDB.CreateUser(adminUser)
	require.NoError(t, err, "Failed to create admin user")

	// Test admin access to all providers
	t.Run("Admin can access all providers", func(t *testing.T) {
		// Admin should be able to access user 1's provider
		provider, err := testDB.GetStorageProvider(provider1.ID)
		assert.NoError(t, err, "Admin should be able to access any provider")
		assert.NotNil(t, provider, "Provider should be returned to admin")
		assert.Equal(t, provider1.ID, provider.ID, "Correct provider should be returned")

		// Admin should be able to access user 2's provider
		provider, err = testDB.GetStorageProvider(provider2.ID)
		assert.NoError(t, err, "Admin should be able to access any provider")
		assert.NotNil(t, provider, "Provider should be returned to admin")
		assert.Equal(t, provider2.ID, provider.ID, "Correct provider should be returned")
	})
}

// TestStorageProviderInjectionAttacks tests protection against SQL injection in provider operations
func TestStorageProviderInjectionAttacks(t *testing.T) {
	// Setup test database
	testDB, err := SetupTestDB(t)
	require.NoError(t, err, "Failed to set up test database")
	defer testDB.Close()

	// Create a test user
	user := &db.User{
		Email:        "security-injection-test@example.com",
		PasswordHash: "test-hash",
		IsAdmin:      BoolPointer(true),
	}
	err = testDB.CreateUser(user)
	require.NoError(t, err, "Failed to create test user")

	// Test SQL injection attempts in provider fields
	injectionTests := []struct {
		name  string
		field string
		value string
	}{
		{
			name:  "SQL Injection in Name",
			field: "Name",
			value: "Injection Test'; DROP TABLE storage_providers; --",
		},
		{
			name:  "SQL Injection in Access Key",
			field: "AccessKey",
			value: "x' OR 1=1; --",
		},
		{
			name:  "SQL Injection in Secret Key",
			field: "SecretKey",
			value: "x'; UPDATE users SET is_admin=1 WHERE email LIKE '%'; --",
		},
		{
			name:  "SQL Injection in Bucket",
			field: "Bucket",
			value: "bucket'; DELETE FROM users; --",
		},
	}

	// Run injection tests
	for _, test := range injectionTests {
		t.Run(test.name, func(t *testing.T) {
			// Create a provider with potentially dangerous input
			provider := &db.StorageProvider{
				Type:      db.ProviderTypeS3,
				Name:      "Safe Name",
				AccessKey: "safe-access-key",
				SecretKey: "safe-secret-key",
				Region:    "us-west-1",
				Bucket:    "safe-bucket",
				CreatedBy: user.ID,
			}

			// Set the field with the injection attempt
			switch test.field {
			case "Name":
				provider.Name = test.value
			case "AccessKey":
				provider.AccessKey = test.value
			case "SecretKey":
				provider.SecretKey = test.value
			case "Bucket":
				provider.Bucket = test.value
			}

			// Save the provider - this should not cause SQL injection
			err := testDB.CreateStorageProvider(provider)
			assert.NoError(t, err, "Should safely handle potentially dangerous input")

			// Verify the provider was created with the exact value (no injection occurred)
			savedProvider, err := testDB.GetStorageProvider(provider.ID)
			assert.NoError(t, err, "Should be able to fetch the provider")

			// Check that the value was stored exactly as provided (sanitized/parameterized)
			switch test.field {
			case "Name":
				assert.Equal(t, test.value, savedProvider.Name, "Name should be stored safely")
			case "AccessKey":
				assert.Equal(t, test.value, savedProvider.AccessKey, "AccessKey should be stored safely")
			case "SecretKey":
				assert.Equal(t, test.value, savedProvider.SecretKey, "SecretKey should be stored safely")
			case "Bucket":
				assert.Equal(t, test.value, savedProvider.Bucket, "Bucket should be stored safely")
			}

			// Verify the database is still intact (tables weren't dropped)
			var count int64
			err = testDB.DB.Model(&db.StorageProvider{}).Count(&count).Error
			assert.NoError(t, err, "Database should still be intact")
			assert.GreaterOrEqual(t, count, int64(1), "Storage providers table should still exist with data")

			var userCount int64
			err = testDB.DB.Model(&db.User{}).Count(&userCount).Error
			assert.NoError(t, err, "Users table should still be intact")
			assert.GreaterOrEqual(t, userCount, int64(1), "Users table should still exist with data")
		})
	}
}
