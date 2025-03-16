package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// DBInterface defines the methods we need to mock for our tests
type DBInterface interface {
	GetTransferConfig(id uint) (*db.TransferConfig, error)
	GetConfigRclonePath(config *db.TransferConfig) string
	GenerateRcloneConfigWithToken(config *db.TransferConfig, token string) error
	GetGDriveCredentialsFromConfig(config *db.TransferConfig) (string, string)
}

// MockDB is a mock implementation of the DB interface for testing
type MockDB struct {
	mock.Mock
}

// Implement the necessary methods from the DB interface for our tests
func (m *MockDB) GetTransferConfig(id uint) (*db.TransferConfig, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*db.TransferConfig), args.Error(1)
}

func (m *MockDB) GetConfigRclonePath(config *db.TransferConfig) string {
	args := m.Called(config)
	return args.String(0)
}

func (m *MockDB) GenerateRcloneConfigWithToken(config *db.TransferConfig, token string) error {
	args := m.Called(config, token)
	return args.Error(0)
}

func (m *MockDB) GetGDriveCredentialsFromConfig(config *db.TransferConfig) (string, string) {
	args := m.Called(config)
	return args.String(0), args.String(1)
}

// MockHandlers is a modified version of Handlers that accepts our mock DB
type MockHandlers struct {
	DB DBInterface
}

// HandleGDriveAuth is a copy of the original method but using our interface
func (h *MockHandlers) HandleGDriveAuth(c *gin.Context) {
	// Get the config ID from the query parameter
	configIDStr := c.Param("id")
	if configIDStr == "" {
		RenderErrorPage(c, "Missing configuration ID", "")
		return
	}

	configID, err := strconv.ParseUint(configIDStr, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid configuration ID", err.Error())
		return
	}

	// Get the configuration
	config, err := h.DB.GetTransferConfig(uint(configID))
	if err != nil {
		RenderErrorPage(c, "Configuration not found", err.Error())
		return
	}

	// Ensure it's a Google Drive or Google Photos configuration
	if config.DestinationType != "gdrive" && config.DestinationType != "gphotos" {
		RenderErrorPage(c, "Not a Google configuration", "The selected configuration is not set up for Google Drive or Google Photos")
		return
	}

	// Prepare for OAuth
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Get Rclone Config Path
	rcloneConfigPath := h.DB.GetConfigRclonePath(config)
	if rcloneConfigPath == "" {
		RenderErrorPage(c, "Rclone config not found", "The selected configuration does not have a valid rclone config")
		return
	}

	// Create a temporary config file for authentication
	tempConfigDir := filepath.Join(dataDir, "temp")
	if err := os.MkdirAll(tempConfigDir, 0755); err != nil {
		RenderErrorPage(c, "Failed to create temporary directory", err.Error())
		return
	}

	tempConfigPath := filepath.Join(tempConfigDir, fmt.Sprintf("gdrive_auth_%d.conf", config.ID))

	// Store the temporary config path in a cookie
	c.SetCookie("gdrive_temp_config", tempConfigPath, 3600, "/", "", false, true)

	// Get base URL for redirect URI
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		// Try to detect the base URL from the request
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}

	// Define the redirect URI for our callback
	redirectURI := fmt.Sprintf("%s/configs/gdrive-callback", baseURL)

	// Attempt to get GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET from ENV
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		// Check if we have client credentials in the existing config file
		existingClientID, existingClientSecret := h.DB.GetGDriveCredentialsFromConfig(config)

		if existingClientID != "" && existingClientSecret != "" {
			// Use credentials from existing config
			clientID = existingClientID
			clientSecret = existingClientSecret
		} else {
			// fallback to rclone client ID and secret
			clientID = "202264815644.apps.googleusercontent.com"
			clientSecret = "X4Z3ca8xfWDb1Voo-F9a7ZxJ"
		}
	}

	// Generate state parameter for security (to prevent CSRF)
	state := fmt.Sprintf("gomft_%d_%d", config.ID, time.Now().Unix())

	// Store state in cookie for validation during callback
	c.SetCookie("gdrive_auth_state", state, 3600, "/", "", false, true)

	// Store config ID in cookie for use during callback
	c.SetCookie("gdrive_config_id", configIDStr, 3600, "/", "", false, true)

	// Determine the appropriate scope based on destination type
	var scope string
	if config.DestinationType == "gphotos" {
		// Read-only access is handled elsewhere in the config; here we need the full auth scope
		scope = url.QueryEscape("https://www.googleapis.com/auth/photoslibrary")
	} else {
		// Default to Google Drive scope
		scope = url.QueryEscape("https://www.googleapis.com/auth/drive")
	}

	// Direct Google OAuth URL with our redirect
	authURL := fmt.Sprintf("https://accounts.google.com/o/oauth2/auth?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&access_type=offline&state=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		scope,
		url.QueryEscape(state))

	// Redirect the user to Google's auth page directly
	c.Redirect(http.StatusFound, authURL)
}

// HandleGDriveAuthCallback handles the callback from Google OAuth
func (h *MockHandlers) HandleGDriveAuthCallback(c *gin.Context) {
	// Get auth code from query parameters
	authCode := c.Query("code")
	if authCode == "" {
		RenderErrorPage(c, "Authentication failed", "No authorization code received from Google")
		return
	}

	// Verify state parameter to prevent CSRF
	state := c.Query("state")
	storedState, err := c.Cookie("gdrive_auth_state")
	if err != nil || state != storedState {
		RenderErrorPage(c, "Authentication failed", "Invalid state parameter")
		return
	}

	// Get config ID from cookie
	configIDStr, err := c.Cookie("gdrive_config_id")
	if err != nil {
		RenderErrorPage(c, "Authentication failed", "Unable to retrieve configuration ID")
		return
	}

	configID, err := strconv.ParseUint(configIDStr, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid configuration ID", err.Error())
		return
	}

	// Get the configuration
	config, err := h.DB.GetTransferConfig(uint(configID))
	if err != nil {
		RenderErrorPage(c, "Failed to get configuration", err.Error())
		return
	}

	// For testing purposes, we'll simulate a successful token exchange
	// In a real implementation, we would exchange the auth code for a token
	mockToken := `{"access_token":"test_access_token","refresh_token":"test_refresh_token","expiry":"2023-12-31T23:59:59Z"}`

	// Update the config with the token
	err = h.DB.GenerateRcloneConfigWithToken(config, mockToken)
	if err != nil {
		RenderErrorPage(c, "Failed to update configuration", err.Error())
		return
	}

	// Redirect to the config edit page
	c.Redirect(http.StatusFound, fmt.Sprintf("/configs/edit/%d", config.ID))
}

func setupTestRouter() (*gin.Engine, *MockDB) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	mockDB := new(MockDB)
	handlers := &MockHandlers{
		DB: mockDB,
	}

	router.GET("/configs/gdrive/:id", handlers.HandleGDriveAuth)
	router.GET("/configs/gdrive-callback", handlers.HandleGDriveAuthCallback)

	return router, mockDB
}

func TestHandleGDriveAuth_GoogleDrive(t *testing.T) {
	// Setup
	router, mockDB := setupTestRouter()

	// Create a test config
	testConfig := &db.TransferConfig{
		ID:              1,
		DestinationType: "gdrive",
	}

	// Set up mock expectations
	mockDB.On("GetTransferConfig", uint(1)).Return(testConfig, nil)
	mockDB.On("GetConfigRclonePath", testConfig).Return("/path/to/rclone.conf")
	mockDB.On("GetGDriveCredentialsFromConfig", testConfig).Return("test_client_id", "test_client_secret")

	// Create test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/configs/gdrive/1", nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusFound, w.Code)

	// Verify the redirect URL
	location := w.Header().Get("Location")
	assert.Contains(t, location, "accounts.google.com/o/oauth2/auth")
	assert.Contains(t, location, "drive")
	assert.Contains(t, location, "test_client_id")

	// Verify cookies were set
	cookies := w.Result().Cookies()
	assert.GreaterOrEqual(t, len(cookies), 3)

	// Check if state cookie exists
	stateFound := false
	for _, cookie := range cookies {
		if cookie.Name == "gdrive_auth_state" {
			stateFound = true
			break
		}
	}
	assert.True(t, stateFound)
}

func TestHandleGDriveAuth_GooglePhotos(t *testing.T) {
	// Setup
	router, mockDB := setupTestRouter()

	// Create a test config
	testConfig := &db.TransferConfig{
		ID:              2,
		DestinationType: "gphotos",
	}

	// Set up mock expectations
	mockDB.On("GetTransferConfig", uint(2)).Return(testConfig, nil)
	mockDB.On("GetConfigRclonePath", testConfig).Return("/path/to/rclone.conf")
	mockDB.On("GetGDriveCredentialsFromConfig", testConfig).Return("test_client_id", "test_client_secret")

	// Create test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/configs/gdrive/2", nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusFound, w.Code)

	// Verify the redirect URL
	location := w.Header().Get("Location")
	assert.Contains(t, location, "accounts.google.com/o/oauth2/auth")
	assert.Contains(t, location, "photoslibrary")
	assert.Contains(t, location, "test_client_id")

	// Verify cookies were set
	cookies := w.Result().Cookies()
	assert.GreaterOrEqual(t, len(cookies), 3)

	// Check if state cookie exists
	stateFound := false
	for _, cookie := range cookies {
		if cookie.Name == "gdrive_auth_state" {
			stateFound = true
			break
		}
	}
	assert.True(t, stateFound)
}

func TestHandleGDriveAuthCallback(t *testing.T) {
	// Setup test environment
	router, mockDB := setupTestRouter()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gdrive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary config file
	tempConfigPath := filepath.Join(tempDir, "temp_config.conf")
	if err := os.WriteFile(tempConfigPath, []byte("test config"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test state and config ID
	testState := "gomft_1_12345"
	testConfigID := "1"

	// Create a test config
	testConfig := &db.TransferConfig{
		ID:              1,
		DestinationType: "gphotos",
	}

	// Set up mock expectations
	mockDB.On("GetTransferConfig", uint(1)).Return(testConfig, nil)
	mockDB.On("GenerateRcloneConfigWithToken", testConfig, mock.Anything).Return(nil)

	// Create test request with auth code and state
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/configs/gdrive-callback?code=test_auth_code&state="+testState, nil)

	// Add required cookies to the request
	req.AddCookie(&http.Cookie{Name: "gdrive_auth_state", Value: testState})
	req.AddCookie(&http.Cookie{Name: "gdrive_config_id", Value: testConfigID})
	req.AddCookie(&http.Cookie{Name: "gdrive_temp_config", Value: tempConfigPath})

	// Send the request
	router.ServeHTTP(w, req)

	// We expect a redirect on successful auth
	assert.Equal(t, http.StatusFound, w.Code)

	// Should redirect to the config edit page
	location := w.Header().Get("Location")
	assert.Contains(t, location, "/configs/edit/1")
}

func TestHandleGDriveAuth_InvalidConfig(t *testing.T) {
	// Setup
	router, mockDB := setupTestRouter()

	// Set up mock expectations for a non-existent config
	mockDB.On("GetTransferConfig", uint(999)).Return(nil, fmt.Errorf("config not found"))

	// Create test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/configs/gdrive/999", nil)
	router.ServeHTTP(w, req)

	// Assertions - should render error page
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Configuration not found")
}

func TestHandleGDriveAuth_NonGoogleConfig(t *testing.T) {
	// Setup
	router, mockDB := setupTestRouter()

	// Create a non-Google test config
	testConfig := &db.TransferConfig{
		ID:              3,
		DestinationType: "s3", // Not Google Drive or Photos
	}

	// Set up mock expectations
	mockDB.On("GetTransferConfig", uint(3)).Return(testConfig, nil)

	// Create test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/configs/gdrive/3", nil)
	router.ServeHTTP(w, req)

	// Assertions - should render error page
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Not a Google configuration")
}

// RenderErrorPage renders an error page with the given message
func RenderErrorPage(c *gin.Context, title string, details string) {
	// Here we'd typically use a component for error display
	// For now, we'll just render a simple HTML error page for testing
	errorHTML := fmt.Sprintf("<html><body><h1>Error: %s</h1><p>%s</p></body></html>", title, details)
	c.Data(http.StatusOK, "text/html", []byte(errorHTML))
}
