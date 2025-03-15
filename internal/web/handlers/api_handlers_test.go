package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/scheduler"
	"github.com/starfleetcptn/gomft/internal/testutils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func setupAPITest(t *testing.T) (*Handlers, *gin.Engine, *db.DB, *db.User) {
	// Set up test database
	database := testutils.SetupTestDB(t)

	// Create test user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := &db.User{
		Email:              "test@example.com",
		PasswordHash:       string(hashedPassword),
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(user)

	// Create mock scheduler
	mockScheduler := scheduler.NewMockScheduler()

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handlers
	handlers := &Handlers{
		DB:        database,
		JWTSecret: "test-jwt-secret",
		Scheduler: mockScheduler,
	}

	return handlers, router, database, user
}

func setupAuthenticatedAPITest(t *testing.T, isAdmin bool) (*Handlers, *gin.Engine, *db.DB, *db.User) {
	handlers, router, database, user := setupAPITest(t)

	// Update user admin status if needed
	if isAdmin != user.IsAdmin {
		user.IsAdmin = isAdmin
		database.Save(user)
	}

	// Set up authentication middleware
	router.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("email", user.Email)
		c.Set("username", "testuser")
		c.Set("isAdmin", user.IsAdmin)
		c.Next()
	})

	return handlers, router, database, user
}

func TestHandleAPILogin(t *testing.T) {
	handlers, router, _, user := setupAPITest(t)

	// Set up route
	router.POST("/api/login", handlers.HandleAPILogin)

	// Test case 1: Successful login
	loginData := map[string]string{
		"email":    user.Email,
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify token exists
	token, exists := response["token"]
	assert.True(t, exists)
	assert.NotEmpty(t, token)

	// Verify user data
	userData, exists := response["user"]
	assert.True(t, exists)
	userMap := userData.(map[string]interface{})
	assert.Equal(t, float64(user.ID), userMap["id"])
	assert.Equal(t, user.Email, userMap["email"])
	assert.Equal(t, user.IsAdmin, userMap["is_admin"])

	// Test case 2: Invalid credentials
	loginData = map[string]string{
		"email":    user.Email,
		"password": "wrongpassword",
	}
	jsonData, _ = json.Marshal(loginData)

	req, _ = http.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusUnauthorized, resp.Code)

	// Test case 3: Invalid request format
	invalidJSON := []byte(`{"email": "test@example.com", "password":}`)

	req, _ = http.NewRequest("POST", "/api/login", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestHandleAPIConfigs(t *testing.T) {
	handlers, router, database, user := setupAuthenticatedAPITest(t, false)

	// Create test configs
	config1 := &db.TransferConfig{
		Name:            "Test Config 1",
		SourceType:      "local",
		SourcePath:      "/source1",
		DestinationType: "local",
		DestinationPath: "/dest1",
		CreatedBy:       user.ID,
	}
	database.Create(config1)

	config2 := &db.TransferConfig{
		Name:            "Test Config 2",
		SourceType:      "local",
		SourcePath:      "/source2",
		DestinationType: "local",
		DestinationPath: "/dest2",
		CreatedBy:       user.ID,
	}
	database.Create(config2)

	// Create config for another user
	otherUser := &db.User{
		Email:              "other@example.com",
		PasswordHash:       "hashedpassword",
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(otherUser)

	otherConfig := &db.TransferConfig{
		Name:            "Other User Config",
		SourceType:      "local",
		SourcePath:      "/source3",
		DestinationType: "local",
		DestinationPath: "/dest3",
		CreatedBy:       otherUser.ID,
	}
	database.Create(otherConfig)

	// Set up route
	router.GET("/api/configs", handlers.HandleAPIConfigs)

	// Create request
	req, _ := http.NewRequest("GET", "/api/configs", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify configs
	configs, exists := response["configs"]
	assert.True(t, exists)

	configsArray := configs.([]interface{})
	assert.Equal(t, 2, len(configsArray))

	// Verify only user's configs are returned
	foundConfig1 := false
	foundConfig2 := false
	foundOtherConfig := false

	for _, c := range configsArray {
		configMap := c.(map[string]interface{})
		if configMap["name"] == config1.Name {
			foundConfig1 = true
		}
		if configMap["name"] == config2.Name {
			foundConfig2 = true
		}
		if configMap["name"] == otherConfig.Name {
			foundOtherConfig = true
		}
	}

	assert.True(t, foundConfig1)
	assert.True(t, foundConfig2)
	assert.False(t, foundOtherConfig)
}

func TestHandleAPIConfig(t *testing.T) {
	handlers, router, database, user := setupAuthenticatedAPITest(t, false)

	// Create test config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	database.Create(config)

	// Create config for another user
	otherUser := &db.User{
		Email:              "other@example.com",
		PasswordHash:       "hashedpassword",
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(otherUser)

	otherConfig := &db.TransferConfig{
		Name:            "Other User Config",
		SourceType:      "local",
		SourcePath:      "/source2",
		DestinationType: "local",
		DestinationPath: "/dest2",
		CreatedBy:       otherUser.ID,
	}
	database.Create(otherConfig)

	// Set up route
	router.GET("/api/configs/:id", handlers.HandleAPIConfig)

	// Test case 1: Get own config
	req, _ := http.NewRequest("GET", "/api/configs/"+strconv.Itoa(int(config.ID)), nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify config
	configData, exists := response["config"]
	assert.True(t, exists)
	configMap := configData.(map[string]interface{})
	assert.Equal(t, config.Name, configMap["name"])

	// Test case 2: Try to get another user's config
	req, _ = http.NewRequest("GET", "/api/configs/"+strconv.Itoa(int(otherConfig.ID)), nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response - should be forbidden
	assert.Equal(t, http.StatusForbidden, resp.Code)

	// Test case 3: Admin can access any config
	// Create admin router
	adminHandlers, adminRouter, _, _ := setupAuthenticatedAPITest(t, true)
	adminRouter.GET("/api/configs/:id", adminHandlers.HandleAPIConfig)

	req, _ = http.NewRequest("GET", "/api/configs/"+strconv.Itoa(int(otherConfig.ID)), nil)
	resp = httptest.NewRecorder()
	adminRouter.ServeHTTP(resp, req)

	// Check response - admin should be able to access
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test case 4: Non-existent config
	req, _ = http.NewRequest("GET", "/api/configs/9999", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandleAPICreateConfig(t *testing.T) {
	handlers, router, _, user := setupAuthenticatedAPITest(t, false)

	// Set up route
	router.POST("/api/configs", handlers.HandleAPICreateConfig)

	// Create config data
	configData := map[string]interface{}{
		"name":             "New API Config",
		"source_type":      "local",
		"source_path":      "/api/source",
		"destination_type": "local",
		"destination_path": "/api/dest",
	}
	jsonData, _ := json.Marshal(configData)

	// Create request
	req, _ := http.NewRequest("POST", "/api/configs", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusCreated, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify config was created
	configResponse, exists := response["config"]
	assert.True(t, exists)
	configMap, ok := configResponse.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "New API Config", configMap["name"])
	assert.Equal(t, float64(user.ID), configMap["created_by"])

	// Test case 2: Invalid request data
	invalidJSON := []byte(`{"name": "Invalid Config", "source_type":}`)

	req, _ = http.NewRequest("POST", "/api/configs", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestHandleAPIUpdateConfig(t *testing.T) {
	handlers, router, database, user := setupAuthenticatedAPITest(t, false)

	// Create test config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	database.Create(config)

	// Create config for another user
	otherUser := &db.User{
		Email:              "other@example.com",
		PasswordHash:       "hashedpassword",
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(otherUser)

	otherConfig := &db.TransferConfig{
		Name:            "Other User Config",
		SourceType:      "local",
		SourcePath:      "/source2",
		DestinationType: "local",
		DestinationPath: "/dest2",
		CreatedBy:       otherUser.ID,
	}
	database.Create(otherConfig)

	// Set up route
	router.PUT("/api/configs/:id", handlers.HandleAPIUpdateConfig)

	// Test case 1: Update own config
	updateData := map[string]interface{}{
		"name":             "Updated Config",
		"source_type":      "local",
		"source_path":      "/updated/source",
		"destination_type": "local",
		"destination_path": "/updated/dest",
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PUT", "/api/configs/"+strconv.Itoa(int(config.ID)), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify config was updated
	configData, exists := response["config"]
	assert.True(t, exists)
	configMap := configData.(map[string]interface{})
	assert.Equal(t, "Updated Config", configMap["name"])
	assert.Equal(t, "/updated/source", configMap["source_path"])

	// Test case 2: Try to update another user's config
	updateData = map[string]interface{}{
		"name": "Trying to update other's config",
	}
	jsonData, _ = json.Marshal(updateData)

	req, _ = http.NewRequest("PUT", "/api/configs/"+strconv.Itoa(int(otherConfig.ID)), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response - should be forbidden
	assert.Equal(t, http.StatusForbidden, resp.Code)

	// Test case 3: Admin can update any config
	// Create admin router
	adminHandlers, adminRouter, _, _ := setupAuthenticatedAPITest(t, true)
	adminRouter.PUT("/api/configs/:id", adminHandlers.HandleAPIUpdateConfig)

	updateData = map[string]interface{}{
		"name": "Admin Updated Config",
	}
	jsonData, _ = json.Marshal(updateData)

	req, _ = http.NewRequest("PUT", "/api/configs/"+strconv.Itoa(int(otherConfig.ID)), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	adminRouter.ServeHTTP(resp, req)

	// Check response - admin should be able to update
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test case 4: Non-existent config
	req, _ = http.NewRequest("PUT", "/api/configs/9999", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandleAPIDeleteConfig(t *testing.T) {
	handlers, router, database, user := setupAuthenticatedAPITest(t, false)

	// Create test config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	database.Create(config)

	// Create config for another user
	otherUser := &db.User{
		Email:              "other@example.com",
		PasswordHash:       "hashedpassword",
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(otherUser)

	otherConfig := &db.TransferConfig{
		Name:            "Other User Config",
		SourceType:      "local",
		SourcePath:      "/source2",
		DestinationType: "local",
		DestinationPath: "/dest2",
		CreatedBy:       otherUser.ID,
	}
	database.Create(otherConfig)

	// Create config with associated job
	configWithJob := &db.TransferConfig{
		Name:            "Config With Job",
		SourceType:      "local",
		SourcePath:      "/source3",
		DestinationType: "local",
		DestinationPath: "/dest3",
		CreatedBy:       user.ID,
	}
	database.Create(configWithJob)

	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "* * * * *",
		ConfigID:  configWithJob.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	database.Create(job)

	// Set up route
	router.DELETE("/api/configs/:id", handlers.HandleAPIDeleteConfig)

	// Test case 1: Delete own config
	req, _ := http.NewRequest("DELETE", "/api/configs/"+strconv.Itoa(int(config.ID)), nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	// Verify config was deleted
	var deletedConfig db.TransferConfig
	err := database.First(&deletedConfig, config.ID).Error
	assert.Error(t, err) // Should not find the config

	// Test case 2: Try to delete another user's config
	req, _ = http.NewRequest("DELETE", "/api/configs/"+strconv.Itoa(int(otherConfig.ID)), nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response - should be forbidden
	assert.Equal(t, http.StatusForbidden, resp.Code)

	// Test case 3: Try to delete config with associated job
	req, _ = http.NewRequest("DELETE", "/api/configs/"+strconv.Itoa(int(configWithJob.ID)), nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response - should be bad request
	assert.Equal(t, http.StatusBadRequest, resp.Code)

	// Test case 4: Admin can delete any config
	// Create admin router
	adminHandlers, adminRouter, _, _ := setupAuthenticatedAPITest(t, true)
	adminRouter.DELETE("/api/configs/:id", adminHandlers.HandleAPIDeleteConfig)

	req, _ = http.NewRequest("DELETE", "/api/configs/"+strconv.Itoa(int(otherConfig.ID)), nil)
	resp = httptest.NewRecorder()
	adminRouter.ServeHTTP(resp, req)

	// Check response - admin should be able to delete
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test case 5: Non-existent config
	req, _ = http.NewRequest("DELETE", "/api/configs/9999", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandleAPIRunJob(t *testing.T) {
	// Setup test environment
	handlers, router, database, user := setupAuthenticatedAPITest(t, false)

	// Create test config
	config := &db.TransferConfig{
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/source",
		DestinationType: "local",
		DestinationPath: "/dest",
		CreatedBy:       user.ID,
	}
	database.Create(config)

	// Create test job
	job := &db.Job{
		Name:      "Test Job",
		Schedule:  "* * * * *",
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: user.ID,
	}
	database.Create(job)

	// Create job for another user
	otherUser := &db.User{
		Email:              "other@example.com",
		PasswordHash:       "hashedpassword",
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(otherUser)

	otherJob := &db.Job{
		Name:      "Other User Job",
		Schedule:  "* * * * *",
		ConfigID:  config.ID,
		Enabled:   true,
		CreatedBy: otherUser.ID,
	}
	database.Create(otherJob)

	// Set up route
	router.POST("/api/jobs/:id/run", handlers.HandleAPIRunJob)

	// Test case 1: Run own job
	req, _ := http.NewRequest("POST", "/api/jobs/"+strconv.Itoa(int(job.ID))+"/run", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test case 2: Try to run another user's job
	req, _ = http.NewRequest("POST", "/api/jobs/"+strconv.Itoa(int(otherJob.ID))+"/run", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response - should be forbidden
	assert.Equal(t, http.StatusForbidden, resp.Code)

	// Test case 3: Admin can run any job
	// Create a new router with admin permissions but using the same handlers
	adminRouter := gin.New()
	adminRouter.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("email", user.Email)
		c.Set("username", "testuser")
		c.Set("isAdmin", true) // Set admin flag to true
		c.Next()
	})
	adminRouter.POST("/api/jobs/:id/run", handlers.HandleAPIRunJob)

	req, _ = http.NewRequest("POST", "/api/jobs/"+strconv.Itoa(int(otherJob.ID))+"/run", nil)
	resp = httptest.NewRecorder()
	adminRouter.ServeHTTP(resp, req)

	// Check response - admin should be able to run
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test case 4: Non-existent job
	req, _ = http.NewRequest("POST", "/api/jobs/9999/run", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response - should be not found
	assert.Equal(t, http.StatusNotFound, resp.Code)
}
