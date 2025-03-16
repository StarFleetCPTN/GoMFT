package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleGDriveAuth initiates the Google Drive authentication process
func (h *Handlers) HandleGDriveAuth(c *gin.Context) {
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

	if config.DestClientID != "" && config.DestClientSecret == "" {
		// If user provided just client ID but no secret, try to find the secret in the config
		_, existingClientSecret := h.DB.GetGDriveCredentialsFromConfig(config)

		if existingClientSecret != "" {
			// Use the secret from the existing config with the provided client ID
			clientSecret = existingClientSecret
		} else {
			// If we still can't find a matching secret, show an error
			RenderErrorPage(c, "Missing client secret", "You provided a custom client ID but no client secret. Both are required for Google authentication.")
			return
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

	// Create a config file with redirect URI-based auth
	configType := "drive"
	if config.DestinationType == "gphotos" {
		configType = "google photos"
	}

	configContent := fmt.Sprintf(`[temp_%s]
type = %s
client_id = %s
client_secret = %s
redirect_url = %s
`, config.DestinationType, configType, clientID, clientSecret, redirectURI)

	// Write the config file
	if err := os.WriteFile(tempConfigPath, []byte(configContent), 0644); err != nil {
		RenderErrorPage(c, "Failed to create temporary config file", err.Error())
		return
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
func (h *Handlers) HandleGDriveAuthCallback(c *gin.Context) {
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

	// Get the temp config path from cookie
	tempConfigPath, err := c.Cookie("gdrive_temp_config")
	if err != nil || tempConfigPath == "" {
		RenderErrorPage(c, "Session expired", "The authentication session has expired")
		return
	}

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
	redirectURI := fmt.Sprintf("%s/configs/gdrive-callback", baseURL)

	// Get the configuration to retrieve client ID and secret
	config, err := h.DB.GetTransferConfig(uint(configID))
	if err != nil {
		RenderErrorPage(c, "Failed to get configuration", err.Error())
		return
	}

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

	if config.DestClientID != "" && config.DestClientSecret == "" {
		// If user provided just client ID but no secret, try to find the secret in the config
		_, existingClientSecret := h.DB.GetGDriveCredentialsFromConfig(config)

		if existingClientSecret != "" {
			// Use the secret from the existing config with the provided client ID
			clientSecret = existingClientSecret
		} else {
			// If we still can't find a matching secret, show an error
			RenderErrorPage(c, "Missing client secret", "You provided a custom client ID but no client secret. Both are required for Google authentication.")
			return
		}
	}

	// Exchange auth code for token using HTTP request
	tokenURL := "https://oauth2.googleapis.com/token"
	formData := url.Values{
		"code":          {authCode},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	resp, err := http.PostForm(tokenURL, formData)
	if err != nil {
		RenderErrorPage(c, "Failed to exchange authorization code for token", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		RenderErrorPage(c, "Failed to read token response", err.Error())
		return
	}

	if resp.StatusCode != http.StatusOK {
		RenderErrorPage(c, "Failed to exchange authorization code for token", string(body))
		return
	}

	// Parse the token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		RenderErrorPage(c, "Failed to parse token response", err.Error())
		return
	}

	// Create a token JSON in the format rclone expects
	tokenJSON := fmt.Sprintf(`{
		"access_token": "%s",
		"token_type": "%s",
		"refresh_token": "%s",
		"expiry": "%s"
	}`,
		tokenResp.AccessToken,
		tokenResp.TokenType,
		tokenResp.RefreshToken,
		time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second).Format(time.RFC3339))

	// Mark the configuration as authenticated in the database
	config.SetGoogleDriveAuthenticated(true)
	if err := h.DB.UpdateTransferConfig(config); err != nil {
		RenderErrorPage(c, "Failed to update configuration", err.Error())
		return
	}

	// Generate the rclone config file with the token
	if err := h.DB.GenerateRcloneConfigWithToken(config, tokenJSON); err != nil {
		RenderErrorPage(c, "Failed to generate rclone configuration", err.Error())
		return
	}

	// Clean up the temporary file
	os.Remove(tempConfigPath)

	// Clear cookies
	c.SetCookie("gdrive_temp_config", "", -1, "/", "", false, true)
	c.SetCookie("gdrive_auth_state", "", -1, "/", "", false, true)
	c.SetCookie("gdrive_config_id", "", -1, "/", "", false, true)

	// Redirect to the config list with a success message
	var successParam string
	if config.DestinationType == "gphotos" {
		successParam = "gphotos_auth_success"
	} else {
		successParam = "gdrive_auth_success"
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/configs?status=%s", successParam))
}

// HandleGDriveTokenProcess processes a Google Drive token directly from a URL parameter
func (h *Handlers) HandleGDriveTokenProcess(c *gin.Context) {
	// Get the parameters
	configID := c.Query("config_id")
	if configID == "" {
		RenderErrorPage(c, "Missing configuration ID", "")
		return
	}

	token := c.Query("token")
	if token == "" {
		RenderErrorPage(c, "Missing token", "")
		return
	}

	// Parse config ID
	configIDUint, err := strconv.ParseUint(configID, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid configuration ID", err.Error())
		return
	}

	// Get the configuration
	config, err := h.DB.GetTransferConfig(uint(configIDUint))
	if err != nil {
		RenderErrorPage(c, "Configuration not found", err.Error())
		return
	}

	// Ensure it's a Google Drive configuration
	if config.DestinationType != "gdrive" {
		RenderErrorPage(c, "Not a Google Drive configuration", "")
		return
	}

	// Mark the configuration as authenticated
	config.SetGoogleDriveAuthenticated(true)
	if err := h.DB.UpdateTransferConfig(config); err != nil {
		RenderErrorPage(c, "Failed to update configuration", err.Error())
		return
	}

	// Generate the rclone config with the token
	if err := h.DB.GenerateRcloneConfigWithToken(config, token); err != nil {
		RenderErrorPage(c, "Failed to generate rclone configuration", err.Error())
		return
	}

	// Redirect to the config list with success
	c.Redirect(http.StatusFound, "/configs?status=gdrive_auth_success")
}

// RenderErrorPage renders an error page with the given message
func RenderErrorPage(c *gin.Context, title string, details string) {
	// Here we'd typically use a component for error display
	// For now, we'll just redirect to the configs page with an error in the query string
	errorURL := "/configs?error=" + url.QueryEscape(title)
	if details != "" {
		errorURL += "&details=" + url.QueryEscape(details)
	}
	c.Redirect(http.StatusFound, errorURL)
}
