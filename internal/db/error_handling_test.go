package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Tests for error handling in GetUserByEmail
func TestGetUserByEmailError(t *testing.T) {
	db := setupTestDB(t)

	// Test the error case with a non-existent email
	user, err := db.GetUserByEmail("nonexistent@example.com")

	// Verify expectations
	assert.Error(t, err, "Should return an error when user is not found")
	assert.Nil(t, user, "User should be nil when an error occurs")
}

// Tests for error handling in GetUserByID
func TestGetUserByIDError(t *testing.T) {
	db := setupTestDB(t)

	// Test the error case with a non-existent ID
	user, err := db.GetUserByID(9999)

	// Verify expectations
	assert.Error(t, err, "Should return an error when user is not found")
	assert.Nil(t, user, "User should be nil when an error occurs")
}

// Tests for error handling in GetPasswordResetToken
func TestGetPasswordResetTokenError(t *testing.T) {
	db := setupTestDB(t)

	// Test the error case with an invalid token
	token, err := db.GetPasswordResetToken("invalid-token")

	// Verify expectations
	assert.Error(t, err, "Should return an error when token is not found")
	assert.Nil(t, token, "Token should be nil when an error occurs")

	// Test with an expired token
	testUser := &User{
		Email:              "expired-token@example.com",
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err = db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create an expired token (expired 1 hour ago)
	expiredToken := &PasswordResetToken{
		UserID:    testUser.ID,
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	err = db.CreatePasswordResetToken(expiredToken)
	assert.NoError(t, err)

	// Try to get the expired token
	retrievedToken, err := db.GetPasswordResetToken("expired-token")
	assert.Error(t, err, "Should return an error for expired token")
	assert.Nil(t, retrievedToken, "Token should be nil for expired token")

	// Create a used token
	usedToken := &PasswordResetToken{
		UserID:    testUser.ID,
		Token:     "used-token",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Used:      BoolPtr(true),
	}
	err = db.CreatePasswordResetToken(usedToken)
	assert.NoError(t, err)

	// Try to get the used token
	retrievedToken, err = db.GetPasswordResetToken("used-token")
	assert.Error(t, err, "Should return an error for used token")
	assert.Nil(t, retrievedToken, "Token should be nil for used token")
}

// Tests for error handling in DeleteTransferConfig
func TestDeleteTransferConfigError(t *testing.T) {
	db := setupTestDB(t)

	// Test deleting a non-existent config
	err := db.DeleteTransferConfig(9999)

	// Verify expectations - should not return an error even if the record doesn't exist
	assert.NoError(t, err, "Should not return an error when deleting non-existent config")
}

// Tests for error handling in DeleteJob
func TestDeleteJobError(t *testing.T) {
	db := setupTestDB(t)

	// Test deleting a non-existent job
	err := db.DeleteJob(9999)

	// Verify expectations - should not return an error even if the record doesn't exist
	assert.NoError(t, err, "Should not return an error when deleting non-existent job")
}

// Tests for error handling in GetFileMetadataByHash
func TestGetFileMetadataByHashError(t *testing.T) {
	db := setupTestDB(t)

	// Test the error case with an invalid hash
	metadata, err := db.GetFileMetadataByHash("invalid-hash")

	// Verify expectations
	assert.Error(t, err, "Should return an error when metadata is not found")
	assert.Nil(t, metadata, "Metadata should be nil when an error occurs")
}

// Tests for error handling in Initialize
func TestInitializeErrors(t *testing.T) {
	// Test with a path that is a directory, not a file
	// This should cause an error when trying to open a SQLite database
	_, err := Initialize("/dev/null/cannot_be_a_db")
	assert.Error(t, err, "Should return an error with invalid path")
}

// Tests for error handling in GenerateRcloneConfig
func TestGenerateRcloneConfigErrors(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "config-error-test@example.com",
		PasswordHash:       "hashed_password",
		IsAdmin:            BoolPtr(false),
		LastPasswordChange: time.Now(),
	}

	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a config with invalid credentials for an SFTP connection
	invalidConfig := &TransferConfig{
		Name:            "Invalid Config",
		SourceType:      "sftp", // Using SFTP with invalid host to force error
		SourceHost:      "nonexistent.host",
		SourcePort:      22,
		SourceUser:      "invaliduser",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		CreatedBy:       testUser.ID,
	}

	err = db.CreateTransferConfig(invalidConfig)
	assert.NoError(t, err)

	// Set a non-existent RCLONE_PATH to force error
	t.Setenv("RCLONE_PATH", "/nonexistent/rclone")

	// This should return an error because the rclone command doesn't exist
	err = db.GenerateRcloneConfig(invalidConfig)
	assert.Error(t, err, "Should return an error when rclone command fails")
}
