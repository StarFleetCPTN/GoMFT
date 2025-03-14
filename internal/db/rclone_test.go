package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGetConfigRclonePathWithEnv tests the GetConfigRclonePath function with different environment variables
func TestGetConfigRclonePathWithEnv(t *testing.T) {
	// Save original environment variable
	originalDataDir := os.Getenv("DATA_DIR")
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Set a custom data directory
	customDir := "/tmp/custom_data_dir"
	os.Setenv("DATA_DIR", customDir)

	db := setupTestDB(t)

	// Create a test config
	testUser := &User{
		Email:              "rclone-env-test@example.com",
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	testConfig := &TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/dest/path",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(testConfig)
	assert.NoError(t, err)

	// Test GetConfigRclonePath with custom DATA_DIR
	configPath := db.GetConfigRclonePath(testConfig)
	assert.Equal(t,
		filepath.Join(customDir, "configs", fmt.Sprintf("config_%d.conf", testConfig.ID)),
		configPath,
		"Should use DATA_DIR environment variable")
}

// TestGenerateRcloneConfigWithoutRclone tests error handling when rclone executable is not available
func TestGenerateRcloneConfigWithoutRclone(t *testing.T) {
	// Save original environment variable
	originalRclonePath := os.Getenv("RCLONE_PATH")
	defer os.Setenv("RCLONE_PATH", originalRclonePath)

	// Set a nonexistent rclone path
	os.Setenv("RCLONE_PATH", "/nonexistent/rclone")

	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "rclone-missing-test@example.com",
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Test configs for different source types
	sourceTypes := []string{"sftp", "s3", "minio", "b2", "smb", "ftp", "webdav", "nextcloud", "onedrive", "google_drive"}

	for _, sourceType := range sourceTypes {
		testConfig := &TransferConfig{
			Name:               fmt.Sprintf("Test %s Config", sourceType),
			SourceType:         sourceType,
			SourceHost:         "example.com",
			SourcePort:         22,
			SourceUser:         "testuser",
			SourcePath:         "/source/path",
			SourceAccessKey:    "access_key",
			SourceSecretKey:    "secret_key",
			SourceRegion:       "us-east-1",
			SourceEndpoint:     "endpoint.example.com",
			SourceClientID:     "client_id",
			SourceClientSecret: "client_secret",
			DestinationType:    "local",
			DestinationPath:    "/dest/path",
			CreatedBy:          testUser.ID,
		}

		err = db.CreateTransferConfig(testConfig)
		assert.NoError(t, err)

		// This should return an error because rclone is not available
		err = db.GenerateRcloneConfig(testConfig)
		assert.Error(t, err, "Should return an error when rclone executable is not found for source type: %s", sourceType)
	}

	// Test configs for different destination types
	destTypes := []string{"sftp", "s3", "minio", "b2", "smb", "ftp", "webdav", "nextcloud", "onedrive", "google_drive"}

	for _, destType := range destTypes {
		testConfig := &TransferConfig{
			Name:             fmt.Sprintf("Test Dest %s Config", destType),
			SourceType:       "local",
			SourcePath:       "/source/path",
			DestinationType:  destType,
			DestHost:         "example.com",
			DestPort:         22,
			DestUser:         "testuser",
			DestinationPath:  "/dest/path",
			DestAccessKey:    "access_key",
			DestSecretKey:    "secret_key",
			DestRegion:       "us-east-1",
			DestEndpoint:     "endpoint.example.com",
			DestClientID:     "client_id",
			DestClientSecret: "client_secret",
			CreatedBy:        testUser.ID,
		}

		err = db.CreateTransferConfig(testConfig)
		assert.NoError(t, err)

		// This should return an error because rclone is not available
		err = db.GenerateRcloneConfig(testConfig)
		if destType != "local" {
			assert.Error(t, err, "Should return an error when rclone executable is not found for dest type: %s", destType)
		} else {
			// Local destination type might not error since it doesn't need to call rclone
			t.Logf("Local destination type might not error")
		}
	}
}
