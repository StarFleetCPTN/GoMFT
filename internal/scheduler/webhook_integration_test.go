package scheduler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobExecutionWebhook tests that webhooks are correctly sent during actual job execution
func TestJobExecutionWebhook(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up a temporary data directory for logs
	tempDir := t.TempDir()

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	t.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a test user
	user := &db.User{
		Email:        "webhook-integration@example.com",
		PasswordHash: "hashed_password",
		IsAdmin:      BoolPtr(true),
	}
	err := database.CreateUser(user)
	require.NoError(t, err)

	// Set up a mock HTTP server to receive webhook notifications
	var (
		receivedPayload []byte
		receivedHeaders http.Header
		webhookCalled   bool
		webhookMutex    sync.Mutex
		waitCh          = make(chan struct{})
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookMutex.Lock()
		defer webhookMutex.Unlock()

		receivedHeaders = r.Header.Clone()
		var err error
		receivedPayload, err = io.ReadAll(r.Body)
		if err != nil {
			t.Logf("Error reading request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		t.Logf("Received webhook payload: %s", string(receivedPayload))
		webhookCalled = true
		close(waitCh)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()
	t.Logf("Mock server URL: %s", mockServer.URL)

	// Create local source and destination directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()
	t.Logf("Source directory: %s", sourceDir)
	t.Logf("Destination directory: %s", destDir)

	// Create a test transfer config with local source and destination
	config := &db.TransferConfig{
		Name:            "Webhook Integration Config",
		SourceType:      "local",
		SourcePath:      sourceDir,
		DestinationType: "local",
		DestinationPath: destDir,
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(config).Error
	require.NoError(t, err)
	t.Logf("Created config with ID: %d", config.ID)

	// Create a test job with webhook enabled
	job := &db.Job{
		Name:            "Webhook Integration Job",
		ConfigID:        config.ID,
		Schedule:        "*/5 * * * *", // not actually used in this test
		Enabled:         BoolPtr(true),
		WebhookEnabled:  BoolPtr(true),
		WebhookURL:      mockServer.URL,
		NotifyOnSuccess: BoolPtr(true),
		NotifyOnFailure: BoolPtr(true),
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(job).Error
	require.NoError(t, err)

	t.Logf("Created job with ID %d, NotifyOnSuccess=%v", job.ID, job.NotifyOnSuccess)

	// Create and initialize the scheduler
	scheduler := New(database)
	defer scheduler.Stop()

	// Create rclone config directory and file
	configDir := filepath.Join(tempDir, "configs")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Create a minimal rclone config file
	rcloneConfig := `
[source_1]
type = local

[dest_1]
type = local
`
	configFile := filepath.Join(configDir, "config_1.conf")
	err = os.WriteFile(configFile, []byte(rcloneConfig), 0644)
	require.NoError(t, err)
	t.Logf("Created rclone config file: %s", configFile)

	// Put a test file in the source directory
	testFile := filepath.Join(sourceDir, "test.txt")
	testFileContent := []byte("This is a test file for webhook integration testing.")
	err = os.WriteFile(testFile, testFileContent, 0644)
	require.NoError(t, err)
	t.Logf("Created test file: %s", testFile)

	// Check that the file exists
	fileInfo, err := os.Stat(testFile)
	require.NoError(t, err, "Test file should exist")
	t.Logf("Test file size: %d bytes", fileInfo.Size())

	// Manually trigger job execution
	t.Logf("Running job now...")
	err = scheduler.RunJobNow(job.ID)
	require.NoError(t, err)

	// Wait for the job to complete and webhook to be called (up to 15 seconds)
	t.Logf("Waiting for webhook to be called...")
	timeout := time.After(15 * time.Second)
	select {
	case <-waitCh:
		t.Logf("Webhook was called")
	case <-timeout:
		// Before failing, check job status
		var histories []db.JobHistory
		err = database.DB.Where("job_id = ?", job.ID).Find(&histories).Error
		require.NoError(t, err)

		if len(histories) > 0 {
			t.Logf("Job history found: status=%s, error=%s",
				histories[0].Status, histories[0].ErrorMessage)
		} else {
			t.Logf("No job history found")
		}

		// Check if destination file exists
		destFile := filepath.Join(destDir, "test.txt")
		if _, err := os.Stat(destFile); err == nil {
			t.Logf("Destination file exists, but webhook was not called")
		} else {
			t.Logf("Destination file does not exist: %v", err)
		}

		webhookMutex.Lock()
		called := webhookCalled
		webhookMutex.Unlock()

		if called {
			t.Logf("Webhook was actually called but channel synchronization failed")
		} else {
			t.Fatal("Timed out waiting for webhook to be called")
		}
		return
	}

	// Verify the webhook notification
	webhookMutex.Lock()
	payload := receivedPayload
	headers := receivedHeaders
	webhookMutex.Unlock()

	assert.NotNil(t, payload, "Webhook notification should have been sent")

	// Verify the payload content
	var payloadMap map[string]interface{}
	err = json.Unmarshal(payload, &payloadMap)
	require.NoError(t, err, "Failed to unmarshal webhook payload")

	// Check essential fields
	assert.Equal(t, "job_execution", payloadMap["event_type"])
	assert.Equal(t, float64(job.ID), payloadMap["job_id"])
	assert.Equal(t, job.Name, payloadMap["job_name"])
	assert.Equal(t, float64(config.ID), payloadMap["config_id"])
	assert.Equal(t, config.Name, payloadMap["config_name"])

	// Check status (should be "completed" or "completed_with_errors")
	status, ok := payloadMap["status"].(string)
	require.True(t, ok, "Status should be a string")
	assert.Contains(t, []string{"completed", "completed_with_errors"}, status)

	// Check that we have bytes transferred
	bytesTransferred, ok := payloadMap["bytes_transferred"].(float64)
	require.True(t, ok, "bytes_transferred should be a number")
	assert.Greater(t, bytesTransferred, float64(0))

	// Check that we have files transferred
	filesTransferred, ok := payloadMap["files_transferred"].(float64)
	require.True(t, ok, "files_transferred should be a number")
	assert.Equal(t, float64(1), filesTransferred)

	// Check standard headers
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, "GoMFT-Webhook/1.0", headers.Get("User-Agent"))

	// Check that the file was actually transferred
	destFile := filepath.Join(destDir, "test.txt")
	_, err = os.Stat(destFile)
	assert.NoError(t, err, "The file should have been transferred")

	// Clean up
	err = database.DB.Unscoped().Delete(job).Error
	require.NoError(t, err)
	err = database.DB.Unscoped().Where("job_id = ?", job.ID).Delete(&db.JobHistory{}).Error
	require.NoError(t, err)
}

// TestFailedJobWebhook tests that webhooks are correctly sent for failed jobs
func TestFailedJobWebhook(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up a temporary data directory for logs
	tempDir := t.TempDir()

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	t.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a test user
	user := &db.User{
		Email:        "webhook-failure@example.com",
		PasswordHash: "hashed_password",
		IsAdmin:      BoolPtr(true),
	}
	err := database.CreateUser(user)
	require.NoError(t, err)

	// Set up a mock HTTP server to receive webhook notifications
	var (
		receivedPayload []byte
		webhookCalled   bool
		webhookMutex    sync.Mutex
		waitCh          = make(chan struct{})
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookMutex.Lock()
		defer webhookMutex.Unlock()

		var err error
		receivedPayload, err = io.ReadAll(r.Body)
		if err != nil {
			t.Logf("Error reading request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		t.Logf("Received webhook payload: %s", string(receivedPayload))
		webhookCalled = true
		close(waitCh)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Get a non-existent directory for source
	nonexistentDir := filepath.Join(t.TempDir(), "non-existent-subdirectory")

	// Create a legitimate destination directory
	destDir := t.TempDir()

	// Create a test transfer config with invalid source (to trigger failure)
	config := &db.TransferConfig{
		Name:            "Webhook Failure Config",
		SourceType:      "local",
		SourcePath:      nonexistentDir,
		DestinationType: "local",
		DestinationPath: destDir,
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(config).Error
	require.NoError(t, err)
	t.Logf("Created config with invalid source path: %s", nonexistentDir)

	// Create a test job with webhook enabled
	job := &db.Job{
		Name:            "Webhook Failure Job",
		ConfigID:        config.ID,
		Schedule:        "*/5 * * * *", // not actually used in this test
		Enabled:         BoolPtr(true),
		WebhookEnabled:  BoolPtr(true),
		WebhookURL:      mockServer.URL,
		NotifyOnSuccess: BoolPtr(true),
		NotifyOnFailure: BoolPtr(true),
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(job).Error
	require.NoError(t, err)

	// Create and initialize the scheduler
	scheduler := New(database)
	defer scheduler.Stop()

	// Create rclone config directory and file
	configDir := filepath.Join(tempDir, "configs")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Create a minimal rclone config file
	rcloneConfig := `
[source_1]
type = local

[dest_1]
type = local
`
	configFile := filepath.Join(configDir, "config_1.conf")
	err = os.WriteFile(configFile, []byte(rcloneConfig), 0644)
	require.NoError(t, err)
	t.Logf("Created rclone config file: %s", configFile)

	// Manually trigger job execution
	t.Logf("Running job now (expecting failure)...")
	err = scheduler.RunJobNow(job.ID)
	require.NoError(t, err)

	// Wait for the job to complete and webhook to be called (up to 15 seconds)
	t.Logf("Waiting for webhook to be called with failure notification...")
	timeout := time.After(15 * time.Second)
	select {
	case <-waitCh:
		t.Logf("Webhook was called")
	case <-timeout:
		// Before failing, check job status
		var histories []db.JobHistory
		err = database.DB.Where("job_id = ?", job.ID).Find(&histories).Error
		require.NoError(t, err)

		if len(histories) > 0 {
			t.Logf("Job history found: status=%s, error=%s",
				histories[0].Status, histories[0].ErrorMessage)
		} else {
			t.Logf("No job history found")
		}

		webhookMutex.Lock()
		called := webhookCalled
		webhookMutex.Unlock()

		if called {
			t.Logf("Webhook was actually called but channel synchronization failed")
		} else {
			t.Fatal("Timed out waiting for webhook to be called")
		}
		return
	}

	// Verify the webhook notification
	assert.NotNil(t, receivedPayload, "Webhook notification should have been sent")

	// Verify the payload content
	var payload map[string]interface{}
	err = json.Unmarshal(receivedPayload, &payload)
	require.NoError(t, err, "Failed to unmarshal webhook payload")

	// Check essential fields
	assert.Equal(t, "job_execution", payload["event_type"])
	assert.Equal(t, float64(job.ID), payload["job_id"])
	assert.Equal(t, "failed", payload["status"])

	// Ensure there's an error message
	errorMsg, ok := payload["error_message"].(string)
	require.True(t, ok, "error_message should be a string")
	assert.NotEmpty(t, errorMsg)
	t.Logf("Error message from webhook: %s", errorMsg)

	// Clean up
	err = database.DB.Unscoped().Delete(job).Error
	require.NoError(t, err)
	err = database.DB.Unscoped().Where("job_id = ?", job.ID).Delete(&db.JobHistory{}).Error
	require.NoError(t, err)
}

// TestWebhookDisabledForSuccessNotification tests that webhooks are not sent for
// successful jobs when notify_on_success is disabled
func TestWebhookDisabledForSuccessNotification(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up a temporary data directory for logs
	tempDir := t.TempDir()

	// Set DATA_DIR environment variable for the test
	originalDataDir := os.Getenv("DATA_DIR")
	t.Setenv("DATA_DIR", tempDir)
	defer os.Setenv("DATA_DIR", originalDataDir)

	// Create a test database
	database := setupTestDB(t)

	// Create a test user
	user := &db.User{
		Email:        "webhook-disabled@example.com",
		PasswordHash: "hashed_password",
		IsAdmin:      BoolPtr(true),
	}
	err := database.CreateUser(user)
	require.NoError(t, err)

	// Set up a mock HTTP server to receive webhook notifications
	var (
		webhookCalled bool
		webhookMutex  sync.Mutex
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookMutex.Lock()
		defer webhookMutex.Unlock()

		// Log the fact that webhook was called (it shouldn't be)
		body, _ := io.ReadAll(r.Body)
		t.Logf("Unexpected webhook call received: %s", string(body))

		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Create local source and destination directories
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// Create a test transfer config with local source and destination
	config := &db.TransferConfig{
		Name:            "Webhook Disabled Config",
		SourceType:      "local",
		SourcePath:      sourceDir,
		DestinationType: "local",
		DestinationPath: destDir,
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(config).Error
	require.NoError(t, err)

	// Create a test job with webhook enabled but notify_on_success disabled
	job := &db.Job{
		Name:            "Webhook Disabled Job",
		ConfigID:        config.ID,
		Schedule:        "*/5 * * * *", // not actually used in this test
		Enabled:         BoolPtr(true),
		WebhookEnabled:  BoolPtr(true),
		WebhookURL:      mockServer.URL,
		NotifyOnSuccess: BoolPtr(false), // This is the key setting we're testing
		NotifyOnFailure: BoolPtr(true),
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(job).Error
	require.NoError(t, err)

	// Update the job to ensure the notification settings are correctly set
	// This is necessary because the database has default values for these fields
	err = database.DB.Model(job).Updates(map[string]interface{}{
		"notify_on_success": false,
	}).Error
	require.NoError(t, err)

	// Reload the job to make sure we have the correct values
	var reloadedJob db.Job
	err = database.DB.First(&reloadedJob, job.ID).Error
	require.NoError(t, err)
	job = &reloadedJob

	t.Logf("Created job with ID %d, NotifyOnSuccess=%v", job.ID, job.NotifyOnSuccess)

	// Create and initialize the scheduler
	scheduler := New(database)
	defer scheduler.Stop()

	// Create rclone config directory and file
	configDir := filepath.Join(tempDir, "configs")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Create a minimal rclone config file
	rcloneConfig := `
[source_1]
type = local

[dest_1]
type = local
`
	configFile := filepath.Join(configDir, "config_1.conf")
	err = os.WriteFile(configFile, []byte(rcloneConfig), 0644)
	require.NoError(t, err)
	t.Logf("Created rclone config file: %s", configFile)

	// Put a test file in the source directory
	testFile := filepath.Join(sourceDir, "test.txt")
	testFileContent := []byte("This is a test file for disabled webhook testing.")
	err = os.WriteFile(testFile, testFileContent, 0644)
	require.NoError(t, err)

	// Manually trigger job execution
	t.Logf("Running job now...")
	err = scheduler.RunJobNow(job.ID)
	require.NoError(t, err)

	// Wait for a bit to ensure job completes (10 seconds should be plenty)
	time.Sleep(10 * time.Second)

	// Check if webhook was called (it should not have been)
	webhookMutex.Lock()
	called := webhookCalled
	webhookMutex.Unlock()

	assert.False(t, called, "Webhook should not have been called for successful job with NotifyOnSuccess=false")

	// Verify the job actually ran successfully by checking for the file
	destFile := filepath.Join(destDir, "test.txt")
	_, err = os.Stat(destFile)
	assert.NoError(t, err, "The job should have completed and transferred the file")

	// Verify job history has been created and shows completion
	var histories []db.JobHistory
	err = database.DB.Where("job_id = ?", job.ID).Find(&histories).Error
	require.NoError(t, err)

	if len(histories) > 0 {
		t.Logf("Job history found: status=%s", histories[0].Status)
		assert.Equal(t, "completed", histories[0].Status, "Job should have completed successfully")
	}

	// Clean up
	err = database.DB.Unscoped().Delete(job).Error
	require.NoError(t, err)
	err = database.DB.Unscoped().Where("job_id = ?", job.ID).Delete(&db.JobHistory{}).Error
	require.NoError(t, err)
}
