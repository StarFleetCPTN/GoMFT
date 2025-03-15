package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func setupDashboardTest(t *testing.T) (*Handlers, *gin.Engine, *db.DB) {
	// Set up test database
	database := testutils.SetupTestDB(t)

	// Create test user
	user := testutils.CreateTestUser(t, database, "test@example.com", false)

	// Create test config
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

	// Create test job
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "*/5 * * * *",
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	if err := database.DB.Create(job).Error; err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create test job history entries
	now := time.Now()

	// Completed job
	completedJob := &db.JobHistory{
		JobID:            job.ID,
		StartTime:        now.Add(-time.Hour),
		EndTime:          &now,
		Status:           "completed",
		BytesTransferred: 1024,
		FilesTransferred: 1,
	}
	if err := database.DB.Create(completedJob).Error; err != nil {
		t.Fatalf("Failed to create completed job history: %v", err)
	}

	// Failed job
	failedJob := &db.JobHistory{
		JobID:        job.ID,
		StartTime:    now.Add(-2 * time.Hour),
		EndTime:      &now,
		Status:       "failed",
		ErrorMessage: "Test error",
	}
	if err := database.DB.Create(failedJob).Error; err != nil {
		t.Fatalf("Failed to create failed job history: %v", err)
	}

	// Running job
	runningJob := &db.JobHistory{
		JobID:     job.ID,
		StartTime: now.Add(-30 * time.Minute),
		Status:    "running",
	}
	if err := database.DB.Create(runningJob).Error; err != nil {
		t.Fatalf("Failed to create running job history: %v", err)
	}

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handlers
	handlers := &Handlers{
		DB: database,
	}

	// Set up authentication middleware
	router.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("isAdmin", false)
		c.Next()
	})

	return handlers, router, database
}

func TestHandleDashboard(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/dashboard", handlers.HandleDashboard)

	// Create request
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Dashboard")
	assert.Contains(t, resp.Body.String(), "Recent Jobs")

	// Check that job statistics are included
	assert.Contains(t, resp.Body.String(), "Active Transfers")
	assert.Contains(t, resp.Body.String(), "Completed Today")
	assert.Contains(t, resp.Body.String(), "Failed Transfers")
}

func TestHandleHistory(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/history", handlers.HandleHistory)

	// Create request
	req, _ := http.NewRequest("GET", "/history", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Transfer History")

	// Check that job history is included
	assert.Contains(t, resp.Body.String(), "Test Config")
	assert.Contains(t, resp.Body.String(), "Completed")
	assert.Contains(t, resp.Body.String(), "Failed")
}

func TestHandleHistoryWithPagination(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/history", handlers.HandleHistory)

	testCases := []struct {
		name            string
		url             string
		expectedStatus  int
		expectedContent string
	}{
		{
			name:            "Default pagination",
			url:             "/history",
			expectedStatus:  http.StatusOK,
			expectedContent: "Test Config",
		},
		{
			name:            "Custom page size",
			url:             "/history?pageSize=25",
			expectedStatus:  http.StatusOK,
			expectedContent: "Test Config",
		},
		{
			name:            "Invalid page size defaults to 10",
			url:             "/history?pageSize=invalid",
			expectedStatus:  http.StatusOK,
			expectedContent: "Test Config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tc.url, nil)
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedStatus, resp.Code)
			assert.Contains(t, resp.Body.String(), tc.expectedContent)
		})
	}
}

func TestHandleHistoryWithSearch(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/history", handlers.HandleHistory)

	// Test search
	req, _ := http.NewRequest("GET", "/history?search=completed", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "completed")
	assert.NotContains(t, resp.Body.String(), "failed") // Should filter out failed jobs
}

func TestHandleHistoryWithHtmx(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/history", handlers.HandleHistory)

	// Test HTMX request
	req, _ := http.NewRequest("GET", "/history", nil)
	req.Header.Set("HX-Request", "true")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	// Should only contain the history content, not the full page
	assert.Contains(t, resp.Body.String(), "Test Config")
	assert.NotContains(t, resp.Body.String(), "<html")
}

func TestHandleDashboardData(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/dashboard/data", handlers.HandleDashboardData)

	// Create request
	req, _ := http.NewRequest("GET", "/dashboard/data", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "recent_runs")
}

func TestHandleDashboardJobsData(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/dashboard/jobs", handlers.HandleDashboardJobsData)

	// Create request
	req, _ := http.NewRequest("GET", "/dashboard/jobs", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "active_jobs")
}

func TestHandleDashboardHistoryData(t *testing.T) {
	handlers, router, _ := setupDashboardTest(t)

	// Set up route
	router.GET("/dashboard/history", handlers.HandleDashboardHistoryData)

	// Create request
	req, _ := http.NewRequest("GET", "/dashboard/history", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "success_count")
	assert.Contains(t, resp.Body.String(), "failure_count")
	assert.Contains(t, resp.Body.String(), "pending_count")
}
