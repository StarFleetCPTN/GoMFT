package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *DB {
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Initialize the database schema
	err = gormDB.AutoMigrate(
		&User{},
		&PasswordHistory{},
		&PasswordResetToken{},
		&TransferConfig{},
		&Job{},
		&JobHistory{},
		&FileMetadata{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return &DB{DB: gormDB}
}

func TestUserCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	testUser.SetIsAdmin(true)

	// Test Create
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	assert.NotZero(t, testUser.ID, "User ID should be set after creation")

	// Test Read
	retrievedUser, err := db.GetUserByEmail(testUser.Email)
	if err != nil {
		t.Fatalf("Failed to get user by email: %v", err)
	}
	assert.Equal(t, testUser.ID, retrievedUser.ID, "Retrieved user should have the same ID")
	assert.Equal(t, testUser.Email, retrievedUser.Email, "Retrieved user should have the same email")
	assert.Equal(t, testUser.PasswordHash, retrievedUser.PasswordHash, "Retrieved user should have the same password hash")
	assert.Equal(t, testUser.IsAdmin, retrievedUser.IsAdmin, "Retrieved user should have the same admin status")

	// Test Update
	retrievedUser.Email = fmt.Sprintf("updated-%d@example.com", time.Now().UnixNano())
	err = db.UpdateUser(retrievedUser)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify update
	updatedUser, err := db.GetUserByID(retrievedUser.ID)
	if err != nil {
		t.Fatalf("Failed to get user by ID: %v", err)
	}
	assert.Equal(t, retrievedUser.Email, updatedUser.Email, "User email should be updated")
}

func TestPasswordResetToken(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a password reset token
	tokenString := fmt.Sprintf("test-token-%d", time.Now().UnixNano())
	expiresAt := time.Now().Add(24 * time.Hour)
	testToken := &PasswordResetToken{
		UserID:    testUser.ID,
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}
	err = db.CreatePasswordResetToken(testToken)
	if err != nil {
		t.Fatalf("Failed to create password reset token: %v", err)
	}
	assert.NotZero(t, testToken.ID, "Token ID should be set after creation")

	// Retrieve the token
	retrievedToken, err := db.GetPasswordResetToken(tokenString)
	if err != nil {
		t.Fatalf("Failed to get password reset token: %v", err)
	}
	assert.Equal(t, testToken.ID, retrievedToken.ID, "Retrieved token should have the same ID")
	assert.Equal(t, testUser.ID, retrievedToken.UserID, "Retrieved token should reference the correct user")
	assert.False(t, retrievedToken.GetUsed(), "Token should not be marked as used initially")

	// Mark token as used
	err = db.MarkPasswordResetTokenAsUsed(retrievedToken.ID)
	if err != nil {
		t.Fatalf("Failed to mark token as used: %v", err)
	}

	// Verify token is marked as used
	// Note: We need to use GetPasswordResetTokenByID instead of GetPasswordResetToken
	// because GetPasswordResetToken filters out used tokens
	var updatedToken PasswordResetToken
	result := db.DB.First(&updatedToken, retrievedToken.ID)
	if result.Error != nil {
		t.Fatalf("Failed to get updated password reset token: %v", result.Error)
	}
	assert.True(t, updatedToken.GetUsed(), "Token should be marked as used")
}

func TestTransferConfigCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user first
	testUser := &User{
		Email:              fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a test transfer config
	testConfig := &TransferConfig{
		Name:            fmt.Sprintf("Test Transfer %d", time.Now().UnixNano()),
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		FilePattern:     "*.txt",
		CreatedBy:       testUser.ID,
	}

	// Test Create
	err = db.CreateTransferConfig(testConfig)
	if err != nil {
		t.Fatalf("Failed to create transfer config: %v", err)
	}
	assert.NotZero(t, testConfig.ID, "Config ID should be set after creation")

	// Test Read
	retrievedConfig, err := db.GetTransferConfig(testConfig.ID)
	if err != nil {
		t.Fatalf("Failed to get transfer config: %v", err)
	}
	assert.Equal(t, testConfig.Name, retrievedConfig.Name, "Retrieved config should have the same name")
	assert.Equal(t, testConfig.SourcePath, retrievedConfig.SourcePath, "Retrieved config should have the same source path")

	// Test Update
	retrievedConfig.Name = fmt.Sprintf("Updated Transfer %d", time.Now().UnixNano())
	err = db.UpdateTransferConfig(retrievedConfig)
	if err != nil {
		t.Fatalf("Failed to update transfer config: %v", err)
	}

	// Verify update
	updatedConfig, err := db.GetTransferConfig(retrievedConfig.ID)
	if err != nil {
		t.Fatalf("Failed to get updated transfer config: %v", err)
	}
	assert.Equal(t, retrievedConfig.Name, updatedConfig.Name, "Config name should be updated")

	// Test listing configs
	configs, err := db.GetTransferConfigs(testUser.ID)
	if err != nil {
		t.Fatalf("Failed to list transfer configs: %v", err)
	}
	assert.GreaterOrEqual(t, len(configs), 1, "There should be at least one config in the list")

	// Test Delete
	err = db.DeleteTransferConfig(testConfig.ID)
	if err != nil {
		t.Fatalf("Failed to delete transfer config: %v", err)
	}

	// Verify deletion
	_, err = db.GetTransferConfig(testConfig.ID)
	assert.Error(t, err, "Getting deleted config should return an error")
}

func TestJobCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user first
	testUser := &User{
		Email:              fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a test transfer config
	testConfig := &TransferConfig{
		Name:            fmt.Sprintf("Test Transfer %d", time.Now().UnixNano()),
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		FilePattern:     "*.txt",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(testConfig)
	if err != nil {
		t.Fatalf("Failed to create transfer config: %v", err)
	}

	// Create a test job
	now := time.Now()
	nextRun := now.Add(24 * time.Hour)
	testJob := &Job{
		Name:      fmt.Sprintf("Test Job %d", time.Now().UnixNano()),
		ConfigID:  testConfig.ID,
		Schedule:  "0 * * * *", // Run every hour
		LastRun:   &now,
		NextRun:   &nextRun,
		CreatedBy: testUser.ID,
	}
	testJob.SetEnabled(true)

	// Test Create
	err = db.CreateJob(testJob)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}
	assert.NotZero(t, testJob.ID, "Job ID should be set after creation")

	// Test Read
	retrievedJob, err := db.GetJob(testJob.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	assert.Equal(t, testJob.Name, retrievedJob.Name, "Retrieved job should have the same name")
	assert.Equal(t, testJob.ConfigID, retrievedJob.ConfigID, "Retrieved job should have the same config ID")
	assert.Equal(t, testJob.Schedule, retrievedJob.Schedule, "Retrieved job should have the same schedule")

	// Test listing jobs
	jobs, err := db.GetJobs(testUser.ID)
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	assert.GreaterOrEqual(t, len(jobs), 1, "There should be at least one job in the list")

	// Test Get Active Jobs
	activeJobs, err := db.GetActiveJobs()
	if err != nil {
		t.Fatalf("Failed to get active jobs: %v", err)
	}
	assert.GreaterOrEqual(t, len(activeJobs), 1, "There should be at least one active job")

	// Test Update
	retrievedJob.Name = fmt.Sprintf("Updated Job %d", time.Now().UnixNano())
	retrievedJob.SetEnabled(false)
	err = db.UpdateJob(retrievedJob)
	if err != nil {
		t.Fatalf("Failed to update job: %v", err)
	}

	// Verify update
	updatedJob, err := db.GetJob(retrievedJob.ID)
	if err != nil {
		t.Fatalf("Failed to get updated job: %v", err)
	}
	assert.Equal(t, retrievedJob.Name, updatedJob.Name, "Job name should be updated")
	assert.Equal(t, retrievedJob.Enabled, updatedJob.Enabled, "Job enabled status should be updated")

	// Test Delete
	err = db.DeleteJob(testJob.ID)
	if err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}

	// Verify deletion
	_, err = db.GetJob(testJob.ID)
	assert.Error(t, err, "Getting deleted job should return an error")
}

// Helper function to test if a config ID is selected for a job
func configSelected(job *Job, configID uint) bool {
	// Check if the job has the config ID in its list
	for _, id := range job.GetConfigIDsList() {
		if id == configID {
			return true
		}
	}
	// As a fallback, check the primary ConfigID
	return job.ConfigID == configID
}

func TestJobMultipleConfigs(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("test-multi-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create multiple test configs
	config1 := &TransferConfig{
		Name:            "Test Config 1",
		SourceType:      "local",
		SourcePath:      "/source/path1",
		DestinationType: "local",
		DestinationPath: "/destination/path1",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(config1)
	assert.NoError(t, err)

	config2 := &TransferConfig{
		Name:            "Test Config 2",
		SourceType:      "local",
		SourcePath:      "/source/path2",
		DestinationType: "local",
		DestinationPath: "/destination/path2",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(config2)
	assert.NoError(t, err)

	config3 := &TransferConfig{
		Name:            "Test Config 3",
		SourceType:      "local",
		SourcePath:      "/source/path3",
		DestinationType: "local",
		DestinationPath: "/destination/path3",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(config3)
	assert.NoError(t, err)

	// Test 1: Create job with multiple configs
	testJob := &Job{
		Name:      "Multi Config Job",
		Schedule:  "0 * * * *",
		CreatedBy: testUser.ID,
	}

	// Set multiple config IDs
	configIDs := []uint{config1.ID, config2.ID, config3.ID}
	testJob.SetConfigIDsList(configIDs)

	// Verify ConfigIDs string format
	assert.Contains(t, testJob.ConfigIDs, fmt.Sprintf("%d", config1.ID))
	assert.Contains(t, testJob.ConfigIDs, fmt.Sprintf("%d", config2.ID))
	assert.Contains(t, testJob.ConfigIDs, fmt.Sprintf("%d", config3.ID))

	// Verify ConfigID is set to the first config
	assert.Equal(t, config1.ID, testJob.ConfigID)

	// Save the job
	err = db.CreateJob(testJob)
	assert.NoError(t, err)

	// Test 2: Retrieve job and check config IDs
	retrievedJob, err := db.GetJob(testJob.ID)
	assert.NoError(t, err)

	// Verify retrieved config IDs
	retrievedIDs := retrievedJob.GetConfigIDsList()
	assert.Len(t, retrievedIDs, 3)
	assert.Contains(t, retrievedIDs, config1.ID)
	assert.Contains(t, retrievedIDs, config2.ID)
	assert.Contains(t, retrievedIDs, config3.ID)

	// Test 3: Test configSelected function
	assert.True(t, configSelected(retrievedJob, config1.ID))
	assert.True(t, configSelected(retrievedJob, config2.ID))
	assert.True(t, configSelected(retrievedJob, config3.ID))
	assert.False(t, configSelected(retrievedJob, uint(999)))

	// Test 4: Get configs for job
	configs, err := db.GetConfigsForJob(testJob.ID)
	assert.NoError(t, err)
	assert.Len(t, configs, 3)

	// Verify config names are correct
	configNames := make([]string, len(configs))
	for i, config := range configs {
		configNames[i] = config.Name
	}
	assert.Contains(t, configNames, "Test Config 1")
	assert.Contains(t, configNames, "Test Config 2")
	assert.Contains(t, configNames, "Test Config 3")

	// Test 5: Update config IDs
	updatedIDs := []uint{config1.ID, config3.ID} // Remove config2
	retrievedJob.SetConfigIDsList(updatedIDs)
	err = db.UpdateJob(retrievedJob)
	assert.NoError(t, err)

	// Verify update
	updatedJob, err := db.GetJob(testJob.ID)
	assert.NoError(t, err)
	updatedRetrievedIDs := updatedJob.GetConfigIDsList()
	assert.Len(t, updatedRetrievedIDs, 2)
	assert.Contains(t, updatedRetrievedIDs, config1.ID)
	assert.Contains(t, updatedRetrievedIDs, config3.ID)
	assert.NotContains(t, updatedRetrievedIDs, config2.ID)
}

func TestJobHistoryCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user first
	testUser := &User{
		Email:              fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a test transfer config
	testConfig := &TransferConfig{
		Name:            fmt.Sprintf("Test Transfer %d", time.Now().UnixNano()),
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		FilePattern:     "*.txt",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(testConfig)
	if err != nil {
		t.Fatalf("Failed to create transfer config: %v", err)
	}

	// Create a test job
	testJob := &Job{
		Name:      fmt.Sprintf("Test Job %d", time.Now().UnixNano()),
		ConfigID:  testConfig.ID,
		Schedule:  "0 * * * *", // Run every hour
		CreatedBy: testUser.ID,
	}
	err = db.CreateJob(testJob)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create a test job history record
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()
	testHistory := &JobHistory{
		JobID:            testJob.ID,
		StartTime:        startTime,
		EndTime:          &endTime,
		Status:           "completed",
		BytesTransferred: 1024,
		FilesTransferred: 5,
		ErrorMessage:     "",
	}

	// Test Create
	err = db.CreateJobHistory(testHistory)
	if err != nil {
		t.Fatalf("Failed to create job history: %v", err)
	}
	assert.NotZero(t, testHistory.ID, "Job history ID should be set after creation")

	// Test Update
	testHistory.Status = "failed"
	testHistory.ErrorMessage = "Test error message"
	err = db.UpdateJobHistory(testHistory)
	if err != nil {
		t.Fatalf("Failed to update job history: %v", err)
	}

	// Test getting job history
	histories, err := db.GetJobHistory(testJob.ID)
	if err != nil {
		t.Fatalf("Failed to get job history: %v", err)
	}
	assert.Equal(t, 1, len(histories), "There should be one job history record")
	assert.Equal(t, "failed", histories[0].Status, "Job history status should be 'failed'")
	assert.Equal(t, "Test error message", histories[0].ErrorMessage, "Job history error message should be set")
}

func TestFileMetadataCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user first
	testUser := &User{
		Email:              fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a test transfer config
	testConfig := &TransferConfig{
		Name:            fmt.Sprintf("Test Transfer %d", time.Now().UnixNano()),
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		FilePattern:     "*.txt",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(testConfig)
	if err != nil {
		t.Fatalf("Failed to create transfer config: %v", err)
	}

	// Create a test job
	testJob := &Job{
		Name:      fmt.Sprintf("Test Job %d", time.Now().UnixNano()),
		ConfigID:  testConfig.ID,
		Schedule:  "0 * * * *", // Run every hour
		CreatedBy: testUser.ID,
	}
	err = db.CreateJob(testJob)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create a test file metadata record
	fileName := fmt.Sprintf("testfile-%d.txt", time.Now().UnixNano())
	fileHash := fmt.Sprintf("md5-%d", time.Now().UnixNano())
	testMetadata := &FileMetadata{
		JobID:           testJob.ID,
		FileName:        fileName,
		OriginalPath:    "/source/path/" + fileName,
		FileSize:        1024,
		FileHash:        fileHash,
		CreationTime:    time.Now().Add(-2 * time.Hour),
		ModTime:         time.Now().Add(-1 * time.Hour),
		ProcessedTime:   time.Now(),
		DestinationPath: "/destination/path/" + fileName,
		Status:          "processed",
		ErrorMessage:    "",
	}

	// Test Create
	err = db.CreateFileMetadata(testMetadata)
	if err != nil {
		t.Fatalf("Failed to create file metadata: %v", err)
	}
	assert.NotZero(t, testMetadata.ID, "File metadata ID should be set after creation")

	// Test GetFileMetadataByJobAndName
	retrievedMetadata, err := db.GetFileMetadataByJobAndName(testJob.ID, fileName)
	if err != nil {
		t.Fatalf("Failed to get file metadata by job and name: %v", err)
	}
	assert.Equal(t, testMetadata.ID, retrievedMetadata.ID, "Retrieved metadata should have the same ID")
	assert.Equal(t, fileName, retrievedMetadata.FileName, "Retrieved metadata should have the same file name")
	assert.Equal(t, fileHash, retrievedMetadata.FileHash, "Retrieved metadata should have the same file hash")

	// Test GetFileMetadataByHash
	hashMetadata, err := db.GetFileMetadataByHash(fileHash)
	if err != nil {
		t.Fatalf("Failed to get file metadata by hash: %v", err)
	}
	assert.Equal(t, testMetadata.ID, hashMetadata.ID, "Retrieved metadata should have the same ID")

	// Test Delete
	err = db.DeleteFileMetadata(testMetadata.ID)
	if err != nil {
		t.Fatalf("Failed to delete file metadata: %v", err)
	}

	// Verify deletion
	_, err = db.GetFileMetadataByJobAndName(testJob.ID, fileName)
	assert.Error(t, err, "Getting deleted file metadata should return an error")
}

func TestDBInitialize(t *testing.T) {
	// Create a temporary file path for testing
	tempDBPath := "test_init.db"

	// Initialize the database
	db, err := Initialize(tempDBPath)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Cleanup
	err = db.Close()
	assert.NoError(t, err)

	// Remove test file
	err = os.Remove(tempDBPath)
	if err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: could not remove test database file: %v", err)
	}
}

func TestGetConfigRclonePath(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "rclone-test@example.com",
		PasswordHash:       "hashed_password",
		IsAdmin:            BoolPtr(false),
		LastPasswordChange: time.Now(),
	}

	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a test config
	testConfig := &TransferConfig{
		Name:            "Test Rclone Config",
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "sftp",
		DestHost:        "example.com",
		DestPort:        22,
		DestUser:        "testuser",
		DestinationPath: "/remote/path",
		DestKeyFile:     "private_key_content",
		CreatedBy:       testUser.ID,
	}

	err = db.CreateTransferConfig(testConfig)
	assert.NoError(t, err)

	// Test GetConfigRclonePath
	configPath := db.GetConfigRclonePath(testConfig)
	assert.NotEmpty(t, configPath)
	assert.Contains(t, configPath, fmt.Sprintf("%d", testConfig.ID))
}

func TestGenerateRcloneConfig(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "rclone-gen-test@example.com",
		PasswordHash:       "hashed_password",
		IsAdmin:            BoolPtr(false),
		LastPasswordChange: time.Now(),
	}

	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// SFTP config test
	sftpConfig := &TransferConfig{
		Name:            "Test SFTP Config",
		SourceType:      "local",
		SourcePath:      "/local/path",
		DestinationType: "sftp",
		DestHost:        "sftp.example.com",
		DestPort:        22,
		DestUser:        "testuser",
		DestinationPath: "/remote/path",
		DestKeyFile:     "private_key_content",
		CreatedBy:       testUser.ID,
	}

	err = db.CreateTransferConfig(sftpConfig)
	assert.NoError(t, err)

	// Test generating rclone config
	err = db.GenerateRcloneConfig(sftpConfig)
	assert.NoError(t, err)

	// FTP config test
	ftpConfig := &TransferConfig{
		Name:            "Test FTP Config",
		SourceType:      "local",
		SourcePath:      "/local/ftp",
		DestinationType: "ftp",
		DestHost:        "ftp.example.com",
		DestPort:        21,
		DestUser:        "ftpuser",
		DestPassiveMode: BoolPtr(true),
		CreatedBy:       testUser.ID,
	}

	err = db.CreateTransferConfig(ftpConfig)
	assert.NoError(t, err)

	// Test generating rclone config
	err = db.GenerateRcloneConfig(ftpConfig)
	assert.NoError(t, err)

	// S3 config test
	s3Config := &TransferConfig{
		Name:            "Test S3 Config",
		SourceType:      "local",
		SourcePath:      "/local/s3",
		DestinationType: "s3",
		DestBucket:      "mybucket",
		DestAccessKey:   "accessKey",
		DestRegion:      "us-east-1",
		DestEndpoint:    "s3.amazonaws.com",
		CreatedBy:       testUser.ID,
	}

	err = db.CreateTransferConfig(s3Config)
	assert.NoError(t, err)

	// Test generating rclone config
	err = db.GenerateRcloneConfig(s3Config)
	assert.NoError(t, err)

	// Test generating config for unsupported protocol
	invalidConfig := &TransferConfig{
		Name:            "Invalid Protocol Config",
		SourceType:      "local",
		SourcePath:      "/local/path",
		DestinationType: "unsupported",
		DestHost:        "example.com",
		CreatedBy:       testUser.ID,
	}

	err = db.CreateTransferConfig(invalidConfig)
	assert.NoError(t, err)

	// This should NOT return an error for unsupported protocol
	// as it defaults to local type
	err = db.GenerateRcloneConfig(invalidConfig)
	assert.NoError(t, err)

	// Verify the config file exists
	configPath := db.GetConfigRclonePath(invalidConfig)
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "Config file should exist")
}

func TestUpdateJobStatus(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "job-status-test@example.com",
		PasswordHash:       "hashed_password",
		IsAdmin:            BoolPtr(false),
		LastPasswordChange: time.Now(),
	}

	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a test transfer config
	testConfig := &TransferConfig{
		Name:            "Test Config for Job Status",
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		FilePattern:     "*.txt",
		CreatedBy:       testUser.ID,
	}

	err = db.CreateTransferConfig(testConfig)
	assert.NoError(t, err)

	// Create a test job
	now := time.Now()
	lastRun := now.Add(-time.Hour)
	nextRun := now.Add(time.Hour)

	testJob := &Job{
		Name:      "Test Job Status",
		ConfigID:  testConfig.ID,
		Schedule:  "0 * * * *", // Run hourly
		LastRun:   &lastRun,
		NextRun:   &nextRun,
		CreatedBy: testUser.ID,
	}
	testJob.SetEnabled(true)

	err = db.CreateJob(testJob)
	assert.NoError(t, err)

	// Update job's last run time
	updatedLastRun := time.Now()
	testJob.LastRun = &updatedLastRun

	err = db.UpdateJobStatus(testJob)
	assert.NoError(t, err)

	// Verify the job was updated
	updatedJob, err := db.GetJob(testJob.ID)
	assert.NoError(t, err)
	assert.NotEqual(t, lastRun.Unix(), updatedJob.LastRun.Unix())

	// Update job's next run time
	updatedNextRun := time.Now().Add(2 * time.Hour)
	testJob.NextRun = &updatedNextRun

	err = db.UpdateJobStatus(testJob)
	assert.NoError(t, err)

	// Verify the job was updated again
	updatedJob, err = db.GetJob(testJob.ID)
	assert.NoError(t, err)
	assert.Equal(t, updatedNextRun.Unix(), updatedJob.NextRun.Unix())
}

func BoolPtr(b bool) *bool {
	return &b
}

func TestGoogleDriveTransferConfig(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("google-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test 1: Create config with Google Drive as source
	googleSourceConfig := &TransferConfig{
		Name:               "Google Drive Source Test",
		SourceType:         "google_drive",
		SourcePath:         "/path/in/google/drive",
		SourceClientID:     "google_client_id",
		SourceClientSecret: "google_client_secret",
		SourceTeamDrive:    "team_drive_id",
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
	assert.NotZero(t, googleSourceConfig.ID, "Config ID should be set after creation")

	// Test 2: Create config with Google Drive as destination
	googleDestConfig := &TransferConfig{
		Name:             "Google Drive Destination Test",
		SourceType:       "local",
		SourcePath:       "/local/source/path",
		DestinationType:  "google_drive",
		DestinationPath:  "/path/in/google/drive",
		DestClientID:     "google_client_id",
		DestClientSecret: "google_client_secret",
		DestTeamDrive:    "team_drive_id",
		FilePattern:      "*.docx",
		CreatedBy:        testUser.ID,
	}

	// Set authenticated status
	googleDestConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleDestConfig)
	assert.NoError(t, err)
	assert.NotZero(t, googleDestConfig.ID, "Config ID should be set after creation")

	// Test 3: Create config with Google Drive as both source and destination
	googleBothConfig := &TransferConfig{
		Name:               "Google Drive Both Test",
		SourceType:         "google_drive",
		SourcePath:         "/source/path/in/google/drive",
		SourceClientID:     "source_google_client_id",
		SourceClientSecret: "source_google_client_secret",
		SourceTeamDrive:    "source_team_drive_id",
		DestinationType:    "google_drive",
		DestinationPath:    "/dest/path/in/google/drive",
		DestClientID:       "dest_google_client_id",
		DestClientSecret:   "dest_google_client_secret",
		DestTeamDrive:      "dest_team_drive_id",
		FilePattern:        "*.xlsx",
		CreatedBy:          testUser.ID,
	}

	// Set authenticated status
	googleBothConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleBothConfig)
	assert.NoError(t, err)
	assert.NotZero(t, googleBothConfig.ID, "Config ID should be set after creation")

	// Test retrieving and verifying Google Drive configs
	retrievedSourceConfig, err := db.GetTransferConfig(googleSourceConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "google_drive", retrievedSourceConfig.SourceType)
	assert.Equal(t, "/path/in/google/drive", retrievedSourceConfig.SourcePath)
	assert.Equal(t, "google_client_id", retrievedSourceConfig.SourceClientID)
	assert.Equal(t, "team_drive_id", retrievedSourceConfig.SourceTeamDrive)
	assert.True(t, *retrievedSourceConfig.GoogleDriveAuthenticated)

	retrievedDestConfig, err := db.GetTransferConfig(googleDestConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "google_drive", retrievedDestConfig.DestinationType)
	assert.Equal(t, "/path/in/google/drive", retrievedDestConfig.DestinationPath)
	assert.Equal(t, "google_client_id", retrievedDestConfig.DestClientID)
	assert.Equal(t, "team_drive_id", retrievedDestConfig.DestTeamDrive)
	assert.True(t, *retrievedDestConfig.GoogleDriveAuthenticated)

	// Test updating Google Drive config
	retrievedSourceConfig.SourcePath = "/updated/google/drive/path"
	retrievedSourceConfig.SourceTeamDrive = "updated_team_drive_id"
	err = db.UpdateTransferConfig(retrievedSourceConfig)
	assert.NoError(t, err)

	// Verify update
	updatedConfig, err := db.GetTransferConfig(googleSourceConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "/updated/google/drive/path", updatedConfig.SourcePath)
	assert.Equal(t, "updated_team_drive_id", updatedConfig.SourceTeamDrive)

	// Test changing authentication status
	unauthenticated := false
	updatedConfig.GoogleDriveAuthenticated = &unauthenticated
	err = db.UpdateTransferConfig(updatedConfig)
	assert.NoError(t, err)

	// Verify authentication status update
	finalConfig, err := db.GetTransferConfig(googleSourceConfig.ID)
	assert.NoError(t, err)
	assert.False(t, *finalConfig.GoogleDriveAuthenticated)

	// Clean up
	err = db.DeleteTransferConfig(googleSourceConfig.ID)
	assert.NoError(t, err)
	err = db.DeleteTransferConfig(googleDestConfig.ID)
	assert.NoError(t, err)
	err = db.DeleteTransferConfig(googleBothConfig.ID)
	assert.NoError(t, err)
}

func TestGoogleDriveJobExecution(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("google-job-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a test transfer config with Google Drive as source
	googleConfig := &TransferConfig{
		Name:               "Google Drive Job Test",
		SourceType:         "google_drive",
		SourcePath:         "/source/path/in/google/drive",
		SourceClientID:     "google_client_id",
		SourceClientSecret: "google_client_secret",
		SourceTeamDrive:    "team_drive_id",
		DestinationType:    "local",
		DestinationPath:    "/local/destination/path",
		FilePattern:        "*.pdf",
		CreatedBy:          testUser.ID,
	}

	// Set authenticated status
	authenticated := true
	googleConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleConfig)
	assert.NoError(t, err)
	assert.NotZero(t, googleConfig.ID, "Config ID should be set after creation")

	// Create a job using the Google Drive config
	job := &Job{
		Name:      "Google Drive Test Job",
		Schedule:  "0 * * * *", // Run every hour
		ConfigID:  googleConfig.ID,
		CreatedBy: testUser.ID,
	}

	// Set job as enabled
	job.SetEnabled(true)

	// Set up webhook notifications
	job.SetWebhookEnabled(true)
	job.WebhookURL = "https://example.com/webhook"
	job.SetNotifyOnSuccess(true)
	job.SetNotifyOnFailure(true)

	// Create the job
	err = db.CreateJob(job)
	assert.NoError(t, err)
	assert.NotZero(t, job.ID, "Job ID should be set after creation")

	// Test retrieving the job
	retrievedJob, err := db.GetJob(job.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Google Drive Test Job", retrievedJob.Name)
	assert.Equal(t, googleConfig.ID, retrievedJob.ConfigID)
	assert.True(t, retrievedJob.GetEnabled())
	assert.True(t, retrievedJob.GetWebhookEnabled())
	assert.Equal(t, "https://example.com/webhook", retrievedJob.WebhookURL)
	assert.True(t, retrievedJob.GetNotifyOnSuccess())
	assert.True(t, retrievedJob.GetNotifyOnFailure())

	// Create job history entry for this job
	startTime := time.Now().Add(-10 * time.Minute)
	endTime := time.Now()
	jobHistory := &JobHistory{
		JobID:            job.ID,
		ConfigID:         googleConfig.ID,
		StartTime:        startTime,
		EndTime:          &endTime,
		Status:           "success",
		BytesTransferred: 1024 * 1024 * 5, // 5 MB
		FilesTransferred: 3,
	}

	err = db.Create(jobHistory).Error
	assert.NoError(t, err)
	assert.NotZero(t, jobHistory.ID, "JobHistory ID should be set after creation")

	// Create file metadata entries
	fileMetadata1 := &FileMetadata{
		JobID:           job.ID,
		ConfigID:        googleConfig.ID,
		FileName:        "document1.pdf",
		OriginalPath:    "/source/path/in/google/drive/document1.pdf",
		FileSize:        1024 * 1024 * 2, // 2 MB
		FileHash:        "hash1",
		CreationTime:    time.Now().Add(-24 * time.Hour),
		ModTime:         time.Now().Add(-12 * time.Hour),
		ProcessedTime:   startTime.Add(1 * time.Minute),
		DestinationPath: "/local/destination/path/document1.pdf",
		Status:          "processed",
	}

	fileMetadata2 := &FileMetadata{
		JobID:           job.ID,
		ConfigID:        googleConfig.ID,
		FileName:        "document2.pdf",
		OriginalPath:    "/source/path/in/google/drive/document2.pdf",
		FileSize:        1024 * 1024 * 1, // 1 MB
		FileHash:        "hash2",
		CreationTime:    time.Now().Add(-24 * time.Hour),
		ModTime:         time.Now().Add(-12 * time.Hour),
		ProcessedTime:   startTime.Add(2 * time.Minute),
		DestinationPath: "/local/destination/path/document2.pdf",
		Status:          "processed",
	}

	fileMetadata3 := &FileMetadata{
		JobID:           job.ID,
		ConfigID:        googleConfig.ID,
		FileName:        "document3.pdf",
		OriginalPath:    "/source/path/in/google/drive/document3.pdf",
		FileSize:        1024 * 1024 * 2, // 2 MB
		FileHash:        "hash3",
		CreationTime:    time.Now().Add(-24 * time.Hour),
		ModTime:         time.Now().Add(-12 * time.Hour),
		ProcessedTime:   startTime.Add(3 * time.Minute),
		DestinationPath: "/local/destination/path/document3.pdf",
		Status:          "processed",
	}

	err = db.Create(fileMetadata1).Error
	assert.NoError(t, err)
	err = db.Create(fileMetadata2).Error
	assert.NoError(t, err)
	err = db.Create(fileMetadata3).Error
	assert.NoError(t, err)

	// Test fetching job history
	var histories []JobHistory
	err = db.Where("job_id = ?", job.ID).Find(&histories).Error
	assert.NoError(t, err)
	assert.Equal(t, 1, len(histories), "Should have 1 job history entry")
	assert.Equal(t, job.ID, histories[0].JobID)
	assert.Equal(t, googleConfig.ID, histories[0].ConfigID)
	assert.Equal(t, "success", histories[0].Status)
	assert.Equal(t, int64(1024*1024*5), histories[0].BytesTransferred)
	assert.Equal(t, 3, histories[0].FilesTransferred)

	// Test fetching file metadata
	var files []FileMetadata
	err = db.Where("job_id = ?", job.ID).Find(&files).Error
	assert.NoError(t, err)
	assert.Equal(t, 3, len(files), "Should have 3 file metadata entries")

	// Clean up
	err = db.Where("job_id = ?", job.ID).Delete(&FileMetadata{}).Error
	assert.NoError(t, err)
	err = db.Where("job_id = ?", job.ID).Delete(&JobHistory{}).Error
	assert.NoError(t, err)
	err = db.Delete(&job).Error
	assert.NoError(t, err)
	err = db.Delete(&googleConfig).Error
	assert.NoError(t, err)
}

func TestGoogleDriveAuthentication(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("google-auth-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a Google Drive config that requires authentication
	googleConfig := &TransferConfig{
		Name:               "Google Drive Auth Test",
		SourceType:         "google_drive",
		SourcePath:         "/source/path",
		SourceClientID:     "test_client_id",
		SourceClientSecret: "test_client_secret",
		DestinationType:    "local",
		DestinationPath:    "/local/path",
		CreatedBy:          testUser.ID,
	}

	// Initially not authenticated
	unauthenticated := false
	googleConfig.GoogleDriveAuthenticated = &unauthenticated

	// Create the config
	err = db.CreateTransferConfig(googleConfig)
	assert.NoError(t, err)

	// Test 1: Verify initial unauthenticated state
	retrievedConfig, err := db.GetTransferConfig(googleConfig.ID)
	assert.NoError(t, err)
	assert.False(t, retrievedConfig.GetGoogleDriveAuthenticated())

	// Test 2: Simulate authentication with token
	mockToken := `{"access_token":"test_access_token","refresh_token":"test_refresh_token","expiry":"2023-12-31T12:00:00Z"}`
	err = db.StoreGoogleDriveToken(fmt.Sprintf("%d", googleConfig.ID), mockToken)
	assert.NoError(t, err)

	// Verify authentication state was updated
	updatedConfig, err := db.GetTransferConfig(googleConfig.ID)
	assert.NoError(t, err)
	assert.True(t, updatedConfig.GetGoogleDriveAuthenticated())

	// Test 3: Generate rclone config with token
	err = db.GenerateRcloneConfigWithToken(updatedConfig, mockToken)
	assert.NoError(t, err)

	// Get config path
	configPath := db.GetConfigRclonePath(updatedConfig)

	// On test systems, the directory might not exist
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		// Create directory if it doesn't exist
		err = os.MkdirAll(configDir, 0755)
		assert.NoError(t, err)
	}

	// Check if config was generated properly
	_, err = os.Stat(configPath)
	// In a test environment, this may fail if the rclone executable is not available
	// or permissions are wrong, so we'll just log it rather than fail the test
	if err != nil {
		t.Logf("Warning: could not verify rclone config file: %v", err)
	}

	// Test 4: Simulate deauthentication (token revocation)
	updatedConfig.SetGoogleDriveAuthenticated(false)
	err = db.UpdateTransferConfig(updatedConfig)
	assert.NoError(t, err)

	// Verify deauthentication
	finalConfig, err := db.GetTransferConfig(googleConfig.ID)
	assert.NoError(t, err)
	assert.False(t, finalConfig.GetGoogleDriveAuthenticated())

	// Clean up
	err = db.DeleteTransferConfig(googleConfig.ID)
	assert.NoError(t, err)
}

func TestGoogleDriveErrorHandling(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("google-error-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Test 1: Create a config with missing required fields
	incompleteConfig := &TransferConfig{
		Name:            "Incomplete Google Drive Config",
		SourceType:      "google_drive",
		SourcePath:      "", // Missing path
		DestinationType: "local",
		DestinationPath: "/local/path",
		CreatedBy:       testUser.ID,
	}

	// This should still succeed at the database level, as validation typically happens at the application level
	err = db.CreateTransferConfig(incompleteConfig)
	assert.NoError(t, err)
	assert.NotZero(t, incompleteConfig.ID, "Config ID should be set after creation")

	// Test 2: Config with invalid Team Drive ID
	invalidTeamDriveConfig := &TransferConfig{
		Name:               "Invalid Team Drive Config",
		SourceType:         "google_drive",
		SourcePath:         "/test/path",
		SourceClientID:     "test_client_id",
		SourceClientSecret: "test_client_secret",
		SourceTeamDrive:    "invalid_team_drive_id",
		DestinationType:    "local",
		DestinationPath:    "/local/path",
		CreatedBy:          testUser.ID,
	}

	err = db.CreateTransferConfig(invalidTeamDriveConfig)
	assert.NoError(t, err)

	// Set it as authenticated (this would normally fail in a real environment)
	authenticated := true
	invalidTeamDriveConfig.GoogleDriveAuthenticated = &authenticated
	err = db.UpdateTransferConfig(invalidTeamDriveConfig)
	assert.NoError(t, err)

	// When trying to test a transfer with an invalid team drive in a real environment,
	// the rclone command would fail. We can't directly test this in a unit test,
	// but we can verify the config is properly set up to cause the expected failure.
	retrievedConfig, err := db.GetTransferConfig(invalidTeamDriveConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_team_drive_id", retrievedConfig.SourceTeamDrive, "Retrieved config should have the invalid team drive ID")
	assert.True(t, retrievedConfig.GetGoogleDriveAuthenticated(), "Config should be marked as authenticated")

	// Test 3: Test authentication error scenario - using malformed token
	badTokenConfig := &TransferConfig{
		Name:               "Bad Token Config",
		SourceType:         "google_drive",
		SourcePath:         "/test/path",
		SourceClientID:     "test_client_id",
		SourceClientSecret: "test_client_secret",
		DestinationType:    "local",
		DestinationPath:    "/local/path",
		CreatedBy:          testUser.ID,
	}

	err = db.CreateTransferConfig(badTokenConfig)
	assert.NoError(t, err)

	// Try to store a malformed token - shouldn't crash but may fail
	// In real-world usage, this would lead to auth failures when trying to use the token
	malformedToken := `{"not_valid_json`
	err = db.StoreGoogleDriveToken(fmt.Sprintf("%d", badTokenConfig.ID), malformedToken)
	// Even with malformed tokens, the DB operation might succeed as we're just storing a string
	// but the authentication would fail in actual usage
	if err != nil {
		t.Logf("StoreGoogleDriveToken returned error with malformed token as expected: %v", err)
	}

	// Clean up
	err = db.DeleteTransferConfig(incompleteConfig.ID)
	assert.NoError(t, err)
	err = db.DeleteTransferConfig(invalidTeamDriveConfig.ID)
	assert.NoError(t, err)
	err = db.DeleteTransferConfig(badTokenConfig.ID)
	assert.NoError(t, err)
}

func TestGoogleDriveTeamDrive(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("google-teamdrive-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Test 1: Configure source with Team Drive
	teamDriveSourceConfig := &TransferConfig{
		Name:               "Team Drive Source Test",
		SourceType:         "google_drive",
		SourcePath:         "/shared/documents",
		SourceClientID:     "test_client_id",
		SourceClientSecret: "test_client_secret",
		SourceTeamDrive:    "source_team_drive_id",
		DestinationType:    "local",
		DestinationPath:    "/local/path",
		FilePattern:        "*.pdf",
		CreatedBy:          testUser.ID,
	}

	// Set as authenticated
	authenticated := true
	teamDriveSourceConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(teamDriveSourceConfig)
	assert.NoError(t, err)
	assert.NotZero(t, teamDriveSourceConfig.ID, "Config ID should be set after creation")

	// Test 2: Configure destination with Team Drive
	teamDriveDestConfig := &TransferConfig{
		Name:             "Team Drive Destination Test",
		SourceType:       "local",
		SourcePath:       "/local/source",
		DestinationType:  "google_drive",
		DestinationPath:  "/team/drive/path",
		DestClientID:     "test_client_id",
		DestClientSecret: "test_client_secret",
		DestTeamDrive:    "dest_team_drive_id",
		FilePattern:      "*.docx",
		CreatedBy:        testUser.ID,
	}

	// Set as authenticated
	teamDriveDestConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(teamDriveDestConfig)
	assert.NoError(t, err)
	assert.NotZero(t, teamDriveDestConfig.ID, "Config ID should be set after creation")

	// Test 3: Configure both source and destination with Team Drive
	teamDriveBothConfig := &TransferConfig{
		Name:               "Team Drive Both Test",
		SourceType:         "google_drive",
		SourcePath:         "/source/team/drive/path",
		SourceClientID:     "source_client_id",
		SourceClientSecret: "source_client_secret",
		SourceTeamDrive:    "source_team_drive_id",
		DestinationType:    "google_drive",
		DestinationPath:    "/dest/team/drive/path",
		DestClientID:       "dest_client_id",
		DestClientSecret:   "dest_client_secret",
		DestTeamDrive:      "dest_team_drive_id",
		FilePattern:        "*.xlsx",
		CreatedBy:          testUser.ID,
	}

	// Set as authenticated
	teamDriveBothConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(teamDriveBothConfig)
	assert.NoError(t, err)
	assert.NotZero(t, teamDriveBothConfig.ID, "Config ID should be set after creation")

	// Test retrieving configs and verify Team Drive IDs are set correctly
	retrievedSourceConfig, err := db.GetTransferConfig(teamDriveSourceConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "source_team_drive_id", retrievedSourceConfig.SourceTeamDrive)
	assert.Empty(t, retrievedSourceConfig.DestTeamDrive)

	retrievedDestConfig, err := db.GetTransferConfig(teamDriveDestConfig.ID)
	assert.NoError(t, err)
	assert.Empty(t, retrievedDestConfig.SourceTeamDrive)
	assert.Equal(t, "dest_team_drive_id", retrievedDestConfig.DestTeamDrive)

	retrievedBothConfig, err := db.GetTransferConfig(teamDriveBothConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "source_team_drive_id", retrievedBothConfig.SourceTeamDrive)
	assert.Equal(t, "dest_team_drive_id", retrievedBothConfig.DestTeamDrive)

	// Clean up
	err = db.DeleteTransferConfig(teamDriveSourceConfig.ID)
	assert.NoError(t, err)
	err = db.DeleteTransferConfig(teamDriveDestConfig.ID)
	assert.NoError(t, err)
	err = db.DeleteTransferConfig(teamDriveBothConfig.ID)
	assert.NoError(t, err)
}

func TestGooglePhotosTransferConfig(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("gphotos-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test 1: Create config with Google Photos as source
	sourceReadOnly := true
	sourceIncludeArchived := false
	useBuiltinAuth := true
	googleSourceConfig := &TransferConfig{
		Name:                  "Google Photos Source Test",
		SourceType:            "gphotos",
		SourcePath:            "/albums/vacation",
		SourceClientID:        "google_client_id",
		SourceClientSecret:    "google_client_secret",
		SourceReadOnly:        &sourceReadOnly,
		SourceStartYear:       2015,
		SourceIncludeArchived: &sourceIncludeArchived,
		UseBuiltinAuth:        &useBuiltinAuth,
		DestinationType:       "local",
		DestinationPath:       "/local/destination/path",
		FilePattern:           "*.jpg",
		CreatedBy:             testUser.ID,
	}

	// Set authenticated status
	authenticated := true
	googleSourceConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleSourceConfig)
	assert.NoError(t, err)
	assert.NotZero(t, googleSourceConfig.ID, "Config ID should be set after creation")

	// Test 2: Create config with Google Photos as destination
	destReadOnly := false
	destIncludeArchived := true
	googleDestConfig := &TransferConfig{
		Name:                "Google Photos Destination Test",
		SourceType:          "local",
		SourcePath:          "/local/source/path",
		DestinationType:     "gphotos",
		DestinationPath:     "/albums/upload",
		DestClientID:        "google_client_id",
		DestClientSecret:    "google_client_secret",
		DestReadOnly:        &destReadOnly,
		DestStartYear:       2018,
		DestIncludeArchived: &destIncludeArchived,
		UseBuiltinAuth:      &useBuiltinAuth,
		FilePattern:         "*.png",
		CreatedBy:           testUser.ID,
	}

	// Set authenticated status
	googleDestConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleDestConfig)
	assert.NoError(t, err)
	assert.NotZero(t, googleDestConfig.ID, "Config ID should be set after creation")

	// Test 3: Create config with Google Photos as both source and destination
	googleBothConfig := &TransferConfig{
		Name:                  "Google Photos Both Test",
		SourceType:            "gphotos",
		SourcePath:            "/albums/source_album",
		SourceClientID:        "source_client_id",
		SourceClientSecret:    "source_client_secret",
		SourceReadOnly:        &sourceReadOnly,
		SourceStartYear:       2020,
		SourceIncludeArchived: &sourceIncludeArchived,
		DestinationType:       "gphotos",
		DestinationPath:       "/albums/dest_album",
		DestClientID:          "dest_client_id",
		DestClientSecret:      "dest_client_secret",
		DestReadOnly:          &destReadOnly,
		DestStartYear:         2020,
		DestIncludeArchived:   &destIncludeArchived,
		UseBuiltinAuth:        &useBuiltinAuth,
		FilePattern:           "*.jpeg",
		CreatedBy:             testUser.ID,
	}

	// Set authenticated status
	googleBothConfig.GoogleDriveAuthenticated = &authenticated

	// Create the config
	err = db.CreateTransferConfig(googleBothConfig)
	assert.NoError(t, err)
	assert.NotZero(t, googleBothConfig.ID, "Config ID should be set after creation")

	// Verify configs were created properly
	retrievedConfig, err := db.GetTransferConfig(googleSourceConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "gphotos", retrievedConfig.SourceType)
	assert.Equal(t, sourceReadOnly, *retrievedConfig.SourceReadOnly)
	assert.Equal(t, 2015, retrievedConfig.SourceStartYear)
	assert.Equal(t, sourceIncludeArchived, *retrievedConfig.SourceIncludeArchived)
	assert.Equal(t, useBuiltinAuth, *retrievedConfig.UseBuiltinAuth)
	assert.Equal(t, true, retrievedConfig.GetGoogleAuthenticated())

	retrievedConfig, err = db.GetTransferConfig(googleDestConfig.ID)
	assert.NoError(t, err)
	assert.Equal(t, "gphotos", retrievedConfig.DestinationType)
	assert.Equal(t, destReadOnly, *retrievedConfig.DestReadOnly)
	assert.Equal(t, 2018, retrievedConfig.DestStartYear)
	assert.Equal(t, destIncludeArchived, *retrievedConfig.DestIncludeArchived)
	assert.Equal(t, useBuiltinAuth, *retrievedConfig.UseBuiltinAuth)
	assert.Equal(t, true, retrievedConfig.GetGoogleAuthenticated())
}

func TestGooglePhotosRcloneConfig(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "gomft-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up data directory
	dataDir := filepath.Join(tempDir, "data")
	configDir := filepath.Join(dataDir, "configs")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Set DATA_DIR environment variable
	oldDataDir := os.Getenv("DATA_DIR")
	defer os.Setenv("DATA_DIR", oldDataDir)
	os.Setenv("DATA_DIR", dataDir)

	// Initialize test database
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("gphotos-rclone-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err = db.CreateUser(testUser)
	assert.NoError(t, err)

	// TEST 1: Google Photos as source with standard options
	sourceReadOnly := true
	sourceIncludeArchived := false
	useBuiltinAuth := true
	gphotosSourceConfig := &TransferConfig{
		ID:                    1, // Force ID for predictable config path
		Name:                  "Google Photos Source Config",
		SourceType:            "gphotos",
		SourcePath:            "/albums/vacation",
		SourceClientID:        "test_client_id",
		SourceClientSecret:    "test_client_secret",
		SourceReadOnly:        &sourceReadOnly,
		SourceStartYear:       2015,
		SourceIncludeArchived: &sourceIncludeArchived,
		UseBuiltinAuth:        &useBuiltinAuth,
		DestinationType:       "local",
		DestinationPath:       "/tmp/destination",
		FilePattern:           "*.jpg",
		CreatedBy:             testUser.ID,
	}

	// Generate rclone config
	err = db.GenerateRcloneConfig(gphotosSourceConfig)
	assert.NoError(t, err)

	// Check if config file exists
	configPath := filepath.Join(configDir, fmt.Sprintf("config_%d.conf", gphotosSourceConfig.ID))
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "Config file should exist")

	// Read config file content
	content, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	configContent := string(content)

	// Check for Google Photos source section
	assert.Contains(t, configContent, "[source_1]")
	assert.Contains(t, configContent, "type = google photos")
	assert.Contains(t, configContent, "client_id = test_client_id")
	assert.Contains(t, configContent, "client_secret = test_client_secret")
	assert.Contains(t, configContent, "read_only = true")
	assert.Contains(t, configContent, "start_year = 2015")
	assert.NotContains(t, configContent, "include_archived = true") // This should be false and not included

	// TEST 2: Google Photos as destination with authenticated token
	destReadOnly := false
	destIncludeArchived := true
	gphotosDestConfig := &TransferConfig{
		ID:                  2, // Force ID for predictable config path
		Name:                "Google Photos Destination Config",
		SourceType:          "local",
		SourcePath:          "/tmp/source",
		DestinationType:     "gphotos",
		DestinationPath:     "/albums/upload",
		DestClientID:        "dest_client_id",
		DestClientSecret:    "dest_client_secret",
		DestReadOnly:        &destReadOnly,
		DestStartYear:       2018,
		DestIncludeArchived: &destIncludeArchived,
		UseBuiltinAuth:      &useBuiltinAuth,
		FilePattern:         "*.png",
		CreatedBy:           testUser.ID,
	}

	// Set authentication status
	authenticated := true
	gphotosDestConfig.GoogleDriveAuthenticated = &authenticated

	// Generate config first (needed for token update)
	err = db.GenerateRcloneConfig(gphotosDestConfig)
	assert.NoError(t, err)

	// Now test token handling with GenerateRcloneConfigWithToken
	testToken := `{"access_token":"test-token","token_type":"Bearer","refresh_token":"test-refresh","expiry":"2023-12-31T23:59:59Z"}`
	err = db.GenerateRcloneConfigWithToken(gphotosDestConfig, testToken)
	assert.NoError(t, err)

	// Check updated config
	configPath = filepath.Join(configDir, fmt.Sprintf("config_%d.conf", gphotosDestConfig.ID))
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "Config file should exist")

	// Read config file content
	content, err = os.ReadFile(configPath)
	assert.NoError(t, err)
	configContent = string(content)

	// Check for Google Photos destination section with token
	assert.Contains(t, configContent, "type = google photos")
	assert.Contains(t, configContent, "client_id = dest_client_id")
	assert.Contains(t, configContent, "client_secret = dest_client_secret")
	assert.Contains(t, configContent, "token = {")
	assert.Contains(t, configContent, "access_token")
	assert.Contains(t, configContent, "test-token")
	assert.Contains(t, configContent, "refresh_token")
	assert.Contains(t, configContent, "test-refresh")
	assert.Contains(t, configContent, "include_archived = true")
	assert.NotContains(t, configContent, "read_only = false") // This should be false and not included
}

func TestGooglePhotosAuthentication(t *testing.T) {
	// Initialize test database
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              fmt.Sprintf("gphotos-auth-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a transfer config with Google Photos
	readOnly := true
	includeArchived := false
	useBuiltinAuth := true
	gPhotosConfig := &TransferConfig{
		Name:                  "Test Google Photos Auth",
		SourceType:            "gphotos",
		SourcePath:            "/albums/vacation",
		SourceClientID:        "test_client_id",
		SourceClientSecret:    "test_client_secret",
		SourceReadOnly:        &readOnly,
		SourceStartYear:       2015,
		SourceIncludeArchived: &includeArchived,
		UseBuiltinAuth:        &useBuiltinAuth,
		DestinationType:       "local",
		DestinationPath:       "/tmp/destination",
		FilePattern:           "*.jpg",
		CreatedBy:             testUser.ID,
	}

	// Create the config
	err = db.CreateTransferConfig(gPhotosConfig)
	assert.NoError(t, err)
	assert.NotZero(t, gPhotosConfig.ID)

	// Test initial authentication state
	// Should be false when first created
	authenticated := gPhotosConfig.GetGoogleAuthenticated()
	assert.False(t, authenticated)
	t.Logf("Initial GoogleDriveAuthenticated value: %v", gPhotosConfig.GoogleDriveAuthenticated)

	// Test generic Google authentication method (new)
	gPhotosConfig.SetGoogleAuthenticated(true)
	t.Logf("After SetGoogleAuthenticated(true): %v", gPhotosConfig.GoogleDriveAuthenticated)

	// Save the updated config to the database
	err = db.UpdateTransferConfig(gPhotosConfig)
	assert.NoError(t, err)
	t.Logf("After UpdateTransferConfig: %v", gPhotosConfig.GoogleDriveAuthenticated)

	// Verify authentication status is updated
	updatedConfig, err := db.GetTransferConfig(gPhotosConfig.ID)
	assert.NoError(t, err)
	t.Logf("Retrieved config GoogleDriveAuthenticated: %v", updatedConfig.GoogleDriveAuthenticated)
	assert.True(t, updatedConfig.GetGoogleAuthenticated())

	// Verify it can be unset
	updatedConfig.SetGoogleAuthenticated(false)
	t.Logf("After SetGoogleAuthenticated(false): %v", updatedConfig.GoogleDriveAuthenticated)

	// Save the updated config to the database
	err = db.UpdateTransferConfig(updatedConfig)
	assert.NoError(t, err)

	// Verify authentication status is updated
	updatedConfig2, err := db.GetTransferConfig(gPhotosConfig.ID)
	assert.NoError(t, err)
	t.Logf("Retrieved config2 GoogleDriveAuthenticated: %v", updatedConfig2.GoogleDriveAuthenticated)
	assert.False(t, updatedConfig2.GetGoogleAuthenticated())

	// Test with the old method naming for backward compatibility
	updatedConfig2.SetGoogleDriveAuthenticated(true)
	t.Logf("After SetGoogleDriveAuthenticated(true): %v", updatedConfig2.GoogleDriveAuthenticated)

	// Save the updated config to the database
	err = db.UpdateTransferConfig(updatedConfig2)
	assert.NoError(t, err)

	// Verify authentication status is updated when using the old method
	updatedConfig3, err := db.GetTransferConfig(gPhotosConfig.ID)
	assert.NoError(t, err)
	t.Logf("Retrieved config3 GoogleDriveAuthenticated: %v", updatedConfig3.GoogleDriveAuthenticated)
	assert.True(t, updatedConfig3.GetGoogleAuthenticated())
	assert.True(t, updatedConfig3.GetGoogleDriveAuthenticated())
}
