package db

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var testDB *gorm.DB

// setupTestDB sets up a SQLite in-memory database for testing
func setupTestDB(t *testing.T) *DB {
	var err error
	testDB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite database: %v", err)
	}

	// Create a minimal TransferConfig struct for testing
	type TransferConfig struct {
		ID                    uint `gorm:"primarykey"`
		SourceProviderID      uint `gorm:"index"`
		DestinationProviderID uint `gorm:"index"`
	}

	// Create the necessary tables
	err = testDB.AutoMigrate(&StorageProvider{}, &TransferConfig{})
	if err != nil {
		t.Fatalf("Failed to migrate tables: %v", err)
	}

	return &DB{DB: testDB}
}

// cleanupTestDB cleans up the test database after each test
func cleanupTestDB(t *testing.T) {
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("Failed to get SQL DB: %v", err)
	}
	sqlDB.Close()
}

// TestStorageProviderCRUD tests the complete CRUD cycle for a StorageProvider
func TestStorageProviderCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t)

	// Create a test provider
	provider := &StorageProvider{
		Name:      "Test SFTP",
		Type:      ProviderTypeSFTP,
		Host:      "example.com",
		Port:      22,
		Username:  "user",
		Password:  "password",
		CreatedBy: 1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test Create
	err := db.CreateStorageProvider(provider)
	if err != nil {
		t.Fatalf("Failed to create storage provider: %v", err)
	}
	if provider.ID == 0 {
		t.Fatal("Expected provider ID to be set after creation")
	}

	// Test Get
	retrievedProvider, err := db.GetStorageProvider(provider.ID)
	if err != nil {
		t.Fatalf("Failed to get storage provider: %v", err)
	}
	if retrievedProvider.ID != provider.ID {
		t.Errorf("Expected provider ID %d, got %d", provider.ID, retrievedProvider.ID)
	}
	if retrievedProvider.Name != "Test SFTP" {
		t.Errorf("Expected name 'Test SFTP', got '%s'", retrievedProvider.Name)
	}
	if retrievedProvider.Type != ProviderTypeSFTP {
		t.Errorf("Expected type '%s', got '%s'", ProviderTypeSFTP, retrievedProvider.Type)
	}

	// Test Update
	retrievedProvider.Name = "Updated SFTP"
	retrievedProvider.Host = "updated.example.com"
	// Make sure we keep the required fields for validation
	retrievedProvider.Port = 22
	retrievedProvider.Username = "user"
	retrievedProvider.Password = "password"
	err = db.UpdateStorageProvider(retrievedProvider)
	if err != nil {
		t.Fatalf("Failed to update storage provider: %v", err)
	}

	// Verify update
	updatedProvider, err := db.GetStorageProvider(provider.ID)
	if err != nil {
		t.Fatalf("Failed to get updated storage provider: %v", err)
	}
	if updatedProvider.Name != "Updated SFTP" {
		t.Errorf("Expected updated name 'Updated SFTP', got '%s'", updatedProvider.Name)
	}
	if updatedProvider.Host != "updated.example.com" {
		t.Errorf("Expected updated host 'updated.example.com', got '%s'", updatedProvider.Host)
	}

	// Test Delete
	err = db.DeleteStorageProvider(provider.ID)
	if err != nil {
		t.Fatalf("Failed to delete storage provider: %v", err)
	}

	// Verify deletion
	_, err = db.GetStorageProvider(provider.ID)
	if err == nil {
		t.Error("Expected error when getting deleted provider, got nil")
	}
}

// TestStorageProviderGetAll tests retrieving all storage providers for a user
func TestStorageProviderGetAll(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t)

	// Create multiple providers for the same user
	providers := []*StorageProvider{
		{
			Name:      "SFTP Provider",
			Type:      ProviderTypeSFTP,
			Host:      "sftp.example.com",
			Port:      22,
			Username:  "sftpuser",
			Password:  "pass",
			CreatedBy: 1,
		},
		{
			Name:      "S3 Provider",
			Type:      ProviderTypeS3,
			AccessKey: "accesskey",
			SecretKey: "secretkey",
			Region:    "us-west-1",
			CreatedBy: 1,
		},
		{
			Name:         "OneDrive Provider",
			Type:         ProviderTypeOneDrive,
			ClientID:     "clientid",
			ClientSecret: "clientsecret",
			CreatedBy:    1,
		},
		{
			Name:      "Another User's Provider",
			Type:      ProviderTypeSFTP,
			Host:      "other.example.com",
			Port:      22,          // Added required port for SFTP
			Username:  "otheruser", // Added required username for SFTP
			Password:  "otherpass", // Added required password for SFTP
			CreatedBy: 2,           // Different user
		},
	}

	// Create all providers
	for _, p := range providers {
		err := db.CreateStorageProvider(p)
		if err != nil {
			t.Fatalf("Failed to create provider %s: %v", p.Name, err)
		}
	}

	// Test GetStorageProviders
	userProviders, err := db.GetStorageProviders(1)
	if err != nil {
		t.Fatalf("Failed to get storage providers: %v", err)
	}

	// Check the results
	if len(userProviders) != 3 {
		t.Errorf("Expected 3 providers for user 1, got %d", len(userProviders))
	}
}

// TestStorageProviderGetByType tests retrieving storage providers by type
func TestStorageProviderGetByType(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t)

	// Create providers of different types
	providers := []*StorageProvider{
		{
			Name:      "SFTP Provider 1",
			Type:      ProviderTypeSFTP,
			Host:      "sftp1.example.com",
			Port:      22,      // Added required port for SFTP
			Username:  "user1", // Added required username for SFTP
			Password:  "pass1", // Added required password for SFTP
			CreatedBy: 1,
		},
		{
			Name:      "SFTP Provider 2",
			Type:      ProviderTypeSFTP,
			Host:      "sftp2.example.com",
			Port:      22,      // Added required port for SFTP
			Username:  "user2", // Added required username for SFTP
			Password:  "pass2", // Added required password for SFTP
			CreatedBy: 1,
		},
		{
			Name:      "S3 Provider",
			Type:      ProviderTypeS3,
			AccessKey: "accesskey",
			SecretKey: "secretkey", // Added required secret key for S3
			Region:    "us-west-1", // Added required region for S3
			CreatedBy: 1,
		},
	}

	// Create all providers
	for _, p := range providers {
		err := db.CreateStorageProvider(p)
		if err != nil {
			t.Fatalf("Failed to create provider %s: %v", p.Name, err)
		}
	}

	// Test GetStorageProvidersByType
	sftpProviders, err := db.GetStorageProvidersByType(1, ProviderTypeSFTP)
	if err != nil {
		t.Fatalf("Failed to get SFTP providers: %v", err)
	}

	// Check the results
	if len(sftpProviders) != 2 {
		t.Errorf("Expected 2 SFTP providers, got %d", len(sftpProviders))
	}
	for _, p := range sftpProviders {
		if p.Type != ProviderTypeSFTP {
			t.Errorf("Expected provider type SFTP, got %s", p.Type)
		}
	}
}

// TestStorageProviderValidationOnSave tests that validation is called before saving
func TestStorageProviderValidationOnSave(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t)

	// Create a provider with invalid data (missing host for SFTP)
	invalidProvider := &StorageProvider{
		Name:      "Invalid SFTP",
		Type:      ProviderTypeSFTP,
		Port:      22,
		Username:  "user",
		Password:  "pass",
		CreatedBy: 1,
	}

	// Test CreateStorageProvider with validation
	err := db.CreateStorageProvider(invalidProvider)
	if err == nil {
		t.Fatal("Expected validation error for invalid provider, got nil")
	}
}

// TestStorageProviderCount tests counting providers for a user
func TestStorageProviderCount(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t)

	// Create multiple providers for different users
	providers := []*StorageProvider{
		{
			Name:      "User 1 Provider 1",
			Type:      ProviderTypeSFTP,
			Host:      "host1.example.com",
			Port:      22,      // Added required port for SFTP
			Username:  "user1", // Added required username for SFTP
			Password:  "pass1", // Added required password for SFTP
			CreatedBy: 1,
		},
		{
			Name:      "User 1 Provider 2",
			Type:      ProviderTypeS3,
			AccessKey: "accesskey",
			SecretKey: "secretkey", // Added required secret key for S3
			Region:    "us-west-1", // Added required region for S3
			CreatedBy: 1,
		},
		{
			Name:      "User 2 Provider",
			Type:      ProviderTypeSFTP,
			Host:      "host2.example.com",
			Port:      22,      // Added required port for SFTP
			Username:  "user2", // Added required username for SFTP
			Password:  "pass2", // Added required password for SFTP
			CreatedBy: 2,
		},
	}

	// Create all providers
	for _, p := range providers {
		// Skip validation for this test since we're just testing count
		err := testDB.Create(p).Error
		if err != nil {
			t.Fatalf("Failed to create provider %s: %v", p.Name, err)
		}
	}

	// Test CountStorageProviders
	count, err := db.CountStorageProviders(1)
	if err != nil {
		t.Fatalf("Failed to count storage providers: %v", err)
	}

	// Check the result
	if count != 2 {
		t.Errorf("Expected count 2 for user 1, got %d", count)
	}

	count, err = db.CountStorageProviders(2)
	if err != nil {
		t.Fatalf("Failed to count storage providers: %v", err)
	}

	// Check the result
	if count != 1 {
		t.Errorf("Expected count 1 for user 2, got %d", count)
	}
}

// TestHelperMethods tests the helper methods on StorageProvider
func TestHelperMethods(t *testing.T) {
	// Test GetPassiveMode and SetPassiveMode
	t.Run("PassiveMode", func(t *testing.T) {
		provider := &StorageProvider{}

		// Default value
		if !provider.GetPassiveMode() {
			t.Error("Expected default PassiveMode to be true")
		}

		// Set to false
		provider.SetPassiveMode(false)
		if provider.GetPassiveMode() {
			t.Error("Expected PassiveMode to be false after setting")
		}

		// Set to true
		provider.SetPassiveMode(true)
		if !provider.GetPassiveMode() {
			t.Error("Expected PassiveMode to be true after setting")
		}
	})

	// Test GetReadOnly and SetReadOnly
	t.Run("ReadOnly", func(t *testing.T) {
		provider := &StorageProvider{}

		// Default value
		if provider.GetReadOnly() {
			t.Error("Expected default ReadOnly to be false")
		}

		// Set to true
		provider.SetReadOnly(true)
		if !provider.GetReadOnly() {
			t.Error("Expected ReadOnly to be true after setting")
		}
	})

	// Test GetAuthenticated and SetAuthenticated
	t.Run("Authenticated", func(t *testing.T) {
		provider := &StorageProvider{}

		// Default value
		if provider.GetAuthenticated() {
			t.Error("Expected default Authenticated to be false")
		}

		// Set to true
		provider.SetAuthenticated(true)
		if !provider.GetAuthenticated() {
			t.Error("Expected Authenticated to be true after setting")
		}
	})
}

// TestIsOAuthProvider tests the IsOAuthProvider method
func TestIsOAuthProvider(t *testing.T) {
	tests := []struct {
		providerType StorageProviderType
		isOAuth      bool
	}{
		{ProviderTypeSFTP, false},
		{ProviderTypeS3, false},
		{ProviderTypeFTP, false},
		{ProviderTypeSMB, false},
		{ProviderTypeOneDrive, true},
		{ProviderTypeGoogleDrive, true},
		{ProviderTypeGooglePhoto, true},
		{ProviderTypeLocal, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			provider := &StorageProvider{Type: tt.providerType}
			if provider.IsOAuthProvider() != tt.isOAuth {
				t.Errorf("IsOAuthProvider() for %s = %v, want %v", tt.providerType, provider.IsOAuthProvider(), tt.isOAuth)
			}
		})
	}
}
