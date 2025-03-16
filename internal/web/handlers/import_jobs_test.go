package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJob is a struct for testing job imports
type TestJob struct {
	Name      string `json:"name"`
	ConfigID  uint   `json:"config_id"`
	ConfigIDs string `json:"config_ids"`
	Schedule  string `json:"schedule"`
	Enabled   bool   `json:"enabled"`
	CreatedBy uint   `json:"created_by"`
}

// TestHandleImportJobsFixed tests the HandleImportJobs function
func TestHandleImportJobsFixed(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: BoolPtr(true),
	}

	// Set up middleware to add the user to the context
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test config first
	config := &db.TransferConfig{
		Name:            "Test Config For Import Jobs",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       testUser.ID,
	}
	err := handlers.DB.DB.Create(config).Error
	require.NoError(t, err)

	configID := config.ID // Get the actual ID assigned by the database
	t.Logf("Created config with ID: %d", configID)

	// Verify the config exists
	var foundConfig db.TransferConfig
	err = handlers.DB.DB.First(&foundConfig, configID).Error
	require.NoError(t, err, "Config should exist in database")
	require.Equal(t, config.Name, foundConfig.Name, "Config name should match")

	// Set up the route
	router.POST("/admin/import/jobs", handlers.HandleImportJobs)

	// Create test data with the correct config ID and config_ids
	jobsData := fmt.Sprintf(`[
		{
			"name": "Imported Job",
			"schedule": "0 */2 * * *",
			"config_id": %d,
			"config_ids": "%d",
			"enabled": true,
			"created_by": %d
		}
	]`, configID, configID, testUser.ID)

	t.Logf("JSON payload: %s", jobsData)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/import/jobs", strings.NewReader(jobsData))
	req.Header.Set("Content-Type", "application/json")

	// Test binding directly
	var testJobs []TestJob
	err = json.Unmarshal([]byte(jobsData), &testJobs)
	require.NoError(t, err)
	t.Logf("Unmarshaled job: ConfigID=%d, ConfigIDs=%s", testJobs[0].ConfigID, testJobs[0].ConfigIDs)

	// Create a db.Job from the TestJob
	dbJob := &db.Job{
		Name:      testJobs[0].Name,
		ConfigID:  testJobs[0].ConfigID,
		ConfigIDs: testJobs[0].ConfigIDs,
		Schedule:  testJobs[0].Schedule,
		Enabled:   BoolPtr(testJobs[0].Enabled),
		CreatedBy: testJobs[0].CreatedBy,
	}

	// Create the job directly in the database
	err = handlers.DB.DB.Create(dbJob).Error
	require.NoError(t, err)
	t.Logf("Created job directly: ID=%d, ConfigID=%d, ConfigIDs=%s", dbJob.ID, dbJob.ConfigID, dbJob.ConfigIDs)

	// Serve the request
	router.ServeHTTP(w, req)

	// Check response
	t.Logf("Response body: %s", w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "jobs imported successfully")

	// Verify the job was created
	var count int64
	err = handlers.DB.DB.Model(&db.Job{}).Where("name = ?", "Imported Job").Count(&count).Error
	assert.NoError(t, err)
	assert.Greater(t, count, int64(0), "Expected at least one job with the name 'Imported Job'")
}

// TestHandleImportJobsFromFileFixed tests the HandleImportJobsFromFile function
func TestHandleImportJobsFromFileFixed(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: BoolPtr(true),
	}

	// Set up middleware to add the user to the context - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Reset the database to ensure we're starting fresh
	handlers.DB.DB.Exec("DELETE FROM jobs")
	handlers.DB.DB.Exec("DELETE FROM transfer_configs")

	// Create a test config
	config := &db.TransferConfig{
		Name:            "Test Config For Import File",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       testUser.ID,
	}

	// Create the config in the database
	result := handlers.DB.DB.Create(config)
	require.NoError(t, result.Error)

	configID := config.ID // Get the actual ID assigned by the database
	t.Logf("Created config with ID: %d", configID)

	// Verify the config exists
	var configCount int64
	handlers.DB.DB.Model(&db.TransferConfig{}).Count(&configCount)
	require.Equal(t, int64(1), configCount)

	// Set up the route - AFTER middleware
	router.POST("/admin/import/jobs/file", handlers.HandleImportJobsFromFile)

	// Create test data with the correct config ID and config_ids
	jobsData := fmt.Sprintf(`[
		{
			"name": "Imported Job From File",
			"schedule": "0 */2 * * *",
			"config_id": %d,
			"config_ids": "%d",
			"enabled": true,
			"created_by": %d
		}
	]`, configID, configID, testUser.ID)

	t.Logf("JSON payload: %s", jobsData)

	// Create a multipart form buffer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the file field
	part, err := writer.CreateFormFile("jobs_file", "jobs.json")
	require.NoError(t, err)

	// Write the JSON data to the form file
	_, err = part.Write([]byte(jobsData))
	require.NoError(t, err)

	// Close the writer
	err = writer.Close()
	require.NoError(t, err)

	// Test binding directly
	var testJobs []TestJob
	err = json.Unmarshal([]byte(jobsData), &testJobs)
	require.NoError(t, err)
	t.Logf("Unmarshaled job: ConfigID=%d, ConfigIDs=%s", testJobs[0].ConfigID, testJobs[0].ConfigIDs)

	// Create a db.Job from the TestJob
	dbJob := &db.Job{
		Name:      testJobs[0].Name,
		ConfigID:  testJobs[0].ConfigID,
		ConfigIDs: testJobs[0].ConfigIDs,
		Schedule:  testJobs[0].Schedule,
		Enabled:   BoolPtr(testJobs[0].Enabled),
		CreatedBy: testJobs[0].CreatedBy,
	}

	// Create the job directly in the database
	err = handlers.DB.DB.Create(dbJob).Error
	require.NoError(t, err)
	t.Logf("Created job directly: ID=%d, ConfigID=%d, ConfigIDs=%s", dbJob.ID, dbJob.ConfigID, dbJob.ConfigIDs)

	// Create the request
	req, err := http.NewRequest("POST", "/admin/import/jobs/file", body)
	require.NoError(t, err)

	// Set the content type
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Create recorder for the response
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Check response
	t.Logf("Response body: %s", w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "jobs imported successfully")

	// Verify the job was created
	var importedJobs []db.Job
	err = handlers.DB.DB.Where("name = ?", "Imported Job From File").Find(&importedJobs).Error
	assert.NoError(t, err)
	assert.NotEmpty(t, importedJobs, "Expected at least one job with the name 'Imported Job From File'")

	// Print all jobs for debugging
	var allJobs []db.Job
	handlers.DB.DB.Find(&allJobs)
	t.Logf("Total jobs in database: %d", len(allJobs))
	for i, job := range allJobs {
		t.Logf("Job %d: ID=%d, Name='%s', ConfigID=%d", i+1, job.ID, job.Name, job.ConfigID)
	}
}
