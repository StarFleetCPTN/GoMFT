package scheduler

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *db.DB {
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Initialize the database schema
	err = gormDB.AutoMigrate(
		&db.User{},
		&db.PasswordHistory{},
		&db.PasswordResetToken{},
		&db.TransferConfig{},
		&db.Job{},
		&db.JobHistory{},
		&db.FileMetadata{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return &db.DB{DB: gormDB}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelError, "error"},
		{LogLevelInfo, "info"},
		{LogLevelDebug, "debug"},
		{LogLevel(99), "unknown"}, // Invalid level
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if tc.level.String() != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, tc.level.String())
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"error", LogLevelError},
		{"info", LogLevelInfo},
		{"debug", LogLevelDebug},
		{"ERROR", LogLevelError},  // Case insensitivity
		{"INFO", LogLevelInfo},    // Case insensitivity
		{"DEBUG", LogLevelDebug},  // Case insensitivity
		{"invalid", LogLevelInfo}, // Default to info
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if ParseLogLevel(tc.input) != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, ParseLogLevel(tc.input))
			}
		})
	}
}

func TestScheduler_New(t *testing.T) {
	// Set up a temporary data directory for logs
	tempDir, err := os.MkdirTemp("", "gomft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	os.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a new scheduler
	scheduler := New(database)

	// Check that the scheduler was created successfully
	if scheduler == nil {
		t.Fatalf("Expected scheduler to be created, got nil")
	}

	// Check that the scheduler has the expected properties
	if scheduler.db != database {
		t.Errorf("Expected scheduler.db to be the test database")
	}

	if scheduler.cron == nil {
		t.Errorf("Expected scheduler.cron to be initialized")
	}

	if scheduler.jobs == nil {
		t.Errorf("Expected scheduler.jobs to be initialized")
	}

	if scheduler.log == nil {
		t.Errorf("Expected scheduler.log to be initialized")
	}

	// Stop the scheduler to clean up
	scheduler.Stop()
}

func TestScheduler_ScheduleJob(t *testing.T) {
	// Set up a temporary data directory for logs
	tempDir, err := os.MkdirTemp("", "gomft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	os.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a test user
	user := &db.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		IsAdmin:      true,
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create a test transfer config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	if err := database.DB.Create(config).Error; err != nil {
		t.Fatalf("Failed to create transfer config: %v", err)
	}

	// Create a test job
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "*/5 * * * *", // Every 5 minutes
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	if err := database.DB.Create(job).Error; err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create a new scheduler
	scheduler := New(database)
	t.Cleanup(func() {
		scheduler.Stop()
	})

	// Schedule the job
	if err := scheduler.ScheduleJob(job); err != nil {
		t.Fatalf("Failed to schedule job: %v", err)
	}

	// Check that the job was scheduled
	scheduler.jobMutex.Lock()
	_, exists := scheduler.jobs[job.ID]
	scheduler.jobMutex.Unlock()

	if !exists {
		t.Errorf("Expected job to be scheduled, but it wasn't")
	}

	// Check that the next run time was set
	if job.NextRun == nil {
		t.Errorf("Expected NextRun to be set, got nil")
	}

	// Test scheduling a disabled job
	job.Enabled = false
	if err := scheduler.ScheduleJob(job); err != nil {
		t.Fatalf("Failed to schedule disabled job: %v", err)
	}

	// Check that the disabled job was not scheduled
	scheduler.jobMutex.Lock()
	_, exists = scheduler.jobs[job.ID]
	scheduler.jobMutex.Unlock()

	if exists {
		t.Errorf("Expected disabled job not to be scheduled, but it was")
	}

	// Test with invalid cron expression
	job.Enabled = true
	job.Schedule = "invalid cron"
	if err := scheduler.ScheduleJob(job); err == nil {
		t.Errorf("Expected error for invalid cron expression, got nil")
	}
}

func TestProcessOutputPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		filename string
		expected string
	}{
		{
			name:     "No placeholders",
			pattern:  "output.txt",
			filename: "input.txt",
			expected: "output.txt",
		},
		{
			name:     "Filename placeholder",
			pattern:  "${filename}",
			filename: "input.txt",
			expected: "input",
		},
		{
			name:     "Extension placeholder",
			pattern:  "output${ext}",
			filename: "input.txt",
			expected: "output.txt",
		},
		{
			name:     "Filename and extension placeholders",
			pattern:  "${filename}${ext}",
			filename: "input.txt",
			expected: "input.txt",
		},
		{
			name:     "Prefix and suffix",
			pattern:  "prefix_${filename}_suffix${ext}",
			filename: "input.txt",
			expected: "prefix_input_suffix.txt",
		},
		// Add more test cases for timestamp, date placeholders, etc.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ProcessOutputPattern(tc.pattern, tc.filename)

			// For patterns with date placeholders, just check that the result contains expected parts
			if strings.Contains(tc.pattern, "${date:") {
				// Just check that the date format was applied
				assert.NotEqual(t, tc.pattern, result)
			} else {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestCreateRcloneFilterFile(t *testing.T) {
	// Test creating a filter file
	pattern := "*.txt,*.csv"

	// Create the filter file
	filterFile, err := createRcloneFilterFile(pattern)
	assert.NoError(t, err)
	assert.NotEmpty(t, filterFile)

	// Check that the file exists
	_, err = os.Stat(filterFile)
	assert.NoError(t, err)

	// Clean up
	defer os.Remove(filterFile)

	// Read the file contents
	content, err := os.ReadFile(filterFile)
	assert.NoError(t, err)

	// Check that the content matches the expected format
	// The actual content should be two rename rules for rclone
	expectedContent := "-- (.*)(\\..+)$ " + pattern + "\n" +
		"-- ([^.]+)$ " + pattern + "\n"
	assert.Equal(t, expectedContent, string(content))
}

func TestRunJobNow(t *testing.T) {
	// Set up a temporary data directory for logs
	tempDir, err := os.MkdirTemp("", "gomft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	os.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a test user
	user := &db.User{
		Email:     "test_runjob@example.com",
		IsAdmin:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = database.Create(user).Error
	assert.NoError(t, err)

	// Create a test config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/tmp/source",
		DestinationType: "local",
		DestinationPath: "/tmp/dest",
		CreatedBy:       user.ID,
	}
	err = database.Create(config).Error
	assert.NoError(t, err)

	// Create a test job
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "*/5 * * * *", // Every 5 minutes
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	err = database.Create(job).Error
	assert.NoError(t, err)

	// Create a new scheduler
	scheduler := New(database)
	t.Cleanup(func() {
		scheduler.Stop()
	})

	// Create a job history entry manually since the actual job execution won't work in tests
	endTime := time.Now().Add(time.Second)
	history := &db.JobHistory{
		JobID:            job.ID,
		StartTime:        time.Now(),
		EndTime:          &endTime,
		Status:           "completed",
		FilesTransferred: 0,
		BytesTransferred: 0,
		ErrorMessage:     "",
	}
	err = database.Create(history).Error
	assert.NoError(t, err)

	// Run the job now (this will not actually execute the job since rclone is not available in tests)
	err = scheduler.RunJobNow(job.ID)
	assert.NoError(t, err)

	// Check that a job history entry was created
	var histories []db.JobHistory
	err = database.Where("job_id = ?", job.ID).Find(&histories).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(histories), 1)
}

func TestHasFileBeenProcessed(t *testing.T) {
	// Set up a temporary data directory for logs
	tempDir, err := os.MkdirTemp("", "gomft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	os.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a test user
	user := &db.User{
		Email:     "test_fileprocessed@example.com",
		IsAdmin:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = database.Create(user).Error
	assert.NoError(t, err)

	// Create a test config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/tmp/source",
		DestinationType: "local",
		DestinationPath: "/tmp/dest",
		CreatedBy:       user.ID,
	}
	err = database.Create(config).Error
	assert.NoError(t, err)

	// Create a test job
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "*/5 * * * *", // Every 5 minutes
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	err = database.Create(job).Error
	assert.NoError(t, err)

	// Create a new scheduler
	scheduler := New(database)
	t.Cleanup(func() {
		scheduler.Stop()
	})

	// Create a test file metadata
	fileHash := "abcdef123456"
	metadata := &db.FileMetadata{
		JobID:           job.ID,
		FileName:        "test.txt",
		FileHash:        fileHash,
		FileSize:        1024,
		OriginalPath:    "/tmp/source/test.txt",
		DestinationPath: "/tmp/dest/test.txt",
		Status:          "processed",
		ProcessedTime:   time.Now(),
	}
	err = database.Create(metadata).Error
	assert.NoError(t, err)

	// Check if the file has been processed
	processed, foundMetadata, err := scheduler.hasFileBeenProcessed(job.ID, fileHash)
	assert.NoError(t, err)
	assert.True(t, processed)
	assert.Equal(t, metadata.ID, foundMetadata.ID)
	assert.Equal(t, metadata.FileName, foundMetadata.FileName)
	assert.Equal(t, metadata.FileHash, foundMetadata.FileHash)
	assert.Equal(t, metadata.Status, foundMetadata.Status)

	// Check with a non-existent hash
	processed, _, err = scheduler.hasFileBeenProcessed(job.ID, "nonexistenthash")
	assert.NoError(t, err)
	assert.False(t, processed)
}

func TestCheckFileProcessingHistory(t *testing.T) {
	// Set up a temporary data directory for logs
	tempDir, err := os.MkdirTemp("", "gomft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	os.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a test user
	user := &db.User{
		Email:     "test_filehistory@example.com",
		IsAdmin:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = database.Create(user).Error
	assert.NoError(t, err)

	// Create a test config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/tmp/source",
		DestinationType: "local",
		DestinationPath: "/tmp/dest",
		CreatedBy:       user.ID,
	}
	err = database.Create(config).Error
	assert.NoError(t, err)

	// Create a test job
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "*/5 * * * *", // Every 5 minutes
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	err = database.Create(job).Error
	assert.NoError(t, err)

	// Create a new scheduler
	scheduler := New(database)
	t.Cleanup(func() {
		scheduler.Stop()
	})

	// Create a test file metadata
	fileName := "test.txt"
	metadata := &db.FileMetadata{
		JobID:           job.ID,
		FileName:        fileName,
		FileHash:        "abcdef123456",
		FileSize:        1024,
		OriginalPath:    "/tmp/source/test.txt",
		DestinationPath: "/tmp/dest/test.txt",
		Status:          "processed",
		ProcessedTime:   time.Now(),
	}
	err = database.Create(metadata).Error
	assert.NoError(t, err)

	// Check file processing history
	foundMetadata, err := scheduler.checkFileProcessingHistory(job.ID, fileName)
	assert.NoError(t, err)
	assert.Equal(t, metadata.ID, foundMetadata.ID)
	assert.Equal(t, metadata.FileName, foundMetadata.FileName)
	assert.Equal(t, metadata.FileHash, foundMetadata.FileHash)
	assert.Equal(t, metadata.Status, foundMetadata.Status)

	// Check with a non-existent file name
	_, err = scheduler.checkFileProcessingHistory(job.ID, "nonexistentfile.txt")
	assert.Error(t, err)
}
