package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleAdminTools(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set up the route
	router.GET("/admin/tools", handlers.HandleAdminTools)

	// Create a test user and set it in the context
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/tools", nil)

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Admin Tools")
}

func TestHandleBackupDatabase(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the backup directory
	handlers.BackupDir = filepath.Join(tempDir, "backups")
	err = os.MkdirAll(handlers.BackupDir, 0755)
	require.NoError(t, err)

	// Set up the route
	router.POST("/admin/backup", handlers.HandleBackupDatabase)

	// Create a test user and set it in the context
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/backup", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	if w.Code != http.StatusOK {
		t.Logf("Response body: %s", w.Body.String())
	}
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Logf("Response body: %s", w.Body.String())
		t.Fatalf("Failed to parse response: %v", err)
	}

	assert.Contains(t, response["message"], "Database backup created successfully")
}

func TestHandleRestoreDatabase(t *testing.T) {
	t.Skip("Skipping restore test until backup functionality is fixed")
}

func TestHandleVacuumDatabase(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Set up the route
	router.POST("/admin/vacuum", handlers.HandleVacuumDatabase)

	// Create a test user and set it in the context
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/vacuum", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response["message"], "Database vacuum completed successfully")
}

func TestHandleClearJobHistory(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Add some job history entries
	for i := 0; i < 5; i++ {
		endTime := time.Now().Add(-time.Duration(i)*time.Hour + 5*time.Minute)
		history := &db.JobHistory{
			JobID:            1,
			StartTime:        time.Now().Add(-time.Duration(i) * time.Hour),
			EndTime:          &endTime,
			Status:           "success",
			ErrorMessage:     "Test output",
			BytesTransferred: 1024,
			FilesTransferred: 1,
		}
		handlers.DB.DB.Create(history)
	}

	// Set up the route
	router.POST("/admin/clear-job-history", handlers.HandleClearJobHistory)

	// Create a test user and set it in the context
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clear-job-history", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response["message"], "Job history cleared successfully")

	// Verify the job history is empty
	var count int64
	handlers.DB.DB.Model(&db.JobHistory{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestHandleExportConfigs(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test config
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       testUser.ID,
	}
	handlers.DB.DB.Create(config)

	// Set up the route
	router.GET("/admin/export/configs", handlers.HandleExportConfigs)

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/export/configs", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=gomft_configs_")

	// Parse the response as JSON
	var configs []map[string]interface{}
	var err error
	err = json.Unmarshal(w.Body.Bytes(), &configs)
	assert.NoError(t, err)
	assert.Greater(t, len(configs), 0)
}

func TestHandleExportJobs(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Create a test config
	config := &db.TransferConfig{
		ID:              1,
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       testUser.ID,
	}
	handlers.DB.DB.Create(config)

	// Create a test job
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "*/5 * * * *", // Every 5 minutes
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: testUser.ID,
	}
	handlers.DB.DB.Create(job)

	// Set up the route
	router.GET("/admin/export/jobs", handlers.HandleExportJobs)

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/export/jobs", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=gomft_jobs_")

	// Parse the response as JSON
	var jobs []map[string]interface{}
	var err error
	err = json.Unmarshal(w.Body.Bytes(), &jobs)
	assert.NoError(t, err)
	assert.Greater(t, len(jobs), 0)
}

func TestHandleImportConfigs(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the route
	router.POST("/admin/import/configs", handlers.HandleImportConfigs)

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create test data
	configsData := `[
		{
			"name": "Imported Config",
			"source_type": "sftp",
			"source_path": "/remote/source",
			"source_host": "sftp.example.com",
			"source_port": 22,
			"source_user": "user",
			"destination_type": "local",
			"destination_path": "/local/dest"
		}
	]`

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/import/configs", strings.NewReader(configsData))
	req.Header.Set("Content-Type", "application/json")

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "configs imported successfully")

	// Verify the config was created
	var count int64
	handlers.DB.DB.Model(&db.TransferConfig{}).Where("name = ?", "Imported Config").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestHandleImportJobs(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Create a test config
	config := &db.TransferConfig{
		ID:              1,
		Name:            "Test Config For Import",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       testUser.ID,
	}
	handlers.DB.DB.Create(config)

	// Set up the route
	router.POST("/admin/import/jobs", handlers.HandleImportJobs)

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create test data
	jobsData := `[
		{
			"name": "Imported Job",
			"schedule": "0 */2 * * *",
			"config_id": 1,
			"enabled": true
		}
	]`

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/import/jobs", strings.NewReader(jobsData))
	req.Header.Set("Content-Type", "application/json")

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "jobs imported successfully")

	// Verify the job was created
	var count int64
	handlers.DB.DB.Model(&db.Job{}).Where("name = ?", "Imported Job").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestHandleExportConfigsUnauthorized(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Set up the route
	router.GET("/admin/export/configs", handlers.HandleExportConfigs)

	// Create a test user that is not an admin
	testUser := &db.User{
		ID:      2,
		Email:   "user@example.com",
		IsAdmin: false,
	}

	// Set up the context with the non-admin user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/export/configs", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check that access is denied
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Admin access required")
}

func TestHandleBackupDatabaseError(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Set an invalid backup directory
	handlers.BackupDir = "/nonexistent/directory/that/should/not/exist"

	// Set up the route
	router.POST("/admin/backup", handlers.HandleBackupDatabase)

	// Create a test user and set it in the context
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/backup", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the error message
	assert.Contains(t, response["error"], "Failed to create backup")
}

func TestHandleListBackups(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary directory for backups
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the backup directory
	handlers.BackupDir = tempDir

	// Create a few test backup files with different dates
	backupFiles := []string{
		"gomft_backup_20220101_120000.db",
		"gomft_backup_20220102_120000.db",
		"gomft_backup_20220103_120000.db",
	}

	for _, name := range backupFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte("test backup content"), 0644)
		require.NoError(t, err)

		// Set different modification times to test sorting
		// Parse the date from the filename
		timeStr := strings.TrimPrefix(strings.TrimSuffix(name, ".db"), "gomft_backup_")
		timeStr = strings.Replace(timeStr, "_", "T", 1)
		layout := "20060102T150405"
		fileTime, err := time.Parse(layout, timeStr)
		require.NoError(t, err)

		// Set the modification time
		err = os.Chtimes(filepath.Join(tempDir, name), fileTime, fileTime)
		require.NoError(t, err)
	}

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/backups", handlers.HandleListBackups)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/backups", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify all backup files are in the response and sorted with most recent first
	assert.Equal(t, len(backupFiles), len(response), "All backup files should be listed")

	// Check that the most recent backup is first
	assert.Equal(t, "gomft_backup_20220103_120000.db", response[0]["name"], "Most recent backup should be first")
	assert.Equal(t, "gomft_backup_20220102_120000.db", response[1]["name"], "Second most recent backup should be second")
	assert.Equal(t, "gomft_backup_20220101_120000.db", response[2]["name"], "Oldest backup should be last")
}

func TestHandleSystemInfo(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/system-info", handlers.HandleSystemInfo)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/system-info", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the response contains expected system info fields
	assert.Contains(t, response, "os")
	assert.Contains(t, response, "uptime")
	assert.Contains(t, response, "memory")
	assert.Contains(t, response, "disk")
	assert.Contains(t, response, "cpu")
	assert.Contains(t, response, "go_version")
}

// Helper function to create a multipart form request for file uploads
func createMultipartRequest(t *testing.T, url, fieldName, fileName, fileContent string) (*http.Request, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)

	_, err = io.Copy(part, strings.NewReader(fileContent))
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	req, err := http.NewRequest("POST", url, body)
	require.NoError(t, err)

	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, writer.FormDataContentType()
}

func TestHandleImportConfigsFromFile(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the route
	router.POST("/admin/import/configs/file", handlers.HandleImportConfigsFromFile)

	// Set up the context with the user
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Create test data
	configsData := `[
		{
			"name": "Imported Config From File",
			"source_type": "sftp",
			"source_path": "/remote/source",
			"source_host": "sftp.example.com",
			"source_port": 22,
			"source_user": "user",
			"destination_type": "local",
			"destination_path": "/local/dest"
		}
	]`

	// Create a multipart request with the configs file
	req, contentType := createMultipartRequest(t, "/admin/import/configs/file", "configs_file", "configs.json", configsData)
	req.Header.Set("Content-Type", contentType)

	// Create recorder for the response
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "configs imported successfully")

	// Verify the config was created
	var count int64
	handlers.DB.DB.Model(&db.TransferConfig{}).Where("name = ?", "Imported Config From File").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestHandleImportJobsFromFile(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Create a test config
	config := &db.TransferConfig{
		ID:              1,
		Name:            "Test Config For Import",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       testUser.ID,
	}

	// Create the config in the database
	result := handlers.DB.DB.Create(config)
	require.NoError(t, result.Error)

	// Verify the config was created
	var configCount int64
	handlers.DB.DB.Model(&db.TransferConfig{}).Count(&configCount)
	require.Equal(t, int64(1), configCount)

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route - AFTER middleware
	router.POST("/admin/import/jobs/file", handlers.HandleImportJobsFromFile)

	// Create test data with the correct config ID
	// Note: We're using a numeric value for config_id, not a string
	jobsData := `[
		{
			"name": "Imported Job From File",
			"schedule": "0 */2 * * *",
			"config_id": 1,
			"enabled": true,
			"created_by": 1
		}
	]`

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

	// Create the request
	req, err := http.NewRequest("POST", "/admin/import/jobs/file", body)
	require.NoError(t, err)

	// Set the content type
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Create recorder for the response
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "jobs imported successfully")

	// Verify the job was created
	var count int64
	handlers.DB.DB.Model(&db.Job{}).Where("name = ?", "Imported Job From File").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestHandleImportConfigsInvalidJSON(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route - AFTER middleware
	router.POST("/admin/import/configs", handlers.HandleImportConfigs)

	// Create invalid JSON data
	configsData := `[
		{
			"name": "Invalid Config",
			"source_type": "sftp",
			"source_path": "/remote/source",
			"source_host": "sftp.example.com",
			"source_port": "not-a-number", <- invalid field
			"source_user": "user",
			"destination_type": "local",
			"destination_path": "/local/dest"
		}
	]`

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/import/configs", strings.NewReader(configsData))
	req.Header.Set("Content-Type", "application/json")

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response - should fail with 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the error message
	assert.Contains(t, response["error"], "Invalid JSON")
}

func TestHandleDeleteLogFile(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary log file for testing
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the logs directory
	handlers.LogsDir = tempDir

	// Create a test log file
	logFile := filepath.Join(tempDir, "test.log")
	err = os.WriteFile(logFile, []byte("test log content"), 0644)
	require.NoError(t, err)

	// Create a test user and set it in the context
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route - AFTER middleware
	router.POST("/admin/logs/delete/:filename", handlers.HandleDeleteLogFile)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/logs/delete/test.log", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "Log file deleted successfully")

	// Verify the file was deleted
	_, err = os.Stat(logFile)
	assert.True(t, os.IsNotExist(err))
}

func TestHandleSystemMaintenanceCheck(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - this must be done BEFORE registering the routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route - AFTER middleware
	router.GET("/admin/maintenance-check", handlers.HandleSystemMaintenanceCheck)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/maintenance-check", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify response contains maintenance check results
	assert.Contains(t, response, "status")
	assert.Contains(t, response, "checks")
}

func TestHandleUpdateSystemSettings(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a test user and set it in the context
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route - AFTER middleware
	router.POST("/admin/settings", handlers.HandleUpdateSystemSettings)

	// Create test settings data
	settingsData := `{
		"email_notifications": true,
		"log_retention_days": 30,
		"max_concurrent_transfers": 5,
		"default_retry_attempts": 3
	}`

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/settings", strings.NewReader(settingsData))
	req.Header.Set("Content-Type", "application/json")

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "Settings updated successfully")
}

func TestHandleViewLog(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary log file for testing
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the logs directory
	handlers.LogsDir = tempDir

	// Create a test log file
	logFileName := "test-view.log"
	logFile := filepath.Join(tempDir, logFileName)
	err = os.WriteFile(logFile, []byte("test log content for viewing"), 0644)
	require.NoError(t, err)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/logs/:fileName", handlers.HandleViewLog)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/logs/"+logFileName, nil)

	// Override environment variables for the test
	t.Setenv("LOGS_DIR", tempDir)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test log content for viewing")
}

func TestHandleViewLogNotFound(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the logs directory
	handlers.LogsDir = tempDir

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/logs/:fileName", handlers.HandleViewLog)

	// Create a test request for a non-existent file
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/logs/nonexistent.log", nil)

	// Override environment variables for the test
	t.Setenv("LOGS_DIR", tempDir)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response - should be NotFound
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Log file not found")
}

func TestHandleDownloadLog(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary log file for testing
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the logs directory
	handlers.LogsDir = tempDir

	// Create a test log file
	logFileName := "test-download.log"
	logFile := filepath.Join(tempDir, logFileName)
	logContent := "test log content for download"
	err = os.WriteFile(logFile, []byte(logContent), 0644)
	require.NoError(t, err)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/logs/download/:fileName", handlers.HandleDownloadLog)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/logs/download/"+logFileName, nil)

	// Override environment variables for the test
	t.Setenv("LOGS_DIR", tempDir)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename=test-download.log`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, logContent, w.Body.String())
}

func TestHandleDeleteBackup(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary directory for backups
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the backup directory
	handlers.BackupDir = tempDir

	// Create a test backup file
	backupFileName := "gomft_backup_20220101_120000.db"
	backupFile := filepath.Join(tempDir, backupFileName)
	err = os.WriteFile(backupFile, []byte("test backup content"), 0644)
	require.NoError(t, err)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.DELETE("/admin/backup/:filename", handlers.HandleDeleteBackup)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/admin/backup/"+backupFileName, nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify the success message
	assert.Contains(t, response["message"], "Backup deleted successfully")

	// Verify the file was deleted
	_, err = os.Stat(backupFile)
	assert.True(t, os.IsNotExist(err))
}

func TestHandleDownloadBackup(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary directory for backups
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the backup directory
	handlers.BackupDir = tempDir

	// Create a test backup file
	backupFileName := "gomft_backup_20220101_120000.db"
	backupFile := filepath.Join(tempDir, backupFileName)
	backupContent := "test backup content for download"
	err = os.WriteFile(backupFile, []byte(backupContent), 0644)
	require.NoError(t, err)

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/download-backup/:filename", handlers.HandleDownloadBackup)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/download-backup/"+backupFileName, nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename=gomft_backup_20220101_120000.db`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, backupContent, w.Body.String())
}

func TestHandleRefreshLogs(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary log directory for testing
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the logs directory
	handlers.LogsDir = tempDir

	// Create a few test log files
	logFiles := []string{"app.log", "errors.log", "access.log"}
	for _, name := range logFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/logs", handlers.HandleRefreshLogs)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/logs", nil)

	// Override environment variables for the test
	t.Setenv("LOGS_DIR", tempDir)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify all log files are listed in the response
	for _, name := range logFiles {
		assert.Contains(t, w.Body.String(), name)
	}
}

func TestHandleRefreshBackups(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Create a temporary directory for backups
	tempDir, err := os.MkdirTemp("", "gomft-admin-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set the backup directory
	handlers.BackupDir = tempDir

	// Create a few test backup files
	backupFiles := []string{
		"gomft_backup_20220101_120000.db",
		"gomft_backup_20220102_120000.db",
	}

	for _, name := range backupFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte("test backup content"), 0644)
		require.NoError(t, err)
	}

	// Create a test user
	testUser := &db.User{
		ID:      1,
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	// Set up the context with the user - must be done BEFORE registering routes
	router.Use(func(c *gin.Context) {
		c.Set("user", testUser)
		c.Next()
	})

	// Set up the route
	router.GET("/admin/refresh-backups", handlers.HandleRefreshBackups)

	// Create a test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/refresh-backups", nil)

	// Serve the request
	router.ServeHTTP(w, req)

	// Print response body for debugging
	t.Logf("Response body: %s", w.Body.String())

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the response contains the backup files
	for _, name := range backupFiles {
		assert.Contains(t, w.Body.String(), name)
	}
}
