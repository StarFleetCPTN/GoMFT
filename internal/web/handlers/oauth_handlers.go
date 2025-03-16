package handlers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// GoogleDriveAuthHandler initiates the Google Drive OAuth flow
func (h *Handlers) HandleGoogleDriveAuth(c *gin.Context) {
	configID := c.Query("config_id")
	if configID == "" {
		RenderErrorPage(c, "Missing config_id parameter", "")
		return
	}

	// Prepare for the OAuth flow
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Ensure oauth directory exists
	oauthDir := filepath.Join(dataDir, "oauth")
	if err := os.MkdirAll(oauthDir, 0755); err != nil {
		RenderErrorPage(c, "Failed to create oauth directory", err.Error())
		return
	}

	// Get rclone path
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	// Set up a temporary rclone config
	tempConfigPath := filepath.Join(oauthDir, fmt.Sprintf("temp_gdrive_%s.conf", configID))

	// Build rclone command to get auth URL
	cmd := exec.Command(
		rclonePath,
		"config",
		"create",
		"temp_gdrive",
		"drive",
		"--config",
		tempConfigPath,
	)

	// Set a timeout context
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Run the command with proper context handling
	// We can't use exec.CommandContext directly since we're creating the command differently
	// So we'll use a goroutine with the context's Done() channel to handle cancellation
	go func() {
		<-ctx.Done() // Wait for context to be done (timeout or cancellation)
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				RenderErrorPage(c, "Failed to kill rclone process", err.Error())
			}
		}
	}()

	// Run the command to get the browser URL (this will fail in a specific way)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Look for the URL in the output
		authURL := extractAuthURL(outputStr)
		if authURL == "" {
			RenderErrorPage(c, "Failed to get Google Drive authentication URL", outputStr)
			return
		}

		// Store the config ID in the session
		session := sessions.Default(c)
		session.Set("gdrive_config_id", configID)
		session.Set("gdrive_temp_config", tempConfigPath)
		if err := session.Save(); err != nil {
			RenderErrorPage(c, "Failed to save session", err.Error())
			return
		}

		// Use component rendering instead of HTML template
		// This would typically use a component like:
		// components.GDriveAuth(c.Request.Context(), components.GDriveAuthData{
		//    AuthURL: authURL,
		//    ConfigID: configID,
		// }).Render(c.Request.Context(), c.Writer)
		// For now, we'll redirect to the configs page with the auth URL and config ID
		c.Redirect(http.StatusFound, fmt.Sprintf("/configs/%s/gdrive-auth?auth_url=%s",
			configID, url.QueryEscape(authURL)))
		return
	}

	// If we get here, something unexpected happened
	RenderErrorPage(c, "Unexpected result from rclone", string(output))
}

// HandleGoogleDriveCallback handles the manual entry of the OAuth code
func (h *Handlers) HandleGoogleDriveCallback(c *gin.Context) {
	// Get the auth code from form submission
	authCode := c.PostForm("auth_code")
	if authCode == "" {
		RenderErrorPage(c, "Missing authentication code", "")
		return
	}

	// Get the config ID from the session
	session := sessions.Default(c)
	configID := session.Get("gdrive_config_id")
	tempConfigPath := session.Get("gdrive_temp_config")

	if configID == nil || tempConfigPath == nil {
		RenderErrorPage(c, "Session expired or invalid. Please try again.", "")
		return
	}

	// Get rclone path
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	// Complete the OAuth flow with the provided code
	cmd := exec.Command(
		rclonePath,
		"config",
		"reconnect",
		"temp_gdrive:",
		"--config",
		tempConfigPath.(string),
	)

	// Create a pipe for stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		RenderErrorPage(c, "Failed to create stdin pipe", err.Error())
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		RenderErrorPage(c, "Failed to start rclone command", err.Error())
		return
	}

	// Write the auth code to stdin
	fmt.Fprintln(stdin, authCode)
	stdin.Close()

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		RenderErrorPage(c, "Failed to complete Google Drive authentication", err.Error())
		return
	}

	// Read the token from the config file
	configData, err := ioutil.ReadFile(tempConfigPath.(string))
	if err != nil {
		RenderErrorPage(c, "Failed to read token from config file", err.Error())
		return
	}

	// Extract token from config
	token := extractToken(string(configData))
	if token == "" {
		RenderErrorPage(c, "Failed to extract token from config", "")
		return
	}

	// Store the token in the database
	configIDStr := configID.(string)
	if err := h.DB.StoreGoogleDriveToken(configIDStr, token); err != nil {
		RenderErrorPage(c, "Failed to save token", err.Error())
		return
	}

	// Clean up temporary config
	os.Remove(tempConfigPath.(string))

	// Clear session data
	session.Delete("gdrive_config_id")
	session.Delete("gdrive_temp_config")
	if err := session.Save(); err != nil {
		RenderErrorPage(c, "Failed to save session", err.Error())
		return
	}

	// Redirect to the configs page
	c.Redirect(http.StatusFound, "/configs?status=gdrive_auth_success")
}

// Helper function to extract the authentication URL from rclone output
func extractAuthURL(output string) string {
	// This is a simplified version - you may need to improve the regex
	// to handle different output formats from rclone
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "http") && strings.Contains(line, "accounts.google.com") {
			// Extract the URL - this is a simplified approach
			words := strings.Fields(line)
			for _, word := range words {
				if strings.HasPrefix(word, "http") {
					return word
				}
			}
		}
	}
	return ""
}

// Helper function to extract token from rclone config
func extractToken(configData string) string {
	// Look for the token JSON in the config
	lines := strings.Split(configData, "\n")
	for _, line := range lines {
		if strings.Contains(line, "token") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
