package handlers

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/email"
	"github.com/starfleetcptn/gomft/internal/scheduler"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Static counter to ensure unique emails for each test
var testEmailCounter int = 0

func setupTestHandlers(t *testing.T) (*Handlers, *gin.Engine) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test DB
	testDB := setupTestDB(t)

	// Create a mock scheduler
	mockScheduler := &scheduler.Scheduler{}

	// Create a mock email service
	mockEmailService := &email.Service{}

	// Create test handlers
	handlers := NewHandlers(
		testDB,
		mockScheduler,
		"test-jwt-secret",
		"test-db-path",
		"test-backup-dir",
		"test-logs-dir",
		mockEmailService,
	)

	// Create a test router
	router := gin.New()

	return handlers, router
}

// setupTestDB creates a test database for handler tests
func setupTestDB(t *testing.T) *db.DB {
	// Set up an in-memory SQLite DB
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Run migrations
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

	// Create a test admin user with a unique email
	testEmailCounter++
	testEmail := fmt.Sprintf("test%d@example.com", testEmailCounter)

	// Generate a hashed password for "admin"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	admin := db.User{
		Email:        testEmail,
		PasswordHash: string(hashedPassword),
	}
	admin.SetIsAdmin(true)

	if err := gormDB.Create(&admin).Error; err != nil {
		t.Fatalf("Failed to create test admin user: %v", err)
	}

	return &db.DB{DB: gormDB}
}
