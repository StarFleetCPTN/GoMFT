package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestDeleteTransferConfigWithTransaction tests the DeleteTransferConfig function with transaction scenarios
func TestDeleteTransferConfigWithTransaction(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "delete-config-test@example.com",
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a test config
	testConfig := &TransferConfig{
		Name:            "Test Delete Config",
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(testConfig)
	assert.NoError(t, err)

	// Test successful deletion
	err = db.DeleteTransferConfig(testConfig.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = db.GetTransferConfig(testConfig.ID)
	assert.Error(t, err, "Config should be deleted")

	// Test deletion with transaction that's rolled back
	// Create another config
	testConfig2 := &TransferConfig{
		Name:            "Test Delete Config 2",
		SourceType:      "local",
		SourcePath:      "/source/path2",
		DestinationType: "local",
		DestinationPath: "/destination/path2",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(testConfig2)
	assert.NoError(t, err)

	// Start a transaction
	tx := db.Begin()
	assert.NotNil(t, tx)

	// Delete the config within the transaction
	err = tx.Delete(&TransferConfig{}, testConfig2.ID).Error
	assert.NoError(t, err)

	// Rollback the transaction
	tx.Rollback()

	// Verify the config still exists
	config, err := db.GetTransferConfig(testConfig2.ID)
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, testConfig2.ID, config.ID)

	// Test deletion with a committed transaction
	tx = db.Begin()
	assert.NotNil(t, tx)

	// Delete the config within the transaction
	err = tx.Delete(&TransferConfig{}, testConfig2.ID).Error
	assert.NoError(t, err)

	// Commit the transaction
	tx.Commit()

	// Verify the config is deleted
	_, err = db.GetTransferConfig(testConfig2.ID)
	assert.Error(t, err, "Config should be deleted after commit")
}

// TestDeleteJobWithTransaction tests the DeleteJob function with transaction scenarios
func TestDeleteJobWithTransaction(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	testUser := &User{
		Email:              "delete-job-test@example.com",
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := db.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a test transfer config
	testConfig := &TransferConfig{
		Name:            "Test Delete Job Config",
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		CreatedBy:       testUser.ID,
	}
	err = db.CreateTransferConfig(testConfig)
	assert.NoError(t, err)

	// Create a test job
	testJob := &Job{
		Name:      "Test Delete Job",
		ConfigID:  testConfig.ID,
		Schedule:  "0 * * * *", // Run hourly
		Enabled:   BoolPtr(true),
		CreatedBy: testUser.ID,
	}
	err = db.CreateJob(testJob)
	assert.NoError(t, err)

	// Test successful deletion
	err = db.DeleteJob(testJob.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = db.GetJob(testJob.ID)
	assert.Error(t, err, "Job should be deleted")

	// Test deletion with transaction that's rolled back
	// Create another job
	testJob2 := &Job{
		Name:      "Test Delete Job 2",
		ConfigID:  testConfig.ID,
		Schedule:  "0 * * * *", // Run hourly
		Enabled:   BoolPtr(true),
		CreatedBy: testUser.ID,
	}
	err = db.CreateJob(testJob2)
	assert.NoError(t, err)

	// Start a transaction
	tx := db.Begin()
	assert.NotNil(t, tx)

	// Delete the job within the transaction
	err = tx.Delete(&Job{}, testJob2.ID).Error
	assert.NoError(t, err)

	// Rollback the transaction
	tx.Rollback()

	// Verify the job still exists
	job, err := db.GetJob(testJob2.ID)
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, testJob2.ID, job.ID)

	// Test deletion with a committed transaction
	tx = db.Begin()
	assert.NotNil(t, tx)

	// Delete the job within the transaction
	err = tx.Delete(&Job{}, testJob2.ID).Error
	assert.NoError(t, err)

	// Commit the transaction
	tx.Commit()

	// Verify the job is deleted
	_, err = db.GetJob(testJob2.ID)
	assert.Error(t, err, "Job should be deleted after commit")
}

// TestTransactionHelpers tests transaction helper methods
func TestTransactionHelpers(t *testing.T) {
	db := setupTestDB(t)

	// Test Begin and Rollback
	tx := db.Begin()
	assert.NotNil(t, tx)
	assert.IsType(t, &gorm.DB{}, tx)

	// Rollback should succeed
	err := tx.Rollback().Error
	assert.NoError(t, err)

	// Test Begin and Commit
	tx = db.Begin()
	assert.NotNil(t, tx)

	// Commit should succeed
	err = tx.Commit().Error
	assert.NoError(t, err)
}
