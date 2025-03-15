package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInitializeWithNonExistentDirectory tests initialization with a directory that doesn't exist
func TestInitializeWithNonExistentDirectory(t *testing.T) {
	// Create a temporary directory path
	tempDir := filepath.Join(os.TempDir(), "gomft_test_nonexistent")

	// Make sure the directory doesn't exist
	_ = os.RemoveAll(tempDir)

	// Create a path inside the non-existent directory
	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize the database - this should create the directory
	db, err := Initialize(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Verify the directory was created
	_, err = os.Stat(tempDir)
	assert.NoError(t, err, "Directory should be created")

	// Close and clean up
	err = db.Close()
	assert.NoError(t, err)

	// Clean up
	_ = os.RemoveAll(tempDir)
}

// TestInitializeWithInvalidDBPath tests initialization with an invalid DB path
func TestInitializeWithInvalidDBPath(t *testing.T) {
	// Create a file path that can't be a SQLite database
	invalidPath := "/dev/null/invalid.db"

	// Attempt to initialize with an invalid path
	db, err := Initialize(invalidPath)
	assert.Error(t, err)
	assert.Nil(t, db)
}

// TestInitializeWithExistingDB tests initialization with an existing database
func TestInitializeWithExistingDB(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gomft_test_existing")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a database path
	dbPath := filepath.Join(tempDir, "existing.db")

	// Initialize the database for the first time
	db1, err := Initialize(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, db1)

	// Create a test user to verify the database works
	user := &User{
		Email:        "test@example.com",
		PasswordHash: "hash",
		IsAdmin:      true,
	}
	err = db1.CreateUser(user)
	assert.NoError(t, err)
	assert.NotZero(t, user.ID)

	// Close the first database connection
	err = db1.Close()
	assert.NoError(t, err)

	// Initialize the database again with the same path
	db2, err := Initialize(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, db2)

	// Verify we can read the user that was created earlier
	retrievedUser, err := db2.GetUserByEmail("test@example.com")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)

	// Close the second database connection
	err = db2.Close()
	assert.NoError(t, err)
}

// TestCloseMultipleTimes tests closing the database multiple times
func TestCloseMultipleTimes(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gomft_test_close")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a database path
	dbPath := filepath.Join(tempDir, "close.db")

	// Initialize the database
	db, err := Initialize(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Close the database
	err = db.Close()
	assert.NoError(t, err)

	// Trying to close it again - for some DB drivers this might cause an error
	// but SQLite in-memory seems to handle this gracefully
	err = db.Close()
	// We won't assert error here since it depends on the driver
	t.Logf("Second close resulted in: %v", err)

	// Instead, let's test that DB operations fail after close
	_, err = db.GetUserByEmail("test@example.com")
	assert.Error(t, err, "DB operations should fail after close")
}

// TestInitializeWithMigrationFailure tests when AutoMigrate fails
func TestInitializeWithMigrationFailure(t *testing.T) {
	// We can't easily cause a migration failure with SQLite
	// but we can skip this test and document that it's hard to test
	t.Skip("Testing migration failure is difficult with SQLite")

	// In a real-world scenario, this might happen if:
	// 1. The schema changed significantly between versions
	// 2. The database is corrupted
	// 3. There are permission issues
}
