package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
)

func setupFileMetadataHandlers(t *testing.T) (*FileMetadataHandler, *gin.Engine, *db.User, *db.Job) {
	// Get base handlers and router from the shared setup
	handlers, router := setupTestHandlers(t)

	// Create a test user with a unique email
	testUser := &db.User{
		Email:              fmt.Sprintf("file-meta-test-%d@example.com", time.Now().UnixNano()),
		PasswordHash:       "hashed_password",
		LastPasswordChange: time.Now(),
	}
	err := handlers.DB.CreateUser(testUser)
	assert.NoError(t, err)

	// Create a test config
	testConfig := &db.TransferConfig{
		Name:            "Test Config for File Metadata",
		SourceType:      "local",
		SourcePath:      "/source/path",
		DestinationType: "local",
		DestinationPath: "/destination/path",
		CreatedBy:       testUser.ID,
	}
	err = handlers.DB.CreateTransferConfig(testConfig)
	assert.NoError(t, err)

	// Create a test job
	testJob := &db.Job{
		Name:      "Test Job for File Metadata",
		ConfigID:  testConfig.ID,
		Schedule:  "0 * * * *", // Run hourly
		Enabled:   true,
		CreatedBy: testUser.ID,
	}
	err = handlers.DB.CreateJob(testJob)
	assert.NoError(t, err)

	// Create test file metadata entries
	for i := 0; i < 5; i++ {
		fileMetadata := &db.FileMetadata{
			JobID:           testJob.ID,
			FileName:        fmt.Sprintf("testfile%d.txt", i),
			OriginalPath:    fmt.Sprintf("/source/path/testfile%d.txt", i),
			FileSize:        int64(1024 * (i + 1)),
			FileHash:        fmt.Sprintf("hash%d", i),
			CreationTime:    time.Now().Add(-24 * time.Hour),
			ModTime:         time.Now().Add(-12 * time.Hour),
			ProcessedTime:   time.Now(),
			DestinationPath: fmt.Sprintf("/destination/path/testfile%d.txt", i),
			Status:          "processed",
		}
		err = handlers.DB.CreateFileMetadata(fileMetadata)
		assert.NoError(t, err)
	}

	// Create middleware to simulate authenticated user
	router.Use(func(c *gin.Context) {
		c.Set("userID", testUser.ID)
		c.Next()
	})

	// Create the FileMetadataHandler that we'll test
	fileMetadataHandler := &FileMetadataHandler{
		DB: handlers.DB,
	}

	return fileMetadataHandler, router, testUser, testJob
}

// Helper function to set HTMX headers on request
func setHTMXHeaders(req *http.Request) {
	req.Header.Set("HX-Request", "true")
}

func TestListFileMetadata(t *testing.T) {
	// Setup
	handler, router, testUser, testJob := setupFileMetadataHandlers(t)

	// Ensure job is owned by test user
	testJob.CreatedBy = testUser.ID
	handler.DB.DB.Save(testJob)

	// Recreate file metadata entries to ensure they're properly linked to the updated job
	handler.DB.DB.Unscoped().Where("job_id = ?", testJob.ID).Delete(&db.FileMetadata{})

	// Create new test file metadata entries for the job
	var fileIDs []uint
	for i := 0; i < 5; i++ {
		fileMetadata := &db.FileMetadata{
			JobID:           testJob.ID,
			FileName:        fmt.Sprintf("testfile%d.txt", i),
			OriginalPath:    fmt.Sprintf("/source/path/testfile%d.txt", i),
			FileSize:        int64(1024 * (i + 1)),
			FileHash:        fmt.Sprintf("hash%d", i),
			CreationTime:    time.Now().Add(-24 * time.Hour),
			ModTime:         time.Now().Add(-12 * time.Hour),
			ProcessedTime:   time.Now(),
			DestinationPath: fmt.Sprintf("/destination/path/testfile%d.txt", i),
			Status:          "processed",
		}
		err := handler.DB.CreateFileMetadata(fileMetadata)
		assert.NoError(t, err)
		fileIDs = append(fileIDs, fileMetadata.ID)
	}

	// Setup route
	router.GET("/files", handler.ListFileMetadata)

	// Test default pagination (page 1, limit 50)
	req, _ := http.NewRequest(http.MethodGet, "/files", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response status code
	assert.Equal(t, http.StatusOK, resp.Code)

	// Verify that the database contains the expected records
	var count int64
	handler.DB.DB.Model(&db.FileMetadata{}).Where("job_id = ?", testJob.ID).Count(&count)
	assert.Equal(t, int64(5), count)

	// Test with pagination params
	req, _ = http.NewRequest(http.MethodGet, "/files?page=1&limit=2", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response status code
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test with status filter
	req, _ = http.NewRequest(http.MethodGet, "/files?status=processed", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response status code
	assert.Equal(t, http.StatusOK, resp.Code)

	// Verify that the database contains the expected records with the status filter
	handler.DB.DB.Model(&db.FileMetadata{}).Where("job_id = ? AND status = ?", testJob.ID, "processed").Count(&count)
	assert.Equal(t, int64(5), count)
}

func TestGetFileMetadataDetails(t *testing.T) {
	// Setup
	handler, router, testUser, _ := setupFileMetadataHandlers(t)

	// Setup route
	router.GET("/files/:id", handler.GetFileMetadataDetails)

	// Get first file metadata ID
	var firstMetadata db.FileMetadata
	result := handler.DB.DB.First(&firstMetadata)
	assert.NoError(t, result.Error)

	// Update the job to make sure the test user owns it
	var job db.Job
	handler.DB.DB.First(&job, firstMetadata.JobID)
	job.CreatedBy = testUser.ID
	handler.DB.DB.Save(&job)

	// Test getting details for valid ID
	req, _ := http.NewRequest(http.MethodGet, "/files/"+strconv.Itoa(int(firstMetadata.ID)), nil)
	setHTMXHeaders(req) // Add HTMX header
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), firstMetadata.FileName)
	assert.Contains(t, resp.Body.String(), firstMetadata.Status)

	// Test getting details for invalid ID
	req, _ = http.NewRequest(http.MethodGet, "/files/999999", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestDeleteFileMetadata(t *testing.T) {
	// Setup
	handler, router, testUser, _ := setupFileMetadataHandlers(t)

	// Setup route
	router.DELETE("/files/:id", handler.DeleteFileMetadata)

	// Get first file metadata ID
	var firstMetadata db.FileMetadata
	result := handler.DB.DB.First(&firstMetadata)
	assert.NoError(t, result.Error)

	// Update the job to make sure the test user owns it
	var job db.Job
	handler.DB.DB.First(&job, firstMetadata.JobID)
	job.CreatedBy = testUser.ID
	handler.DB.DB.Save(&job)

	// Test deleting with valid ID
	req, _ := http.NewRequest(http.MethodDelete, "/files/"+strconv.Itoa(int(firstMetadata.ID)), nil)
	setHTMXHeaders(req) // Add HTMX header
	resp := httptest.NewRecorder()
	fmt.Println("Deleting file metadata")
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	// Verify deletion
	var deletedMetadata db.FileMetadata
	result = handler.DB.DB.First(&deletedMetadata, firstMetadata.ID)
	assert.Error(t, result.Error) // Should not find the deleted record

	// Test deleting with invalid ID
	req, _ = http.NewRequest(http.MethodDelete, "/files/999999", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp = httptest.NewRecorder()
	fmt.Println("Deleting file metadata")
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestGetFileMetadataForJob(t *testing.T) {
	// Setup
	handler, router, testUser, testJob := setupFileMetadataHandlers(t)

	// Ensure job is owned by test user
	testJob.CreatedBy = testUser.ID
	handler.DB.DB.Save(testJob)

	// Setup route
	router.GET("/files/job/:job_id", handler.GetFileMetadataForJob)

	// Test getting files for valid job ID
	req, _ := http.NewRequest(http.MethodGet, "/files/job/"+strconv.Itoa(int(testJob.ID)), nil)
	setHTMXHeaders(req) // Add HTMX header
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "testfile0.txt")

	// Test getting files for invalid job ID
	req, _ = http.NewRequest(http.MethodGet, "/files/job/999999", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusNotFound, resp.Code) // Not found for invalid job ID
}

func TestSearchFileMetadata(t *testing.T) {
	// Setup
	handler, router, _, _ := setupFileMetadataHandlers(t)

	// Setup route
	router.GET("/files/search", handler.SearchFileMetadata)

	// Test search by filename
	req, _ := http.NewRequest(http.MethodGet, "/files/search?filename=testfile", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "testfile0.txt")
	assert.Contains(t, resp.Body.String(), "testfile4.txt")

	// Test search by specific filename
	req, _ = http.NewRequest(http.MethodGet, "/files/search?filename=testfile1", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "testfile1.txt")
	assert.NotContains(t, resp.Body.String(), "testfile2.txt")

	// Test search with no results
	req, _ = http.NewRequest(http.MethodGet, "/files/search?filename=nonexistent", nil)
	setHTMXHeaders(req) // Add HTMX header
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), "testfile")
}

func TestHandleFileMetadataPartial(t *testing.T) {
	// Setup
	handler, router, testUser, testJob := setupFileMetadataHandlers(t)

	// Ensure job is owned by test user
	testJob.CreatedBy = testUser.ID
	handler.DB.DB.Save(testJob)

	// Recreate file metadata entries to ensure they're properly linked to the updated job
	handler.DB.DB.Unscoped().Where("job_id = ?", testJob.ID).Delete(&db.FileMetadata{})

	// Create new test file metadata entries for the job
	var fileIDs []uint
	for i := 0; i < 5; i++ {
		fileMetadata := &db.FileMetadata{
			JobID:           testJob.ID,
			FileName:        fmt.Sprintf("testfile%d.txt", i),
			OriginalPath:    fmt.Sprintf("/source/path/testfile%d.txt", i),
			FileSize:        int64(1024 * (i + 1)),
			FileHash:        fmt.Sprintf("hash%d", i),
			CreationTime:    time.Now().Add(-24 * time.Hour),
			ModTime:         time.Now().Add(-12 * time.Hour),
			ProcessedTime:   time.Now(),
			DestinationPath: fmt.Sprintf("/destination/path/testfile%d.txt", i),
			Status:          "processed",
		}
		err := handler.DB.CreateFileMetadata(fileMetadata)
		assert.NoError(t, err)
		fileIDs = append(fileIDs, fileMetadata.ID)
	}

	// Setup route
	router.GET("/files/partial", handler.HandleFileMetadataPartial)

	// Test partial loading of file metadata (with HTMX header)
	req, _ := http.NewRequest(http.MethodGet, "/files/partial?page=1&limit=2", nil)
	setHTMXHeaders(req) // Add HTMX headers
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response status code
	assert.Equal(t, http.StatusOK, resp.Code)

	// Verify that the database contains the expected records
	var count int64
	handler.DB.DB.Model(&db.FileMetadata{}).Where("job_id = ?", testJob.ID).Count(&count)
	assert.Equal(t, int64(5), count)

	// Test with different page (with HTMX header)
	req, _ = http.NewRequest(http.MethodGet, "/files/partial?page=2&limit=2", nil)
	setHTMXHeaders(req) // Add HTMX headers
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response status code
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandleFileMetadataSearchPartial(t *testing.T) {
	// Setup
	handler, router, _, _ := setupFileMetadataHandlers(t)

	// Setup route
	router.GET("/files/search/partial", handler.HandleFileMetadataSearchPartial)

	// Test partial search results (with HTMX header)
	req, _ := http.NewRequest(http.MethodGet, "/files/search/partial?filename=testfile&page=1&limit=2", nil)
	setHTMXHeaders(req) // Add HTMX headers
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	responseBody := resp.Body.String()

	// Verify the response contains test files
	assert.Contains(t, responseBody, "testfile")

	// Test search with no results (with HTMX header)
	req, _ = http.NewRequest(http.MethodGet, "/files/search/partial?filename=nonexistent", nil)
	setHTMXHeaders(req) // Add HTMX headers
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), "testfile")
}
