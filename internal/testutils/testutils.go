// Package testutils provides utilities for testing the application
package testutils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/auth"
	"github.com/starfleetcptn/gomft/internal/config"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/email"
	"github.com/starfleetcptn/gomft/internal/scheduler"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SetupTestDB creates an in-memory SQLite database for testing
func SetupTestDB(t *testing.T) *db.DB {
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Drop all tables to ensure a clean database
	err = gormDB.Migrator().DropTable(
		&db.User{},
		&db.PasswordHistory{},
		&db.PasswordResetToken{},
		&db.TransferConfig{},
		&db.Job{},
		&db.JobHistory{},
		&db.FileMetadata{},
	)
	if err != nil {
		t.Logf("Warning: Failed to drop tables: %v", err)
	}

	// Initialize the database schema
	err = gormDB.AutoMigrate(
		&db.User{},
		&db.PasswordHistory{},
		&db.PasswordResetToken{},
		&db.TransferConfig{},
		&db.Job{},
		&db.JobHistory{},
		&db.FileMetadata{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return &db.DB{DB: gormDB}
}

// CreateTestUser creates a test user in the database
func CreateTestUser(t *testing.T, database *db.DB, email string, isAdmin bool) *db.User {
	// Generate hashed password using bcrypt directly
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("testpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &db.User{
		Email:              email,
		PasswordHash:       string(hashedPassword),
		IsAdmin:            isAdmin,
		LastPasswordChange: time.Now(),
	}

	if err := database.CreateUser(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

// SetupTestConfig creates a test configuration
func SetupTestConfig(t *testing.T) *config.Config {
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

// SetupTestScheduler creates a mock scheduler for testing
func SetupTestScheduler(t *testing.T) *scheduler.Scheduler {
	// In a real test, we would create a proper mock scheduler
	// For now, we return an empty scheduler
	return &scheduler.Scheduler{}
}

// SetupTestEmailService creates a mock email service for testing
func SetupTestEmailService(t *testing.T) *email.Service {
	// In a real test, we would create a proper mock email service
	// For now, we return an empty email service
	return &email.Service{}
}

// GenerateTestToken generates a JWT token for testing
func GenerateTestToken(userID uint, isAdmin bool, jwtSecret string) (string, error) {
	// In a real application, we would include email, but for testing purposes we can create a fake email
	email := "test@example.com"
	if isAdmin {
		email = "admin@example.com"
	}

	// Create token with 1 hour expiry
	expirationTime := 1 * time.Hour
	return auth.GenerateToken(userID, email, jwtSecret, expirationTime)
}
