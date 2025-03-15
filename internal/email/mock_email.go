package email

import (
	"fmt"

	"github.com/starfleetcptn/gomft/internal/config"
)

// MockService implements the email Service for testing purposes
type MockService struct {
	SendEmailCalls              int
	SendPasswordResetEmailCalls int
	ReturnError                 error
}

// NewMockService creates a new mock email service
func NewMockService() *Service {
	// Create minimal config
	cfg := &config.Config{
		Email: config.EmailConfig{
			Enabled: false,
		},
		BaseURL: "http://localhost:8080",
	}

	return &Service{
		Config: cfg,
	}
}

// SendPasswordResetEmail mocks sending a password reset email
func (s *MockService) SendPasswordResetEmail(toEmail, username, resetToken string) error {
	return fmt.Errorf("email service is disabled, reset link would be: %s/reset-password?token=%s",
		"http://localhost:8080", resetToken)
}
