package email

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/starfleetcptn/gomft/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConfig(enabled bool) *config.Config {
	return &config.Config{
		BaseURL: "http://localhost:8080",
		Email: config.EmailConfig{
			Enabled:     enabled,
			Host:        "smtp.example.com",
			Port:        587,
			Username:    "user",
			Password:    "password",
			FromEmail:   "noreply@example.com",
			FromName:    "GoMFT Test",
			RequireAuth: true,
			EnableTLS:   true,
			ReplyTo:     "support@example.com",
		},
	}
}

func TestNewService(t *testing.T) {
	cfg := createTestConfig(true)
	service := NewService(cfg)
	require.NotNil(t, service)
	assert.Equal(t, cfg, service.Config)
}

func TestGeneratePasswordResetEmailHTML(t *testing.T) {
	cfg := createTestConfig(true)
	service := NewService(cfg)

	testUsername := "testuser"
	testResetLink := "http://localhost:8080/reset-password?token=testtoken123"
	testAppName := "GoMFT"
	testYear := time.Now().Year()

	data := map[string]interface{}{
		"Username":     testUsername,
		"ResetLink":    testResetLink,
		"AppName":      testAppName,
		"Year":         testYear,
		"ExpiresHours": 0.25,
	}

	htmlContent, err := service.generatePasswordResetEmailHTML(data)
	require.NoError(t, err)
	require.NotEmpty(t, htmlContent)

	// Basic checks for content presence
	assert.Contains(t, htmlContent, "Reset Your Password")
	assert.Contains(t, htmlContent, fmt.Sprintf("Hello %s", testUsername))
	assert.Contains(t, htmlContent, testResetLink) // Check link appears (both in button and text)
	assert.Contains(t, htmlContent, fmt.Sprintf("href=\"%s\"", testResetLink))
	assert.Contains(t, htmlContent, fmt.Sprintf("&copy; %d %s", testYear, testAppName))
	assert.Contains(t, htmlContent, "This link will expire in 15 minutes.")

	// Test without username
	dataNoUser := map[string]interface{}{
		"ResetLink":    testResetLink,
		"AppName":      testAppName,
		"Year":         testYear,
		"ExpiresHours": 0.25,
	}
	htmlContentNoUser, err := service.generatePasswordResetEmailHTML(dataNoUser)
	require.NoError(t, err)
	assert.Contains(t, htmlContentNoUser, "Hello,") // Should just say Hello,
	assert.NotContains(t, htmlContentNoUser, fmt.Sprintf("Hello %s", testUsername))
}

func TestGenerateTestEmailHTML(t *testing.T) {
	cfg := createTestConfig(true)
	service := NewService(cfg)

	testSubject := "My Test Subject"
	testMessage := "This is the test message body."
	testAppName := "GoMFT"
	testYear := time.Now().Year()
	testCurrentTime := time.Now().Format(time.RFC1123Z) // Use the same format

	data := map[string]interface{}{
		"Subject":     testSubject,
		"Message":     testMessage,
		"AppName":     testAppName,
		"Year":        testYear,
		"SMTPServer":  cfg.Email.Host,
		"SMTPPort":    cfg.Email.Port,
		"FromEmail":   cfg.Email.FromEmail,
		"CurrentTime": testCurrentTime,
	}

	htmlContent, err := service.generateTestEmailHTML(data)
	require.NoError(t, err)
	require.NotEmpty(t, htmlContent)

	// Basic checks for content presence
	assert.Contains(t, htmlContent, fmt.Sprintf("<title>%s</title>", testSubject))
	assert.Contains(t, htmlContent, fmt.Sprintf("<h1>%s</h1>", testSubject))
	assert.Contains(t, htmlContent, testMessage)
	assert.Contains(t, htmlContent, fmt.Sprintf("%s:%d", cfg.Email.Host, cfg.Email.Port))
	assert.Contains(t, htmlContent, cfg.Email.FromEmail)
	assert.Contains(t, htmlContent, testCurrentTime)
	assert.Contains(t, htmlContent, fmt.Sprintf("&copy; %d %s", testYear, testAppName))
}

func TestSendPasswordResetEmail_Disabled(t *testing.T) {
	cfg := createTestConfig(false) // Email disabled
	service := NewService(cfg)

	toEmail := "test@example.com"
	username := "testuser"
	resetToken := "disabledtoken123"

	err := service.SendPasswordResetEmail(toEmail, username, resetToken)
	require.Error(t, err)

	expectedErrorSubstr := fmt.Sprintf("email service is disabled, reset link would be: %s/reset-password?token=%s",
		cfg.BaseURL, resetToken)
	assert.Contains(t, err.Error(), expectedErrorSubstr)
}

func TestSendTestEmail_Disabled(t *testing.T) {
	cfg := createTestConfig(false) // Email disabled
	service := NewService(cfg)

	toEmail := "test@example.com"

	err := service.SendTestEmail(toEmail, "Test Subject", "Test Message")
	require.Error(t, err)
	assert.EqualError(t, err, "email service is disabled")
}

// --- Placeholder/TODO for more complex tests ---

// TODO: TestSendPasswordResetEmail_Enabled - Requires mocking sendEmail or SMTP interactions
// TODO: TestSendTestEmail_Enabled - Requires mocking sendEmail or SMTP interactions
// TODO: TestSendEmail - Requires extensive mocking of net/smtp package

// Example structure for testing enabled path (without actual sending/mocking)
// This verifies the function prepares the correct data before calling sendEmail
func TestSendPasswordResetEmail_Enabled_DataPreparation(t *testing.T) {
	cfg := createTestConfig(true)
	service := NewService(cfg)

	// We need a way to intercept the call to sendEmail or verify its inputs
	// For now, we just check that no error occurs up to that point
	// and that the HTML generation works (implicitly tested by TestGeneratePasswordResetEmailHTML)

	toEmail := "recipient@example.com"
	username := "testuser-enabled"
	resetToken := "enabledtoken456"

	// If generatePasswordResetEmailHTML works, this call should proceed
	// without error until the actual sendEmail call (which we aren't testing here)
	// A full test would mock sendEmail and verify the arguments passed to it.
	err := service.SendPasswordResetEmail(toEmail, username, resetToken)

	// In a real scenario without mocking, this might fail if SMTP connection fails.
	// For this basic check, we assume HTML generation is the main potential failure point *before* sendEmail.
	// If TestGeneratePasswordResetEmailHTML passes, we expect no error *from generation*.
	// We cannot assert assert.NoError(t, err) reliably without mocking sendEmail.
	t.Logf("SendPasswordResetEmail (enabled) returned: %v (expected success or SMTP error)", err)
	// Asserting that the error, if any, is NOT related to template generation could be a weak check.
	if err != nil {
		assert.False(t, strings.Contains(err.Error(), "template"), "Error should be SMTP related, not template related")
	}
}

func TestSendTestEmail_Enabled_DataPreparation(t *testing.T) {
	cfg := createTestConfig(true)
	service := NewService(cfg)

	toEmail := "recipient@example.com"
	subject := "Specific Test Subject"
	message := "Specific test message."

	// Test with specific subject and message
	err := service.SendTestEmail(toEmail, subject, message)
	t.Logf("SendTestEmail (enabled, specific) returned: %v (expected success or SMTP error)", err)
	if err != nil {
		assert.False(t, strings.Contains(err.Error(), "template"), "Error should be SMTP related, not template related")
	}

	// Test with default subject and message
	errDefault := service.SendTestEmail(toEmail, "", "")
	t.Logf("SendTestEmail (enabled, default) returned: %v (expected success or SMTP error)", errDefault)
	if errDefault != nil {
		assert.False(t, strings.Contains(errDefault.Error(), "template"), "Error should be SMTP related, not template related")
	}
	// A full test would mock sendEmail and verify the subject/message passed (checking defaults).
}
