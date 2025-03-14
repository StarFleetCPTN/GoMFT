package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDeleteTransferConfigEdgeCases tests edge cases for the DeleteTransferConfig function
func TestDeleteTransferConfigEdgeCases(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "config-edge-test@example.com",
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create multiple configs
	configs := make([]*TransferConfig, 5)
	for i := 0; i < 5; i++ {
		config := &TransferConfig{
			Name:            fmt.Sprintf("Edge Config %d", i),
			SourceType:      "local",
			SourcePath:      fmt.Sprintf("/source/path/%d", i),
			DestinationType: "local",
			DestinationPath: fmt.Sprintf("/destination/path/%d", i),
			CreatedBy:       testUser.ID,
		}
		err = db.CreateTransferConfig(config)
		assert.NoError(t, err)
		configs[i] = config
	}

	// Delete them in reverse order
	for i := 4; i >= 0; i-- {
		err = db.DeleteTransferConfig(configs[i].ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = db.GetTransferConfig(configs[i].ID)
		assert.Error(t, err, "Config should be deleted")
	}

	// Test deleting a config that has a job associated with it
	configWithJob := &TransferConfig{
		Name:            "Config with Job",
		SourceType:      "local",
		SourcePath:      "/source/path/job",
		DestinationType: "local",
		DestinationPath: "/destination/path/job",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(configWithJob)
	assert.NoError(t, err)

	// Create a job for this config
	job := &Job{
		Name:      "Job for Config",
		ConfigID:  configWithJob.ID,
		Schedule:  "0 * * * *",
		Enabled:   true,
		CreatedBy: testUser.ID,
	}
	err = db.CreateJob(job)
	assert.NoError(t, err)

	// Try to delete the config - this should fail due to foreign key constraint
	err = db.DeleteTransferConfig(configWithJob.ID)
	assert.Error(t, err, "Should not be able to delete config with associated jobs")
	assert.Contains(t, err.Error(), "jobs are using this configuration", "Error should mention jobs")

	// Delete the job first
	err = db.DeleteJob(job.ID)
	assert.NoError(t, err)

	// Now delete the config - this should succeed
	err = db.DeleteTransferConfig(configWithJob.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = db.GetTransferConfig(configWithJob.ID)
	assert.Error(t, err, "Config should be deleted")
}

// TestDeleteJobEdgeCases tests edge cases for the DeleteJob function
func TestDeleteJobEdgeCases(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "job-edge-test@example.com",
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a test config
	config := &TransferConfig{
		Name:            "Config for Job Edge Cases",
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(config)
	assert.NoError(t, err)

	// Create multiple jobs
	jobs := make([]*Job, 5)
	for i := 0; i < 5; i++ {
		job := &Job{
			Name:      fmt.Sprintf("Edge Job %d", i),
			ConfigID:  config.ID,
			Schedule:  "0 * * * *",
			Enabled:   true,
			CreatedBy: testUser.ID,
		}
		err = db.CreateJob(job)
		assert.NoError(t, err)
		jobs[i] = job
	}

	// Delete them in reverse order
	for i := 4; i >= 0; i-- {
		err = db.DeleteJob(jobs[i].ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = db.GetJob(jobs[i].ID)
		assert.Error(t, err, "Job should be deleted")
	}

	// Create a job with history records
	jobWithHistory := &Job{
		Name:      "Job with History",
		ConfigID:  config.ID,
		Schedule:  "0 * * * *",
		Enabled:   true,
		CreatedBy: testUser.ID,
	}
	err = db.CreateJob(jobWithHistory)
	assert.NoError(t, err)

	// Create history records
	for i := 0; i < 3; i++ {
		startTime := time.Now().Add(time.Duration(-i) * time.Hour)
		endTime := startTime.Add(30 * time.Minute)
		history := &JobHistory{
			JobID:            jobWithHistory.ID,
			StartTime:        startTime,
			EndTime:          &endTime,
			Status:           "completed",
			BytesTransferred: int64(1024 * (i + 1)),
			FilesTransferred: i + 1,
		}
		err = db.CreateJobHistory(history)
		assert.NoError(t, err)
	}

	// Now delete the job - this should succeed even with history records
	// (due to foreign key constraints in the database)
	err = db.DeleteJob(jobWithHistory.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = db.GetJob(jobWithHistory.ID)
	assert.Error(t, err, "Job should be deleted")
}

// TestInitializeEdgeCases tests edge cases for the Initialize function
func TestInitializeEdgeCases(t *testing.T) {
	// Test with a read-only directory (if possible)
	tempDir, err := os.MkdirTemp("", "gomft_test_readonly")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Try to make the directory read-only
	// Note: This may not work on all systems due to permissions
	origPerms, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	// Try to make it read-only
	err = os.Chmod(tempDir, 0400) // read-only
	if err != nil {
		t.Logf("Warning: Could not set directory to read-only: %v", err)
		t.Skip("Could not set directory to read-only, skipping test")
	}
	defer os.Chmod(tempDir, origPerms.Mode()) // restore original permissions

	dbPath := filepath.Join(tempDir, "readonly.db")
	// This might fail because the directory is read-only
	db, err := Initialize(dbPath)
	if err != nil {
		// Expected error due to read-only directory
		t.Logf("Got expected error for read-only directory: %v", err)
	} else {
		// If it succeeded, clean up
		t.Logf("Warning: DB initialization succeeded even with read-only directory!")
		err = db.Close()
		assert.NoError(t, err)
	}
}

// TestCloseEdgeCases tests edge cases for the Close function
func TestCloseEdgeCases(t *testing.T) {
	// Create a temporary database
	tempDir, err := os.MkdirTemp("", "gomft_test_close_edge")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "close_edge.db")
	db, err := Initialize(dbPath)
	assert.NoError(t, err)

	// Test calling methods after close
	sqlDB, err := db.DB.DB()
	assert.NoError(t, err)

	// Get initial stats
	stats := sqlDB.Stats()
	t.Logf("Initial stats: MaxOpenConnections=%d, OpenConnections=%d, InUse=%d",
		stats.MaxOpenConnections, stats.OpenConnections, stats.InUse)

	// Close the DB
	err = db.Close()
	assert.NoError(t, err)

	// Try to get stats again - this might fail
	stats = sqlDB.Stats()
	t.Logf("After close stats: MaxOpenConnections=%d, OpenConnections=%d, InUse=%d",
		stats.MaxOpenConnections, stats.OpenConnections, stats.InUse)

	// Verify that DB operations fail after close
	_, err = db.GetUserByEmail("test@example.com")
	assert.Error(t, err, "DB operations should fail after close")
}
