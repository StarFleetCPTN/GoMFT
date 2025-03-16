package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebhookConfiguration tests the webhook configuration during job creation and editing
func TestWebhookConfiguration(t *testing.T) {
	// Set up test environment
	handlers, router, database, user, config := setupJobsTest(t)

	// Add job create route
	router.POST("/jobs/create", handlers.HandleCreateJob)

	// Create job form data with webhook enabled
	formData := url.Values{
		"name":              {"Webhook Test Job"},
		"config_ids[]":      {strconv.Itoa(int(config.ID))},
		"schedule":          {"*/15 * * * *"},
		"enabled":           {"true"},
		"webhook_enabled":   {"true"},
		"webhook_url":       {"https://example.com/webhook"},
		"webhook_secret":    {"test-secret"},
		"webhook_headers":   {`{"X-Test-Header": "test-value"}`},
		"notify_on_success": {"true"},
		"notify_on_failure": {"true"},
	}

	// Submit form
	req, _ := http.NewRequest("POST", "/jobs/create", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect on success
	assert.Equal(t, http.StatusFound, resp.Code)

	// Check if job was created with webhook settings
	var jobs []db.Job
	err := database.DB.Where("created_by = ?", user.ID).Find(&jobs).Error
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(jobs), 1)

	// Get the most recently created job
	var job db.Job
	err = database.DB.Where("created_by = ?", user.ID).Order("created_at DESC").First(&job).Error
	require.NoError(t, err)

	// Verify webhook settings were saved correctly
	assert.True(t, job.GetWebhookEnabled())
	assert.Equal(t, "https://example.com/webhook", job.WebhookURL)
	assert.Equal(t, "test-secret", job.WebhookSecret)
	assert.Equal(t, `{"X-Test-Header": "test-value"}`, job.WebhookHeaders)
	assert.True(t, job.GetNotifyOnSuccess())
	assert.True(t, job.GetNotifyOnFailure())
}

// TestWebhookEditConfiguration tests editing webhook configuration
func TestWebhookEditConfiguration(t *testing.T) {
	// Set up test environment
	handlers, router, database, user, config := setupJobsTest(t)

	// Create a job first
	job := &db.Job{
		Name:           "Initial Job",
		ConfigID:       config.ID,
		Schedule:       "*/30 * * * *",
		Enabled:        BoolPtr(true),
		WebhookEnabled: BoolPtr(false), // Initially disabled
		CreatedBy:      user.ID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	err := database.DB.Create(job).Error
	require.NoError(t, err)

	// Add job update route
	router.PUT("/jobs/:id", handlers.HandleUpdateJob)

	// Create edit form data to enable webhook
	formData := url.Values{
		"name":              {"Updated Job"},
		"config_ids[]":      {strconv.Itoa(int(config.ID))},
		"schedule":          {"*/30 * * * *"},
		"enabled":           {"true"},
		"webhook_enabled":   {"true"},                        // Enabling webhook
		"webhook_url":       {"https://example.com/webhook"}, // Adding URL
		"webhook_secret":    {"new-secret"},                  // Adding secret
		"webhook_headers":   {`{"X-Api-Key": "12345"}`},      // Adding headers
		"notify_on_success": {"true"},                        // Configure notifications
		"notify_on_failure": {"false"},                       // Only notify on success
	}

	// Submit edit form
	req, _ := http.NewRequest("PUT", "/jobs/"+strconv.Itoa(int(job.ID)), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect on success
	assert.Equal(t, http.StatusFound, resp.Code)

	// Get the updated job
	var updatedJob db.Job
	err = database.DB.First(&updatedJob, job.ID).Error
	require.NoError(t, err)

	// Verify webhook settings were updated correctly
	assert.True(t, updatedJob.GetWebhookEnabled())
	assert.Equal(t, "https://example.com/webhook", updatedJob.WebhookURL)
	assert.Equal(t, "new-secret", updatedJob.WebhookSecret)
	assert.Equal(t, `{"X-Api-Key": "12345"}`, updatedJob.WebhookHeaders)
	assert.True(t, updatedJob.GetNotifyOnSuccess())
	assert.False(t, updatedJob.GetNotifyOnFailure())
}

// TestDisablingWebhook tests disabling a previously enabled webhook
func TestDisablingWebhook(t *testing.T) {
	// Set up test environment
	handlers, router, database, user, config := setupJobsTest(t)

	// Create a job with webhook enabled
	job := &db.Job{
		Name:            "Webhook Enabled Job",
		ConfigID:        config.ID,
		Schedule:        "*/30 * * * *",
		Enabled:         BoolPtr(true),
		WebhookEnabled:  BoolPtr(true),
		WebhookURL:      "https://example.com/webhook",
		WebhookSecret:   "secret",
		WebhookHeaders:  `{"X-Test": "test"}`,
		NotifyOnSuccess: BoolPtr(true),
		NotifyOnFailure: BoolPtr(true),
		CreatedBy:       user.ID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	err := database.DB.Create(job).Error
	require.NoError(t, err)

	// Add job update route
	router.PUT("/jobs/:id", handlers.HandleUpdateJob)

	// Create edit form data to disable webhook
	formData := url.Values{
		"name":            {"Webhook Disabled Job"},
		"config_ids[]":    {strconv.Itoa(int(config.ID))},
		"schedule":        {"*/30 * * * *"},
		"enabled":         {"true"},
		"webhook_enabled": {"false"},                       // Explicitly set to false
		"webhook_url":     {"https://example.com/webhook"}, // URL remains the same
		"webhook_secret":  {"secret"},                      // Secret remains the same
		"webhook_headers": {`{"X-Test": "test"}`},          // Headers remain the same
	}

	// Submit edit form
	req, _ := http.NewRequest("PUT", "/jobs/"+strconv.Itoa(int(job.ID)), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect on success
	assert.Equal(t, http.StatusFound, resp.Code)

	// Get the updated job
	var updatedJob db.Job
	err = database.DB.First(&updatedJob, job.ID).Error
	require.NoError(t, err)

	// Verify webhook was disabled
	assert.False(t, updatedJob.GetWebhookEnabled())

	// Other fields should remain unchanged
	assert.Equal(t, "https://example.com/webhook", updatedJob.WebhookURL)
	assert.Equal(t, "secret", updatedJob.WebhookSecret)
	assert.Equal(t, `{"X-Test": "test"}`, updatedJob.WebhookHeaders)
}

// TestWebhookValidation tests validation of webhook URL
func TestWebhookValidation(t *testing.T) {
	// Set up test environment
	handlers, router, _, _, config := setupJobsTest(t)

	// Add job create route
	router.POST("/jobs/create", handlers.HandleCreateJob)

	// Create job form data with invalid webhook URL
	formData := url.Values{
		"name":              {"Invalid Webhook Job"},
		"config_ids[]":      {strconv.Itoa(int(config.ID))},
		"schedule":          {"*/15 * * * *"},
		"enabled":           {"true"},
		"webhook_enabled":   {"true"},
		"webhook_url":       {"invalid-url"}, // Invalid URL
		"webhook_secret":    {"test-secret"},
		"notify_on_success": {"true"},
		"notify_on_failure": {"true"},
	}

	// Submit form
	req, _ := http.NewRequest("POST", "/jobs/create", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should not create job with invalid webhook URL
	assert.NotEqual(t, http.StatusFound, resp.Code)
	assert.Contains(t, resp.Body.String(), "valid URL")

	// Test invalid headers JSON
	formData = url.Values{
		"name":              {"Invalid Headers Job"},
		"config_ids[]":      {strconv.Itoa(int(config.ID))},
		"schedule":          {"*/15 * * * *"},
		"enabled":           {"true"},
		"webhook_enabled":   {"true"},
		"webhook_url":       {"https://example.com/webhook"},
		"webhook_secret":    {"test-secret"},
		"webhook_headers":   {`{"invalid json`}, // Invalid JSON
		"notify_on_success": {"true"},
		"notify_on_failure": {"true"},
	}

	// Submit form
	req, _ = http.NewRequest("POST", "/jobs/create", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should not create job with invalid headers JSON
	assert.NotEqual(t, http.StatusFound, resp.Code)
	assert.Contains(t, resp.Body.String(), "valid JSON")
}

func BoolPtr(b bool) *bool {
	return &b
}
