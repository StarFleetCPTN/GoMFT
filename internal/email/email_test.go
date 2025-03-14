package email

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/starfleetcptn/gomft/internal/config"
)

// Setup test configuration without using testutils (to avoid import cycles)
func setupTestConfig(t *testing.T) *config.Config {
	tempDir, err := os.MkdirTemp("", "gomft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return &config.Config{
		ServerAddress: ":9090",
		DataDir:       filepath.Join(tempDir, "data"),
		BackupDir:     filepath.Join(tempDir, "backups"),
		JWTSecret:     "test-jwt-secret",
		BaseURL:       "http://test.example.com",
		Email: config.EmailConfig{
			Enabled:     false,
			Host:        "smtp.test.com",
			Port:        587,
			Username:    "test@example.com",
			Password:    "test-password",
			FromEmail:   "test@example.com",
			FromName:    "Test",
			EnableTLS:   true,
			RequireAuth: true,
		},
	}
}

func TestEmailServiceDisabled(t *testing.T) {
	// Set up test config with email disabled
	cfg := setupTestConfig(t)
	cfg.Email.Enabled = false

	// Create the email service
	service := NewService(cfg)

	// Send a password reset email
	err := service.SendPasswordResetEmail("test@example.com", "Test User", "token123")

	// Expect an error indicating the service is disabled
	if err == nil {
		t.Error("Expected error when email service is disabled, but got none")
	}

	// Check that the error message contains the reset link
	expectedMsg := cfg.BaseURL + "/reset-password?token=token123"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain the reset link %s, got: %s", expectedMsg, err.Error())
	}
}

func TestGeneratePasswordResetEmailHTML(t *testing.T) {
	// Set up test config
	cfg := setupTestConfig(t)
	service := NewService(cfg)

	// Test cases
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected []string // Strings that should be included in the HTML
	}{
		{
			name: "Complete user data",
			data: map[string]interface{}{
				"Username":     "John Doe",
				"ResetLink":    "http://example.com/reset?token=abc123",
				"AppName":      "GoMFT",
				"Year":         2023,
				"ExpiresHours": 0.25,
			},
			expected: []string{
				"Hello John Doe",
				"http://example.com/reset?token=abc123",
				"GoMFT",
				"2023",
				"15 minutes",
			},
		},
		{
			name: "No username",
			data: map[string]interface{}{
				"ResetLink":    "http://example.com/reset?token=abc123",
				"AppName":      "GoMFT",
				"Year":         2023,
				"ExpiresHours": 0.25,
			},
			expected: []string{
				"Hello",
				"http://example.com/reset?token=abc123",
				"GoMFT",
				"2023",
				"15 minutes",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Generate HTML
			html, err := service.generatePasswordResetEmailHTML(tc.data)

			// Check for errors
			if err != nil {
				t.Fatalf("Error generating HTML: %v", err)
			}

			// Check that all expected strings are included
			for _, expected := range tc.expected {
				if !strings.Contains(html, expected) {
					t.Errorf("Expected HTML to contain %q, but it doesn't", expected)
				}
			}
		})
	}
}
