package db

import (
	"fmt"
	"os"
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
		IsAdmin:            true,
		LastPasswordChange: time.Now(),
	}

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
	assert.False(t, retrievedToken.Used, "Token should not be marked as used initially")

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
	assert.True(t, updatedToken.Used, "Token should be marked as used")
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
		Enabled:   true,
		LastRun:   &now,
		NextRun:   &nextRun,
		CreatedBy: testUser.ID,
	}

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
	retrievedJob.Enabled = false
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
		Enabled:   true,
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
		Enabled:   true,
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
		IsAdmin:            false,
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
		IsAdmin:            false,
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
		DestPassiveMode: true,
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
		IsAdmin:            false,
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
		Enabled:   true,
		LastRun:   &lastRun,
		NextRun:   &nextRun,
		CreatedBy: testUser.ID,
	}

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
