package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
)

// HandleStorageProviderGDriveAuth initiates the Google Drive authentication process for storage providers
func (h *Handlers) HandleStorageProviderGDriveAuth(c *gin.Context) {
	// Get the provider ID from the query parameter
	providerIDStr := c.Param("id")
	if providerIDStr == "" {
		RenderErrorPage(c, "Missing provider ID", "")
		return
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid provider ID", err.Error())
		return
	}

	// Get the provider
	provider, err := h.DB.GetStorageProvider(uint(providerID))
	if err != nil {
		RenderErrorPage(c, "Provider not found", err.Error())
		return
	}

	// Ensure it's a Google Drive or Google Photos provider
	if provider.Type != "drive" && provider.Type != "gphotos" {
		RenderErrorPage(c, "Not a Google provider", "The selected provider is not set up for Google Drive or Google Photos")
		return
	}

	// Prepare for OAuth
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Create a temporary config file for authentication
	tempConfigDir := filepath.Join(dataDir, "temp")
	if err := os.MkdirAll(tempConfigDir, 0755); err != nil {
		RenderErrorPage(c, "Failed to create temporary directory", err.Error())
		return
	}

	tempConfigPath := filepath.Join(tempConfigDir, fmt.Sprintf("gdrive_auth_provider_%d.conf", provider.ID))

	// Store the temporary config path in a cookie
	c.SetCookie("gdrive_temp_config_provider", tempConfigPath, 3600, "/", "", false, true)

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
	redirectURI := fmt.Sprintf("%s/storage-providers/gdrive-callback", baseURL)

	// Attempt to get GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET from ENV
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	// Check if provider has client credentials
	if provider.ClientID != "" {
		clientID = provider.ClientID
	}
	if provider.ClientSecret != "" {
		clientSecret = provider.ClientSecret
	}

	if clientID == "" || clientSecret == "" {
		// fallback to rclone client ID and secret
		clientID = "202264815644.apps.googleusercontent.com"
		clientSecret = "X4Z3ca8xfWDb1Voo-F9a7ZxJ"
	}

	// Generate state parameter for security (to prevent CSRF)
	state := fmt.Sprintf("gomft_provider_%d_%d", provider.ID, time.Now().Unix())

	// Store state in cookie for validation during callback
	c.SetCookie("gdrive_auth_state_provider", state, 3600, "/", "", false, true)

	// Store provider ID in cookie for use during callback
	c.SetCookie("gdrive_provider_id", providerIDStr, 3600, "/", "", false, true)

	// Determine the appropriate scope based on provider type
	var scope string
	if provider.Type == "google_photo" {
		scope = url.QueryEscape("https://www.googleapis.com/auth/photoslibrary")
	} else {
		// Default to Google Drive scope
		scope = url.QueryEscape("https://www.googleapis.com/auth/drive")
	}

	// Create a config file with redirect URI-based auth
	configType := "drive"
	if provider.Type == "google_photo" {
		configType = "google photos"
	}

	// Use a standardized name for the rclone config section
	configSection := "temp_drive"
	if provider.Type == "google_photo" {
		configSection = "temp_gphotos"
	}

	configContent := fmt.Sprintf(`[%s]
type = %s
client_id = %s
client_secret = %s
redirect_url = %s
`, configSection, configType, clientID, clientSecret, redirectURI)

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

// HandleStorageProviderGDriveAuthCallback handles the callback from Google OAuth for storage providers
func (h *Handlers) HandleStorageProviderGDriveAuthCallback(c *gin.Context) {
	// Get auth code from query parameters
	authCode := c.Query("code")
	if authCode == "" {
		RenderErrorPage(c, "Authentication failed", "No authorization code received from Google")
		return
	}

	// Verify state parameter to prevent CSRF
	state := c.Query("state")
	storedState, err := c.Cookie("gdrive_auth_state_provider")
	if err != nil || state != storedState {
		RenderErrorPage(c, "Authentication failed", "Invalid state parameter")
		return
	}

	// Get provider ID from cookie
	providerIDStr, err := c.Cookie("gdrive_provider_id")
	if err != nil {
		RenderErrorPage(c, "Authentication failed", "Unable to retrieve provider ID")
		return
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid provider ID", err.Error())
		return
	}

	// Get the temp config path from cookie
	tempConfigPath, err := c.Cookie("gdrive_temp_config_provider")
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
	redirectURI := fmt.Sprintf("%s/storage-providers/gdrive-callback", baseURL)

	// Get the provider to retrieve client ID and secret
	provider, err := h.DB.GetStorageProvider(uint(providerID))
	if err != nil {
		RenderErrorPage(c, "Failed to get provider", err.Error())
		return
	}

	// Attempt to get GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET from provider or ENV
	clientID := provider.ClientID
	clientSecret := provider.ClientSecret

	if clientID == "" {
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		// fallback to rclone client ID and secret
		clientID = "202264815644.apps.googleusercontent.com"
		clientSecret = "X4Z3ca8xfWDb1Voo-F9a7ZxJ"
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

	// Mark the provider as authenticated in the database
	authenticated := true
	provider.Authenticated = &authenticated
	if err := h.DB.UpdateStorageProvider(provider); err != nil {
		RenderErrorPage(c, "Failed to update provider", err.Error())
		return
	}

	// Store the token in the provider's refresh token field
	provider.RefreshToken = tokenJSON
	if err := h.DB.UpdateStorageProvider(provider); err != nil {
		RenderErrorPage(c, "Failed to store token", err.Error())
		return
	}

	// Clean up the temporary file
	os.Remove(tempConfigPath)

	// Clear cookies
	c.SetCookie("gdrive_temp_config_provider", "", -1, "/", "", false, true)
	c.SetCookie("gdrive_auth_state_provider", "", -1, "/", "", false, true)
	c.SetCookie("gdrive_provider_id", "", -1, "/", "", false, true)

	// Redirect to the provider list with a success message
	c.Redirect(http.StatusFound, "/storage-providers?status=gdrive_auth_success")
}

// HandleStorageProviderGDriveTokenProcess processes a Google Drive token directly from a URL parameter for storage providers
func (h *Handlers) HandleStorageProviderGDriveTokenProcess(c *gin.Context) {
	// Get the parameters
	providerID := c.Query("provider_id")
	if providerID == "" {
		RenderErrorPage(c, "Missing provider ID", "")
		return
	}

	token := c.Query("token")
	if token == "" {
		RenderErrorPage(c, "Missing token", "")
		return
	}

	// Parse provider ID
	providerIDUint, err := strconv.ParseUint(providerID, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid provider ID", err.Error())
		return
	}

	// Get the provider
	provider, err := h.DB.GetStorageProvider(uint(providerIDUint))
	if err != nil {
		RenderErrorPage(c, "Provider not found", err.Error())
		return
	}

	// Ensure it's a Google Drive or Google Photos provider
	if provider.Type != "drive" && provider.Type != "gphotos" {
		RenderErrorPage(c, "Not a Google provider", "")
		return
	}

	// Mark the provider as authenticated
	authenticated := true
	provider.Authenticated = &authenticated
	if err := h.DB.UpdateStorageProvider(provider); err != nil {
		RenderErrorPage(c, "Failed to update provider", err.Error())
		return
	}

	// Store the token in the provider's refresh token field
	provider.RefreshToken = token
	if err := h.DB.UpdateStorageProvider(provider); err != nil {
		RenderErrorPage(c, "Failed to store token", err.Error())
		return
	}

	// Redirect to the provider list with success
	c.Redirect(http.StatusFound, "/storage-providers?status=gdrive_auth_success")
}

// HandleStorageProviderGDriveHeadlessAuth initiates the headless Google Drive/Photos authentication process for storage providers
func (h *Handlers) HandleStorageProviderGDriveHeadlessAuth(c *gin.Context) {
	// Get the provider ID from the query parameter
	providerIDStr := c.Param("id")
	if providerIDStr == "" {
		RenderErrorPage(c, "Missing provider ID", "")
		return
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid provider ID", err.Error())
		return
	}

	// Get the provider
	provider, err := h.DB.GetStorageProvider(uint(providerID))
	if err != nil {
		RenderErrorPage(c, "Provider not found", err.Error())
		return
	}

	// Ensure it's a Google Drive or Google Photos provider
	if provider.Type != "drive" && provider.Type != "gphotos" {
		RenderErrorPage(c, "Not a Google provider", "The selected provider is not set up for Google Drive or Google Photos")
		return
	}

	// Determine which Google service we're authenticating with
	var serviceType string
	if provider.Type == "drive" {
		serviceType = "drive"
	} else {
		serviceType = "gphotos"
	}

	// Get client ID and secret
	clientID := provider.ClientID
	clientSecret := provider.ClientSecret

	// If not provided in provider, try env variables
	if clientID == "" {
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	}

	// If still not provided, use default rclone values
	if clientID == "" {
		clientID = "202264815644.apps.googleusercontent.com"
	}
	if clientSecret == "" {
		clientSecret = "X4Z3ca8xfWDb1Voo-F9a7ZxJ"
	}

	// Generate and return the authorize command to be run on a machine with a browser
	authorizeCommand := fmt.Sprintf("rclone authorize \"%s\"", serviceType)

	// If using custom client ID/secret, include them in the command
	if clientID != "202264815644.apps.googleusercontent.com" || clientSecret != "X4Z3ca8xfWDb1Voo-F9a7ZxJ" {
		authorizeCommand = fmt.Sprintf("rclone authorize \"%s\" %s %s", serviceType, clientID, clientSecret)
	}

	// Log the command for debugging
	log.Printf("Generated headless auth command for provider: %s", authorizeCommand)

	// Store provider ID in cookie for use during token submission
	c.SetCookie("gdrive_headless_provider_id", providerIDStr, 3600*24, "/", "", false, true)

	data := components.StorageProviderGDriveHeadlessAuthData{
		AuthCommand: authorizeCommand,
		ProviderID:  providerIDStr,
	}

	components.StorageProviderGDriveHeadlessAuth(c, data).Render(c, c.Writer)
}

// HandleStorageProviderGDriveHeadlessTokenSubmit handles the submission of the token from the headless auth for storage providers
func (h *Handlers) HandleStorageProviderGDriveHeadlessTokenSubmit(c *gin.Context) {
	// Get the auth token from form submission
	authToken := c.PostForm("auth_token")
	if authToken == "" {
		RenderErrorPage(c, "Missing authentication token", "")
		return
	}

	// Get provider ID from cookie or form
	providerIDStr, err := c.Cookie("gdrive_headless_provider_id")
	if err != nil {
		// If not in cookie, try from form
		providerIDStr = c.PostForm("provider_id") // Use provider_id from the form
		if providerIDStr == "" {
			RenderErrorPage(c, "Authentication failed", "Unable to retrieve provider ID")
			return
		}
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		RenderErrorPage(c, "Invalid provider ID", err.Error())
		return
	}

	// Get the provider
	provider, err := h.DB.GetStorageProvider(uint(providerID))
	if err != nil {
		RenderErrorPage(c, "Provider not found", err.Error())
		return
	}

	// Mark the provider as authenticated
	authenticated := true
	provider.Authenticated = &authenticated
	if err := h.DB.UpdateStorageProvider(provider); err != nil {
		RenderErrorPage(c, "Failed to update provider", err.Error())
		return
	}

	// Store the token in the provider's refresh token field
	provider.RefreshToken = authToken
	if err := h.DB.UpdateStorageProvider(provider); err != nil {
		RenderErrorPage(c, "Failed to store token", err.Error())
		return
	}

	// Clear cookie
	c.SetCookie("gdrive_headless_provider_id", "", -1, "/", "", false, true)

	// Redirect to the providers page with success message
	c.Redirect(http.StatusFound, "/storage-providers?status=gdrive_auth_success")
}
