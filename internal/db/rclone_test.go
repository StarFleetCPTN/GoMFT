package db

import (
	"fmt"
	"os"
	"os/exec"
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

func TestGoogleDriveRcloneConfig(t *testing.T) {
	// Skip if rclone not available
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone" // default to PATH lookup
	}
	_, err := exec.Command(rclonePath, "--version").CombinedOutput()
	if err != nil {
		t.Skip("Skipping test as rclone is not available")
	}

	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("google-rclone-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err = db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create Google Drive source config
	googleSourceConfig := &TransferConfig{
		Name:               "Google Drive Source Rclone Test",
		SourceType:         "google_drive",
		SourcePath:         "/path/in/google/drive",
		SourceClientID:     "source_google_client_id",
		SourceClientSecret: "source_google_client_secret",
		SourceTeamDrive:    "source_team_drive_id",
		DestinationType:    "local",
		DestinationPath:    "/local/destination/path",
		FilePattern:        "*.pdf",
		CreatedBy:          testUser.ID,
	}

	// Set authenticated status
	authenticated := true
	googleSourceConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleSourceConfig)
	assert.NoError(t, err)

	err = db.GenerateRcloneConfigWithToken(googleSourceConfig, "test_token")
	assert.NoError(t, err)

	// Generate rclone config for source
	configPath := db.GetConfigRclonePath(googleSourceConfig)

	// Check that the file exists
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "Rclone config file should exist")

	// Read the config file
	configContent, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	content := string(configContent)

	// Verify it contains Google Drive specific content
	assert.Contains(t, content, "type = drive")
	assert.Contains(t, content, fmt.Sprintf("client_id = %s", googleSourceConfig.SourceClientID))
	assert.Contains(t, content, "source")
	assert.Contains(t, content, fmt.Sprintf("team_drive = %s", googleSourceConfig.SourceTeamDrive))

	// Create Google Drive destination config
	googleDestConfig := &TransferConfig{
		Name:             "Google Drive Dest Rclone Test",
		SourceType:       "local",
		SourcePath:       "/local/source/path",
		DestinationType:  "google_drive",
		DestinationPath:  "/dest/path/in/google/drive",
		DestClientID:     "dest_google_client_id",
		DestClientSecret: "dest_google_client_secret",
		DestTeamDrive:    "dest_team_drive_id",
		FilePattern:      "*.pdf",
		CreatedBy:        testUser.ID,
	}

	// Set authenticated status
	googleDestConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleDestConfig)
	assert.NoError(t, err)

	// Generate rclone config for destination
	configPath = db.GetConfigRclonePath(googleDestConfig)

	// Check that the file exists
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "Rclone config file should exist")

	// Read the config file
	configContent, err = os.ReadFile(configPath)
	assert.NoError(t, err)
	content = string(configContent)

	// Verify it contains Google Drive specific content
	assert.Contains(t, content, "type = drive")
	assert.Contains(t, content, fmt.Sprintf("client_id = %s", googleDestConfig.DestClientID))
	assert.Contains(t, content, "dest")
	assert.Contains(t, content, fmt.Sprintf("team_drive = %s", googleDestConfig.DestTeamDrive))

	// Clean up
	err = db.Delete(&googleSourceConfig).Error
	assert.NoError(t, err)
	err = db.Delete(&googleDestConfig).Error
	assert.NoError(t, err)
}
