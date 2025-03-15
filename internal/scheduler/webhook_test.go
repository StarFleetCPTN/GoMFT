package scheduler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebhookNotification tests the webhook notification functionality
func TestWebhookNotification(t *testing.T) {
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
		Email:        "webhook-test@example.com",
		PasswordHash: "hashed_password",
		IsAdmin:      true,
	}
	err := database.CreateUser(user)
	require.NoError(t, err)

	// Create a test transfer config
	config := &db.TransferConfig{
		Name:            "Webhook Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(config).Error
	require.NoError(t, err)

	// Create a mock HTTP server to receive webhook notifications
	var (
		receivedPayload []byte
		receivedHeaders http.Header
		webhookCalled   bool
		webhookMutex    sync.Mutex
		waitCh          chan struct{}
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

		// Debug output to help understand what's happening
		t.Logf("Webhook called with payload: %s", string(receivedPayload))

		webhookCalled = true
		w.WriteHeader(http.StatusOK)

		// Signal that webhook was called
		if waitCh != nil {
			close(waitCh)
		}
	}))
	defer mockServer.Close()

	// Create a test scheduler
	scheduler := New(database)
	defer scheduler.Stop()

	// Test cases
	tests := []struct {
		name               string
		job                *db.Job
		history            *db.JobHistory
		webhookEnabled     bool
		webhookURL         string
		webhookSecret      string
		webhookHeaders     map[string]string
		notifyOnSuccess    bool
		notifyOnFailure    bool
		status             string
		expectNotification bool
	}{
		{
			name: "Successful job with notification",
			job: &db.Job{
				Name:            "Success Job",
				ConfigID:        config.ID,
				WebhookEnabled:  true,
				WebhookURL:      mockServer.URL,
				NotifyOnSuccess: true,
				NotifyOnFailure: true,
				CreatedBy:       user.ID,
			},
			history: &db.JobHistory{
				Status:           "completed",
				StartTime:        time.Now().Add(-5 * time.Minute),
				EndTime:          timePtr(time.Now()),
				BytesTransferred: 1024,
				FilesTransferred: 2,
			},
			webhookEnabled:     true,
			webhookURL:         mockServer.URL,
			notifyOnSuccess:    true,
			notifyOnFailure:    true,
			status:             "completed",
			expectNotification: true,
		},
		{
			name: "Failed job with notification",
			job: &db.Job{
				Name:            "Failed Job",
				ConfigID:        config.ID,
				WebhookEnabled:  true,
				WebhookURL:      mockServer.URL,
				NotifyOnSuccess: true,
				NotifyOnFailure: true,
				CreatedBy:       user.ID,
			},
			history: &db.JobHistory{
				Status:       "failed",
				StartTime:    time.Now().Add(-5 * time.Minute),
				EndTime:      timePtr(time.Now()),
				ErrorMessage: "Test error message",
			},
			webhookEnabled:     true,
			webhookURL:         mockServer.URL,
			notifyOnSuccess:    true,
			notifyOnFailure:    true,
			status:             "failed",
			expectNotification: true,
		},
		{
			name: "Successful job with notification disabled for success",
			job: &db.Job{
				Name:            "Success Job No Notify",
				ConfigID:        config.ID,
				WebhookEnabled:  true,
				WebhookURL:      mockServer.URL,
				NotifyOnSuccess: false,
				NotifyOnFailure: true,
				CreatedBy:       user.ID,
			},
			history: &db.JobHistory{
				Status:    "completed",
				StartTime: time.Now().Add(-5 * time.Minute),
				EndTime:   timePtr(time.Now()),
			},
			webhookEnabled:     true,
			webhookURL:         mockServer.URL,
			notifyOnSuccess:    false,
			notifyOnFailure:    true,
			status:             "completed",
			expectNotification: false,
		},
		{
			name: "Failed job with notification disabled for failure",
			job: &db.Job{
				Name:            "Failed Job No Notify",
				ConfigID:        config.ID,
				WebhookEnabled:  true,
				WebhookURL:      mockServer.URL,
				NotifyOnSuccess: true,
				NotifyOnFailure: false,
				CreatedBy:       user.ID,
			},
			history: &db.JobHistory{
				Status:       "failed",
				StartTime:    time.Now().Add(-5 * time.Minute),
				EndTime:      timePtr(time.Now()),
				ErrorMessage: "Test error message",
			},
			webhookEnabled:     true,
			webhookURL:         mockServer.URL,
			notifyOnSuccess:    true,
			notifyOnFailure:    false,
			status:             "failed",
			expectNotification: false,
		},
		{
			name: "Webhook disabled",
			job: &db.Job{
				Name:            "Webhook Disabled",
				ConfigID:        config.ID,
				WebhookEnabled:  false,
				WebhookURL:      mockServer.URL,
				NotifyOnSuccess: true,
				NotifyOnFailure: true,
				CreatedBy:       user.ID,
			},
			history: &db.JobHistory{
				Status:    "completed",
				StartTime: time.Now().Add(-5 * time.Minute),
				EndTime:   timePtr(time.Now()),
			},
			webhookEnabled:     false,
			webhookURL:         mockServer.URL,
			notifyOnSuccess:    true,
			notifyOnFailure:    true,
			status:             "completed",
			expectNotification: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset received data
			webhookMutex.Lock()
			receivedPayload = nil
			receivedHeaders = nil
			webhookCalled = false
			waitCh = make(chan struct{})
			webhookMutex.Unlock()

			// Debug the test case configuration
			t.Logf("Test configuration: name=%s, webhookEnabled=%v, notifyOnSuccess=%v, notifyOnFailure=%v, status=%s, expectNotification=%v",
				tc.name, tc.webhookEnabled, tc.notifyOnSuccess, tc.notifyOnFailure, tc.status, tc.expectNotification)

			// Create a new job instance for each test case
			job := &db.Job{
				Name:            tc.job.Name,
				ConfigID:        tc.job.ConfigID,
				WebhookEnabled:  tc.webhookEnabled,
				WebhookURL:      tc.webhookURL,
				NotifyOnSuccess: tc.notifyOnSuccess,
				NotifyOnFailure: tc.notifyOnFailure,
				CreatedBy:       tc.job.CreatedBy,
			}

			t.Logf("Job before DB create: WebhookEnabled=%v, NotifyOnSuccess=%v, NotifyOnFailure=%v",
				job.WebhookEnabled, job.NotifyOnSuccess, job.NotifyOnFailure)

			err := database.DB.Create(job).Error
			require.NoError(t, err)

			// Update the job to ensure the notification settings are correctly set
			// This is necessary because the database has default values for these fields
			err = database.DB.Model(job).Updates(map[string]interface{}{
				"notify_on_success": tc.notifyOnSuccess,
				"notify_on_failure": tc.notifyOnFailure,
			}).Error
			require.NoError(t, err)

			// Reload the job to make sure we have the correct values
			var reloadedJob db.Job
			err = database.DB.First(&reloadedJob, job.ID).Error
			require.NoError(t, err)
			job = &reloadedJob

			t.Logf("Job after DB create: WebhookEnabled=%v, NotifyOnSuccess=%v, NotifyOnFailure=%v",
				job.WebhookEnabled, job.NotifyOnSuccess, job.NotifyOnFailure)

			// Create and save job history
			history := tc.history
			history.JobID = job.ID

			err = database.DB.Create(history).Error
			require.NoError(t, err)

			// Debug info
			t.Logf("Test case: %s", tc.name)
			t.Logf("Job settings: WebhookEnabled=%v, NotifyOnSuccess=%v, NotifyOnFailure=%v",
				job.WebhookEnabled, job.NotifyOnSuccess, job.NotifyOnFailure)
			t.Logf("History status: %s", history.Status)

			// Send webhook notification
			scheduler.sendWebhookNotification(job, history, config)

			// Wait for webhook call to complete if expected
			if tc.expectNotification {
				// Wait with timeout for webhook to be called
				select {
				case <-waitCh:
					// Webhook was called
				case <-time.After(2 * time.Second):
					t.Fatalf("Timed out waiting for webhook to be called")
				}
			} else {
				// Give it a small window to ensure it doesn't call when not expected
				time.Sleep(500 * time.Millisecond)
			}

			// Check if notification was sent as expected
			webhookMutex.Lock()
			called := webhookCalled
			payload := receivedPayload
			headers := receivedHeaders
			webhookMutex.Unlock()

			if tc.expectNotification {
				assert.True(t, called, "Expected webhook notification to be sent")
				require.NotNil(t, payload, "Expected webhook payload to be non-nil")

				// Verify the payload
				var payloadMap map[string]interface{}
				err := json.Unmarshal(payload, &payloadMap)
				require.NoError(t, err, "Failed to unmarshal webhook payload")

				// Check common fields
				assert.Equal(t, "job_execution", payloadMap["event_type"])
				assert.Equal(t, float64(job.ID), payloadMap["job_id"])
				assert.Equal(t, job.Name, payloadMap["job_name"])
				assert.Equal(t, float64(config.ID), payloadMap["config_id"])
				assert.Equal(t, config.Name, payloadMap["config_name"])
				assert.Equal(t, history.Status, payloadMap["status"])

				// Check headers
				assert.Equal(t, "application/json", headers.Get("Content-Type"))
				assert.Equal(t, "GoMFT-Webhook/1.0", headers.Get("User-Agent"))

				// Additional checks for specific status
				if history.Status == "failed" {
					assert.Equal(t, history.ErrorMessage, payloadMap["error_message"])
				}
			} else {
				assert.False(t, called, "Expected no webhook notification to be sent")
			}

			// Clean up
			err = database.DB.Unscoped().Delete(history).Error
			require.NoError(t, err)
			err = database.DB.Unscoped().Delete(job).Error
			require.NoError(t, err)
		})
	}
}

// TestWebhookAuthentication tests the webhook authentication functionality
func TestWebhookAuthentication(t *testing.T) {
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
		Email:        "webhook-auth-test@example.com",
		PasswordHash: "hashed_password",
		IsAdmin:      true,
	}
	err := database.CreateUser(user)
	require.NoError(t, err)

	// Create a test transfer config
	config := &db.TransferConfig{
		Name:            "Webhook Auth Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(config).Error
	require.NoError(t, err)

	// Create a mock HTTP server to receive webhook notifications
	var (
		receivedPayload []byte
		receivedHeaders http.Header
		waitCh          = make(chan struct{})
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		var err error
		receivedPayload, err = io.ReadAll(r.Body)
		if err != nil {
			t.Logf("Error reading request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		close(waitCh)
	}))
	defer mockServer.Close()

	// Create a test scheduler
	scheduler := New(database)
	defer scheduler.Stop()

	// Set up job with webhook secret
	secret := "test-webhook-secret"
	job := &db.Job{
		Name:            "Auth Test Job",
		ConfigID:        config.ID,
		WebhookEnabled:  true,
		WebhookURL:      mockServer.URL,
		WebhookSecret:   secret,
		NotifyOnSuccess: true,
		NotifyOnFailure: true,
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(job).Error
	require.NoError(t, err)

	// Create job history
	history := &db.JobHistory{
		JobID:            job.ID,
		Status:           "completed",
		StartTime:        time.Now().Add(-5 * time.Minute),
		EndTime:          timePtr(time.Now()),
		BytesTransferred: 1024,
		FilesTransferred: 2,
	}
	err = database.DB.Create(history).Error
	require.NoError(t, err)

	// Send webhook notification
	scheduler.sendWebhookNotification(job, history, config)

	// Wait for webhook to be called
	select {
	case <-waitCh:
		// Webhook was called
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for webhook to be called")
	}

	// Verify the signature
	require.NotNil(t, receivedPayload, "Expected webhook notification to be sent")

	// Check that the X-Hub-Signature-256 header exists
	signature := receivedHeaders.Get("X-Hub-Signature-256")
	require.NotEmpty(t, signature, "Expected X-Hub-Signature-256 header to be set")

	// Verify that the signature matches the expected HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(receivedPayload)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Print both signatures for debugging if they don't match
	if expectedSignature != signature {
		t.Logf("Expected signature: %s", expectedSignature)
		t.Logf("Actual signature: %s", signature)
		t.Logf("Secret used: %s", secret)
		t.Logf("Payload length: %d", len(receivedPayload))
	}

	assert.Equal(t, expectedSignature, signature, "Signature does not match expected value")

	// Clean up
	err = database.DB.Unscoped().Delete(history).Error
	require.NoError(t, err)
	err = database.DB.Unscoped().Delete(job).Error
	require.NoError(t, err)
}

// TestWebhookCustomHeaders tests the custom headers functionality for webhooks
func TestWebhookCustomHeaders(t *testing.T) {
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
		Email:        "webhook-headers-test@example.com",
		PasswordHash: "hashed_password",
		IsAdmin:      true,
	}
	err := database.CreateUser(user)
	require.NoError(t, err)

	// Create a test transfer config
	config := &db.TransferConfig{
		Name:            "Webhook Headers Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(config).Error
	require.NoError(t, err)

	// Create a mock HTTP server to receive webhook notifications
	var (
		receivedPayload []byte
		receivedHeaders http.Header
		waitCh          = make(chan struct{})
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		var err error
		receivedPayload, err = io.ReadAll(r.Body)
		if err != nil {
			t.Logf("Error reading request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		close(waitCh)
	}))
	defer mockServer.Close()

	// Create a test scheduler
	scheduler := New(database)
	defer scheduler.Stop()

	// Define custom headers
	customHeaders := map[string]string{
		"X-API-Key":   "test-api-key",
		"X-Client-ID": "test-client-id",
		"X-Source":    "gomft-test",
	}

	customHeadersJSON, err := json.Marshal(customHeaders)
	require.NoError(t, err)

	// Set up job with custom headers
	job := &db.Job{
		Name:            "Custom Headers Test Job",
		ConfigID:        config.ID,
		WebhookEnabled:  true,
		WebhookURL:      mockServer.URL,
		WebhookHeaders:  string(customHeadersJSON),
		NotifyOnSuccess: true,
		NotifyOnFailure: true,
		CreatedBy:       user.ID,
	}
	err = database.DB.Create(job).Error
	require.NoError(t, err)

	// Create job history
	history := &db.JobHistory{
		JobID:            job.ID,
		Status:           "completed",
		StartTime:        time.Now().Add(-5 * time.Minute),
		EndTime:          timePtr(time.Now()),
		BytesTransferred: 1024,
		FilesTransferred: 2,
	}
	err = database.DB.Create(history).Error
	require.NoError(t, err)

	// Send webhook notification
	scheduler.sendWebhookNotification(job, history, config)

	// Wait for webhook to be called
	select {
	case <-waitCh:
		// Webhook was called
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for webhook to be called")
	}

	// Verify the headers
	require.NotNil(t, receivedPayload, "Expected webhook notification to be sent")

	// Check that all custom headers are present
	for key, value := range customHeaders {
		actualValue := receivedHeaders.Get(key)
		if actualValue != value {
			t.Logf("Custom header mismatch for %s: expected=%s, got=%s", key, value, actualValue)
		}
		assert.Equal(t, value, actualValue, "Expected custom header %s to be set", key)
	}

	// Also check standard headers
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "GoMFT-Webhook/1.0", receivedHeaders.Get("User-Agent"))

	// Clean up
	err = database.DB.Unscoped().Delete(history).Error
	require.NoError(t, err)
	err = database.DB.Unscoped().Delete(job).Error
	require.NoError(t, err)
}

// Helper function to create a pointer to a time.Time value
func timePtr(t time.Time) *time.Time {
	return &t
}
