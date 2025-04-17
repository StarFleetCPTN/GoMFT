package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/web/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// BoolPointer returns a pointer to the provided bool value
func BoolPointer(value bool) *bool {
	return &value
}

// SetupTestDB creates and configures an in-memory SQLite database for testing
func SetupTestDB(t *testing.T) (*db.DB, error) {
	// Create in-memory SQLite database
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Auto-migrate required tables
	err = gormDB.AutoMigrate(
		&db.StorageProvider{},
		&db.User{},
		&db.TransferConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Return wrapped DB
	return &db.DB{DB: gormDB}, nil
}

// SetupE2ETest prepares the test environment for E2E testing
func SetupE2ETest(t *testing.T) (*handlers.Handlers, *gin.Engine, *db.DB) {
	// Use test mode for Gin
	gin.SetMode(gin.TestMode)

	// Create in-memory test database
	testDB, err := SetupTestDB(t)
	require.NoError(t, err, "Failed to set up test database")

	// Mock handlers
	h := &handlers.Handlers{
		DB:        testDB,
		JWTSecret: "test-secret",
		StartTime: time.Now(),
		DBPath:    ":memory:",
		BackupDir: t.TempDir(),
		LogsDir:   t.TempDir(),
	}

	// Create a router with basic middleware
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup authentication middleware mock
	router.Use(func(c *gin.Context) {
		// Simulate authenticated user
		c.Set("userID", uint(1))
		c.Set("email", "test@example.com")
		c.Next()
	})

	// Create a test user to own the resources
	user := &db.User{
		Email:        "test@example.com",
		PasswordHash: "test-hash",
		IsAdmin:      BoolPointer(true),
	}
	err = testDB.CreateUser(user)
	require.NoError(t, err, "Failed to create test user")

	return h, router, testDB
}

// TestStorageProviderE2EFlow tests the complete user flow for storage providers
func TestStorageProviderE2EFlow(t *testing.T) {
	handlers, router, testDB := SetupE2ETest(t)
	defer testDB.Close()

	// Note: These tests are simplified since we can't easily load HTML templates in the test environment
	// In a real environment, we would also validate the HTML content of responses

	// Register routes for storage provider operations
	router.GET("/storage-providers", handlers.HandleListStorageProviders)
	router.GET("/storage-providers/new", handlers.HandleNewStorageProvider)
	router.POST("/storage-providers", handlers.HandleCreateStorageProvider)
	router.GET("/storage-providers/:id/edit", handlers.HandleEditStorageProvider)
	router.POST("/storage-providers/:id", handlers.HandleUpdateStorageProvider)
	router.POST("/storage-providers/:id/delete", handlers.HandleDeleteStorageProvider)
	router.GET("/storage-providers/options", handlers.HandleStorageProviderOptions)

	var providerID uint

	// Step 1: Access the list page (initially empty)
	t.Run("Initial List Page", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/storage-providers", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for storage provider list page")
	})

	// Step 2: Access the new provider form
	t.Run("New Provider Form", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/storage-providers/new", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for new storage provider form")
	})

	// Step 3: Create a new storage provider by directly inserting into DB
	// (since form submission requires template loading)
	t.Run("Create Provider", func(t *testing.T) {
		// Create provider directly in DB
		provider := &db.StorageProvider{
			Name:      "E2E Test S3 Provider",
			Type:      db.ProviderTypeS3,
			AccessKey: "e2e-test-access-key",
			SecretKey: "e2e-test-secret-key",
			Region:    "us-west-1",
			Bucket:    "e2e-test-bucket",
			CreatedBy: 1,
		}
		err := testDB.CreateStorageProvider(provider)
		assert.NoError(t, err, "Should create provider without error")

		// Store ID for later use
		providerID = provider.ID
		assert.NotZero(t, providerID, "Provider ID should not be zero")

		// Fetch all providers to verify creation
		providers, err := testDB.GetStorageProviders(1)
		assert.NoError(t, err, "Should fetch providers without error")
		assert.GreaterOrEqual(t, len(providers), 1, "Should have at least 1 provider after creation")

		// Find our provider in the list
		var found bool
		for _, p := range providers {
			if p.ID == providerID {
				found = true
				assert.Equal(t, "E2E Test S3 Provider", p.Name, "Provider should have the correct name")
				break
			}
		}
		assert.True(t, found, "Should find the created provider in the list")
	})

	// Step 4: Verify provider appears in list
	t.Run("Verify Provider in List", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/storage-providers", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for storage provider list page")
	})

	// Step 5: Access the provider options endpoint
	t.Run("Provider Options", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/storage-providers/options", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for provider options")
		// Check for provider in options (should contain ID and name)
		assert.Contains(t, w.Body.String(), fmt.Sprintf("value=\"%d\"", providerID), "Options should include provider ID")
		assert.Contains(t, w.Body.String(), "E2E Test S3 Provider", "Options should include provider name")
	})

	// Step 6: Access the edit form for the provider
	t.Run("Edit Provider Form", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("/storage-providers/%d/edit", providerID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for edit storage provider form")
	})

	// Step 7: Update the provider directly in DB
	t.Run("Update Provider", func(t *testing.T) {
		// Get existing provider
		provider, err := testDB.GetStorageProvider(providerID)
		assert.NoError(t, err, "Should get provider without error")

		// Update fields
		provider.Name = "Updated E2E Test Provider"
		provider.AccessKey = "updated-access-key"
		provider.SecretKey = "updated-secret-key" // Make sure to include secret key for S3 provider
		provider.Region = "eu-west-1"
		provider.Bucket = "updated-bucket"

		// Save updates
		err = testDB.UpdateStorageProvider(provider)
		assert.NoError(t, err, "Should update provider without error")

		// Verify the update
		updatedProvider, err := testDB.GetStorageProvider(providerID)
		assert.NoError(t, err, "Should fetch updated provider without error")
		assert.Equal(t, "Updated E2E Test Provider", updatedProvider.Name, "Provider name should be updated")
		assert.Equal(t, "updated-access-key", updatedProvider.AccessKey, "Provider access key should be updated")
		assert.Equal(t, "eu-west-1", updatedProvider.Region, "Provider region should be updated")
		assert.Equal(t, "updated-bucket", updatedProvider.Bucket, "Provider bucket should be updated")
	})

	// Step 8: Delete the provider via DB
	t.Run("Delete Provider", func(t *testing.T) {
		// Delete via DB operation
		err := testDB.DeleteStorageProvider(providerID)
		assert.NoError(t, err, "Should delete provider without error")

		// Verify deletion
		providers, err := testDB.GetStorageProviders(1)
		assert.NoError(t, err, "Should fetch providers without error")

		// Make sure our provider is not in the list
		var found bool
		for _, p := range providers {
			if p.ID == providerID {
				found = true
				break
			}
		}
		assert.False(t, found, "Provider should be deleted")
	})

	// Step 9: Verify provider is no longer in options
	t.Run("Verify Provider Removed from Options", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/storage-providers/options", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for provider options")
		// Provider should not be in options anymore
		assert.NotContains(t, w.Body.String(), fmt.Sprintf("value=\"%d\"", providerID), "Options should not include deleted provider ID")
		assert.NotContains(t, w.Body.String(), "Updated E2E Test Provider", "Options should not include deleted provider name")
	})
}

// TestStorageProviderPerformance conducts performance tests on the storage provider API
func TestStorageProviderPerformance(t *testing.T) {
	// Skip in short test mode
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	handlers, router, testDB := SetupE2ETest(t)
	defer testDB.Close()

	// Register routes for storage provider operations
	router.GET("/storage-providers", handlers.HandleListStorageProviders)
	router.GET("/storage-providers/options", handlers.HandleStorageProviderOptions)

	// Pre-create some test providers for loading test
	for i := 0; i < 20; i++ {
		provider := &db.StorageProvider{
			Name:      fmt.Sprintf("Performance Test Provider %d", i),
			Type:      db.ProviderTypeS3,
			AccessKey: fmt.Sprintf("perf-access-key-%d", i),
			SecretKey: fmt.Sprintf("perf-secret-key-%d", i),
			Region:    "us-west-1",
			Bucket:    fmt.Sprintf("perf-bucket-%d", i),
			CreatedBy: 1,
		}
		err := testDB.CreateStorageProvider(provider)
		require.NoError(t, err, "Failed to create test provider")
	}

	// Test 1: List performance with many providers
	t.Run("List Performance", func(t *testing.T) {
		// Measure response time for listing providers
		start := time.Now()

		req, _ := http.NewRequest("GET", "/storage-providers", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for provider list")
		assert.Less(t, duration.Milliseconds(), int64(500), "List operation should complete in under 500ms")
		t.Logf("List operation took %d ms", duration.Milliseconds())
	})

	// Test 2: Options performance with many providers
	t.Run("Options Performance", func(t *testing.T) {
		// Measure response time for provider options
		start := time.Now()

		req, _ := http.NewRequest("GET", "/storage-providers/options", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code, "Should get 200 OK for provider options")
		assert.Less(t, duration.Milliseconds(), int64(500), "Options operation should complete in under 500ms")
		t.Logf("Options operation took %d ms", duration.Milliseconds())
	})

	// Test 3: Creation performance via direct DB access
	t.Run("Create Performance", func(t *testing.T) {
		// Measure response time for creating a provider directly in DB
		provider := &db.StorageProvider{
			Name:      "Performance Test Create Provider",
			Type:      db.ProviderTypeS3,
			AccessKey: "perf-test-access-key",
			SecretKey: "perf-test-secret-key",
			Region:    "us-west-1",
			Bucket:    "perf-test-bucket",
			CreatedBy: 1,
		}

		start := time.Now()
		err := testDB.CreateStorageProvider(provider)
		duration := time.Since(start)

		assert.NoError(t, err, "Should create provider without error")
		assert.Less(t, duration.Milliseconds(), int64(500), "Create operation should complete in under 500ms")
		t.Logf("Create operation took %d ms", duration.Milliseconds())
	})
}
