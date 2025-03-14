package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func setupConfigTest(t *testing.T) (*Handlers, *gin.Engine, *db.DB, *db.User) {
	// Set up test database
	database := testutils.SetupTestDB(t)

	// Create test user
	user := testutils.CreateTestUser(t, database, "test@example.com", false)

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

	return handlers, router, database, user
}

func createTestConfig(t *testing.T, database *db.DB, userID uint) *db.TransferConfig {
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       userID,
	}
	if err := database.Create(config).Error; err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	return config
}

func TestHandleConfigs(t *testing.T) {
	handlers, router, database, user := setupConfigTest(t)

	// Create test configs
	config1 := createTestConfig(t, database, user.ID)
	config2 := createTestConfig(t, database, user.ID)

	// Create a config for another user
	otherUser := testutils.CreateTestUser(t, database, "other@example.com", false)
	createTestConfig(t, database, otherUser.ID)

	// Set up route
	router.GET("/configs", handlers.HandleConfigs)

	// Create request
	req, _ := http.NewRequest("GET", "/configs", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	// Response should include user's configs
	assert.Contains(t, resp.Body.String(), config1.Name)
	assert.Contains(t, resp.Body.String(), config2.Name)

	// Should not contain configs from other users
	assert.Contains(t, resp.Body.String(), strconv.Itoa(int(config1.ID)))
	assert.Contains(t, resp.Body.String(), strconv.Itoa(int(config2.ID)))
	assert.NotContains(t, resp.Body.String(), "other@example.com")
}

func TestHandleNewConfig(t *testing.T) {
	handlers, router, _, _ := setupConfigTest(t)

	// Set up route
	router.GET("/configs/new", handlers.HandleNewConfig)

	// Create request
	req, _ := http.NewRequest("GET", "/configs/new", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "New Configuration")
	assert.Contains(t, resp.Body.String(), "Source Type")
	assert.Contains(t, resp.Body.String(), "Destination Type")
}

func TestHandleEditConfig(t *testing.T) {
	handlers, router, database, user := setupConfigTest(t)

	// Create test config
	config := createTestConfig(t, database, user.ID)

	// Create a config for another user
	otherUser := testutils.CreateTestUser(t, database, "other@example.com", false)
	otherConfig := createTestConfig(t, database, otherUser.ID)

	// Set up route
	router.GET("/configs/:id/edit", handlers.HandleEditConfig)

	// Test cases
	testCases := []struct {
		name         string
		configID     uint
		expectedCode int
		expectedBody string
	}{
		{
			name:         "Edit own config",
			configID:     config.ID,
			expectedCode: http.StatusOK,
			expectedBody: "Edit Configuration",
		},
		{
			name:         "Cannot edit other user's config",
			configID:     otherConfig.ID,
			expectedCode: http.StatusFound, // Redirect to /configs
			expectedBody: "",
		},
		{
			name:         "Non-existent config",
			configID:     9999,
			expectedCode: http.StatusFound, // Redirect to /configs
			expectedBody: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, _ := http.NewRequest("GET", "/configs/"+strconv.Itoa(int(tc.configID))+"/edit", nil)
			resp := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(resp, req)

			// Check response code
			assert.Equal(t, tc.expectedCode, resp.Code)

			if tc.expectedBody != "" {
				assert.Contains(t, resp.Body.String(), tc.expectedBody)
			}
		})
	}

	// Test admin access to other user's config
	adminRouter := gin.New()
	adminRouter.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("isAdmin", true) // Set as admin
		c.Next()
	})
	adminRouter.GET("/configs/:id/edit", handlers.HandleEditConfig)

	// Admin should be able to edit other user's config
	req, _ := http.NewRequest("GET", "/configs/"+strconv.Itoa(int(otherConfig.ID))+"/edit", nil)
	resp := httptest.NewRecorder()
	adminRouter.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Edit Configuration")
}

func TestHandleCreateConfig(t *testing.T) {
	handlers, router, database, user := setupConfigTest(t)

	// Set up route
	router.POST("/configs", handlers.HandleCreateConfig)

	// Prepare form data
	formData := url.Values{
		"name":             {"New Test Config"},
		"source_type":      {"local"},
		"source_path":      {"/test/source"},
		"destination_type": {"local"},
		"destination_path": {"/test/dest"},
		"file_pattern":     {"*.txt"},
	}

	// Create request
	req, _ := http.NewRequest("POST", "/configs", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response (should redirect on success)
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Equal(t, "/configs", resp.Header().Get("Location"))

	// Verify config was created in database
	var configs []db.TransferConfig
	database.Where("created_by = ?", user.ID).Find(&configs)

	assert.Equal(t, 1, len(configs))
	assert.Equal(t, "New Test Config", configs[0].Name)
	assert.Equal(t, "local", configs[0].SourceType)
	assert.Equal(t, "/test/source", configs[0].SourcePath)
	assert.Equal(t, "local", configs[0].DestinationType)
	assert.Equal(t, "/test/dest", configs[0].DestinationPath)
}

func TestHandleUpdateConfig(t *testing.T) {
	handlers, router, database, user := setupConfigTest(t)

	// Create test config
	config := createTestConfig(t, database, user.ID)

	// Create a config for another user
	otherUser := testutils.CreateTestUser(t, database, "other@example.com", false)
	otherConfig := createTestConfig(t, database, otherUser.ID)

	// Set up route
	router.PUT("/configs/:id", handlers.HandleUpdateConfig)

	// Prepare form data for update
	formData := url.Values{
		"name":             {"Updated Config"},
		"source_type":      {"local"},
		"source_path":      {"/updated/source"},
		"destination_type": {"local"},
		"destination_path": {"/updated/dest"},
		"file_pattern":     {"*.csv"},
	}

	// Test cases
	testCases := []struct {
		name         string
		configID     uint
		expectedCode int
		checkUpdate  bool
	}{
		{
			name:         "Update own config",
			configID:     config.ID,
			expectedCode: http.StatusFound, // Redirect to /configs
			checkUpdate:  true,
		},
		{
			name:         "Cannot update other user's config",
			configID:     otherConfig.ID,
			expectedCode: http.StatusForbidden,
			checkUpdate:  false,
		},
		{
			name:         "Non-existent config",
			configID:     9999,
			expectedCode: http.StatusNotFound,
			checkUpdate:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, _ := http.NewRequest("PUT", "/configs/"+strconv.Itoa(int(tc.configID)), strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(resp, req)

			// Check response code
			assert.Equal(t, tc.expectedCode, resp.Code)

			// Verify config was updated if expected
			if tc.checkUpdate {
				var updatedConfig db.TransferConfig
				database.First(&updatedConfig, tc.configID)

				assert.Equal(t, "Updated Config", updatedConfig.Name)
				assert.Equal(t, "/updated/source", updatedConfig.SourcePath)
				assert.Equal(t, "/updated/dest", updatedConfig.DestinationPath)
				assert.Equal(t, "*.csv", updatedConfig.FilePattern)
			}
		})
	}

	// Test admin access to update other user's config
	adminRouter := gin.New()
	adminRouter.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("isAdmin", true) // Set as admin
		c.Next()
	})
	adminRouter.PUT("/configs/:id", handlers.HandleUpdateConfig)

	// Admin should be able to update other user's config
	req, _ := http.NewRequest("PUT", "/configs/"+strconv.Itoa(int(otherConfig.ID)), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	adminRouter.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusFound, resp.Code)

	// Verify other user's config was updated
	var updatedOtherConfig db.TransferConfig
	database.First(&updatedOtherConfig, otherConfig.ID)
	assert.Equal(t, "Updated Config", updatedOtherConfig.Name)
}

func TestHandleDeleteConfig(t *testing.T) {
	handlers, router, database, user := setupConfigTest(t)

	// Create test config
	config := createTestConfig(t, database, user.ID)

	// Create a config for another user
	otherUser := testutils.CreateTestUser(t, database, "other@example.com", false)
	otherConfig := createTestConfig(t, database, otherUser.ID)

	// Create config with associated job
	configWithJob := createTestConfig(t, database, user.ID)
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "*/5 * * * *",
		ConfigID:  configWithJob.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	if err := database.Create(job).Error; err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	// Set up route
	router.DELETE("/configs/:id", handlers.HandleDeleteConfig)

	// Test cases
	testCases := []struct {
		name         string
		configID     uint
		expectedCode int
		errorMsg     string
	}{
		{
			name:         "Delete own config",
			configID:     config.ID,
			expectedCode: http.StatusOK,
			errorMsg:     "",
		},
		{
			name:         "Cannot delete other user's config",
			configID:     otherConfig.ID,
			expectedCode: http.StatusForbidden,
			errorMsg:     "You do not have permission to delete this config",
		},
		{
			name:         "Cannot delete config with jobs",
			configID:     configWithJob.ID,
			expectedCode: http.StatusBadRequest,
			errorMsg:     "Config is in use by jobs and cannot be deleted",
		},
		{
			name:         "Non-existent config",
			configID:     9999,
			expectedCode: http.StatusNotFound,
			errorMsg:     "Config not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, _ := http.NewRequest("DELETE", "/configs/"+strconv.Itoa(int(tc.configID)), nil)
			resp := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(resp, req)

			// Check response code
			assert.Equal(t, tc.expectedCode, resp.Code)

			if tc.errorMsg != "" {
				// Parse response body
				var response map[string]string
				err := json.Unmarshal(resp.Body.Bytes(), &response)
				assert.NoError(t, err)

				// Check error message
				assert.Equal(t, tc.errorMsg, response["error"])
			} else {
				// Verify config was deleted - using a new DB query
				var foundConfig db.TransferConfig
				err := database.First(&foundConfig, tc.configID).Error
				assert.Error(t, err, "Expected config to be deleted but it was found")
			}
		})
	}

	// Test admin access to delete other user's config
	adminRouter := gin.New()
	adminRouter.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("isAdmin", true) // Set as admin
		c.Next()
	})
	adminRouter.DELETE("/configs/:id", handlers.HandleDeleteConfig)

	// Admin should be able to delete other user's config
	req, _ := http.NewRequest("DELETE", "/configs/"+strconv.Itoa(int(otherConfig.ID)), nil)
	resp := httptest.NewRecorder()
	adminRouter.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	// Verify config was deleted
	var foundConfig db.TransferConfig
	err := database.First(&foundConfig, otherConfig.ID).Error
	assert.Error(t, err, "Expected config to be deleted but it was found")
}
