package db_test

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// setupTransferConfigTestDB sets up a SQLite in-memory database for testing
func setupTransferConfigTestDB(t *testing.T) *db.DB {
	testDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite database: %v", err)
	}

	// Create tables
	err = testDB.AutoMigrate(&db.StorageProvider{}, &db.TransferConfig{}, &db.User{})
	if err != nil {
		t.Fatalf("Failed to migrate tables: %v", err)
	}

	// Create test user
	user := &db.User{
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
	}
	err = testDB.Create(user).Error
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return &db.DB{DB: testDB}
}

// cleanupTransferConfigTestDB cleans up the test database
func cleanupTransferConfigTestDB(t *testing.T, testDB *gorm.DB) {
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("Failed to get SQL DB: %v", err)
	}
	sqlDB.Close()
}

// TestTransferConfigWithProviderReferences tests the TransferConfig with StorageProvider references
func TestTransferConfigWithProviderReferences(t *testing.T) {
	testDB := setupTransferConfigTestDB(t)
	defer cleanupTransferConfigTestDB(t, testDB.DB)

	// Create test storage providers
	sourceProvider := &db.StorageProvider{
		Name:              "Test Source SFTP",
		Type:              db.ProviderTypeSFTP,
		Host:              "source.example.com",
		Port:              22,
		Username:          "sourceuser",
		EncryptedPassword: "encrypted_password_source",
		CreatedBy:         1,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	destProvider := &db.StorageProvider{
		Name:               "Test Destination S3",
		Type:               db.ProviderTypeS3,
		AccessKey:          "destkey",
		EncryptedSecretKey: "encrypted_secret_key_dest",
		Region:             "us-west-1",
		CreatedBy:          1,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Save providers to database
	err := testDB.CreateStorageProvider(sourceProvider)
	assert.NoError(t, err, "Failed to create source provider")

	err = testDB.CreateStorageProvider(destProvider)
	assert.NoError(t, err, "Failed to create destination provider")

	// Create a transfer config with provider references
	config := &db.TransferConfig{
		Name:            "Test Config with Provider References",
		SourcePath:      "/source/path",
		DestinationPath: "/dest/path",
		CreatedBy:       1,
		SourceType:      string(db.ProviderTypeSFTP), // Set for compatibility
		DestinationType: string(db.ProviderTypeS3),   // Set for compatibility
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Set provider references
	config.SetSourceProvider(sourceProvider)
	config.SetDestinationProvider(destProvider)

	// Save to database
	err = testDB.Create(config).Error
	assert.NoError(t, err, "Failed to create transfer config")

	// Test IsUsingProviderReferences methods
	assert.True(t, config.IsUsingSourceProviderReference(), "Should be using source provider reference")
	assert.True(t, config.IsUsingDestinationProviderReference(), "Should be using destination provider reference")
	assert.True(t, config.IsUsingProviderReferences(), "Should be using provider references")

	// Clear providers to test loading from DB
	config.SourceProvider = nil
	config.DestinationProvider = nil

	// Test GetSourceCredentials
	sourceCreds, err := config.GetSourceCredentials(testDB)
	assert.NoError(t, err, "Failed to get source credentials")
	assert.Equal(t, "source.example.com", sourceCreds["host"], "Source host mismatch")
	assert.Equal(t, 22, sourceCreds["port"], "Source port mismatch")
	assert.Equal(t, "sourceuser", sourceCreds["username"], "Source username mismatch")
	assert.Equal(t, "encrypted_password_source", sourceCreds["encrypted_password"], "Source encrypted password mismatch")

	// Test GetDestinationCredentials
	destCreds, err := config.GetDestinationCredentials(testDB)
	assert.NoError(t, err, "Failed to get destination credentials")
	assert.Equal(t, "destkey", destCreds["access_key"], "Destination access key mismatch")
	assert.Equal(t, "encrypted_secret_key_dest", destCreds["encrypted_secret_key"], "Destination encrypted secret key mismatch")
	assert.Equal(t, "us-west-1", destCreds["region"], "Destination region mismatch")

	// Test that providers were loaded
	assert.NotNil(t, config.SourceProvider, "Source provider should be loaded")
	assert.NotNil(t, config.DestinationProvider, "Destination provider should be loaded")
}

// TestTransferConfigWithoutProviderReferences tests the TransferConfig without StorageProvider references
func TestTransferConfigWithoutProviderReferences(t *testing.T) {
	testDB := setupTransferConfigTestDB(t)
	defer cleanupTransferConfigTestDB(t, testDB.DB)

	// Create a transfer config without provider references (legacy mode)
	config := &db.TransferConfig{
		Name:            "Test Config without Provider References",
		SourceType:      string(db.ProviderTypeSFTP),
		SourceHost:      "direct.example.com",
		SourcePort:      2222,
		SourceUser:      "directuser",
		SourcePassword:  "directpass", // This would be in form only
		SourcePath:      "/direct/source",
		DestinationType: string(db.ProviderTypeS3),
		DestAccessKey:   "directaccesskey",
		DestSecretKey:   "directsecretkey", // This would be in form only
		DestRegion:      "eu-central-1",
		DestinationPath: "/direct/dest",
		CreatedBy:       1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Save to database
	err := testDB.Create(config).Error
	assert.NoError(t, err, "Failed to create direct transfer config")

	// Test IsUsingProviderReferences methods
	assert.False(t, config.IsUsingSourceProviderReference(), "Should not be using source provider reference")
	assert.False(t, config.IsUsingDestinationProviderReference(), "Should not be using destination provider reference")
	assert.False(t, config.IsUsingProviderReferences(), "Should not be using provider references")

	// Test GetSourceCredentials
	sourceCreds, err := config.GetSourceCredentials(testDB)
	assert.NoError(t, err, "Failed to get direct source credentials")
	assert.Equal(t, "direct.example.com", sourceCreds["host"], "Direct source host mismatch")
	assert.Equal(t, 2222, sourceCreds["port"], "Direct source port mismatch")
	assert.Equal(t, "directuser", sourceCreds["username"], "Direct source username mismatch")
	assert.Equal(t, "directpass", sourceCreds["password"], "Direct source password mismatch")

	// Test GetDestinationCredentials
	destCreds, err := config.GetDestinationCredentials(testDB)
	assert.NoError(t, err, "Failed to get direct destination credentials")
	assert.Equal(t, "directaccesskey", destCreds["access_key"], "Direct destination access key mismatch")
	assert.Equal(t, "directsecretkey", destCreds["secret_key"], "Direct destination secret key mismatch")
	assert.Equal(t, "eu-central-1", destCreds["region"], "Direct destination region mismatch")
}

// TestTransferConfigMixedProviderReferences tests TransferConfig with mixed provider references
func TestTransferConfigMixedProviderReferences(t *testing.T) {
	testDB := setupTransferConfigTestDB(t)
	defer cleanupTransferConfigTestDB(t, testDB.DB)

	// Create test storage provider for source only
	sourceProvider := &db.StorageProvider{
		Name:              "Test Mixed Source",
		Type:              db.ProviderTypeFTP,
		Host:              "mixed-source.example.com",
		Port:              21,
		Username:          "mixeduser",
		EncryptedPassword: "encrypted_password_mixed",
		CreatedBy:         1,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Save provider to database
	err := testDB.CreateStorageProvider(sourceProvider)
	assert.NoError(t, err, "Failed to create mixed source provider")

	// Create a transfer config with mixed provider references
	config := &db.TransferConfig{
		Name:            "Test Config with Mixed Provider References",
		SourcePath:      "/mixed/source",
		DestinationType: string(db.ProviderTypeS3),
		DestAccessKey:   "mixedaccesskey",
		DestSecretKey:   "mixedsecretkey", // This would be in form only
		DestRegion:      "ap-northeast-1",
		DestinationPath: "/mixed/dest",
		CreatedBy:       1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Set source provider reference only
	config.SetSourceProvider(sourceProvider)

	// Save to database
	err = testDB.Create(config).Error
	assert.NoError(t, err, "Failed to create mixed transfer config")

	// Test reference methods
	assert.True(t, config.IsUsingSourceProviderReference(), "Should be using source provider reference")
	assert.False(t, config.IsUsingDestinationProviderReference(), "Should not be using destination provider reference")
	assert.False(t, config.IsUsingProviderReferences(), "Should not be using both provider references")

	// Clear provider to test loading from DB
	config.SourceProvider = nil

	// Test GetSourceCredentials
	sourceCreds, err := config.GetSourceCredentials(testDB)
	assert.NoError(t, err, "Failed to get mixed source credentials")
	assert.Equal(t, "mixed-source.example.com", sourceCreds["host"], "Mixed source host mismatch")
	assert.Equal(t, 21, sourceCreds["port"], "Mixed source port mismatch")
	assert.Equal(t, "mixeduser", sourceCreds["username"], "Mixed source username mismatch")
	assert.Equal(t, "encrypted_password_mixed", sourceCreds["encrypted_password"], "Mixed source encrypted password mismatch")

	// Test GetDestinationCredentials
	destCreds, err := config.GetDestinationCredentials(testDB)
	assert.NoError(t, err, "Failed to get mixed destination credentials")
	assert.Equal(t, "mixedaccesskey", destCreds["access_key"], "Mixed destination access key mismatch")
	assert.Equal(t, "mixedsecretkey", destCreds["secret_key"], "Mixed destination secret key mismatch")
	assert.Equal(t, "ap-northeast-1", destCreds["region"], "Mixed destination region mismatch")

	// Test that source provider was loaded
	assert.NotNil(t, config.SourceProvider, "Source provider should be loaded")
}

// TestTransferConfigNonExistentProviderReferences tests error handling for non-existent provider references
func TestTransferConfigNonExistentProviderReferences(t *testing.T) {
	testDB := setupTransferConfigTestDB(t)
	defer cleanupTransferConfigTestDB(t, testDB.DB)

	// Create uint pointers for provider IDs
	sourceProviderID := uint(999)
	destProviderID := uint(888)

	// Create a transfer config with references to non-existent providers
	config := &db.TransferConfig{
		Name:                  "Test Config with Non-existent Provider References",
		SourcePath:            "/source/path",
		DestinationPath:       "/dest/path",
		CreatedBy:             1,
		SourceType:            string(db.ProviderTypeSFTP),
		DestinationType:       string(db.ProviderTypeS3),
		SourceProviderID:      &sourceProviderID, // Use pointer to uint
		DestinationProviderID: &destProviderID,   // Use pointer to uint
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	// Save to database
	err := testDB.Create(config).Error
	assert.NoError(t, err, "Failed to create transfer config with non-existent provider references")

	// Test GetSourceCredentials - should return error for non-existent provider
	sourceCreds, err := config.GetSourceCredentials(testDB)
	assert.Error(t, err, "Should get error for non-existent source provider")
	assert.Nil(t, sourceCreds, "Source credentials should be nil for non-existent provider")
	assert.Contains(t, err.Error(), "record not found", "Error should mention record not found")

	// Test GetDestinationCredentials - should return error for non-existent provider
	destCreds, err := config.GetDestinationCredentials(testDB)
	assert.Error(t, err, "Should get error for non-existent destination provider")
	assert.Nil(t, destCreds, "Destination credentials should be nil for non-existent provider")
	assert.Contains(t, err.Error(), "record not found", "Error should mention record not found")
}

// TestTransferConfigIncompatibleProviderTypes tests behavior when provider types don't match config types
func TestTransferConfigIncompatibleProviderTypes(t *testing.T) {
	testDB := setupTransferConfigTestDB(t)
	defer cleanupTransferConfigTestDB(t, testDB.DB)

	// Create test storage providers
	sourceProvider := &db.StorageProvider{
		Name:               "S3 Source",
		Type:               db.ProviderTypeS3,
		AccessKey:          "sourcekey",
		EncryptedSecretKey: "encrypted_secret_key_source",
		Region:             "us-east-1",
		CreatedBy:          1,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	destProvider := &db.StorageProvider{
		Name:              "FTP Destination",
		Type:              db.ProviderTypeFTP,
		Host:              "dest.example.com",
		Port:              21,
		Username:          "destuser",
		EncryptedPassword: "encrypted_password_dest",
		CreatedBy:         1,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Save providers to database
	err := testDB.CreateStorageProvider(sourceProvider)
	assert.NoError(t, err, "Failed to create source provider")

	err = testDB.CreateStorageProvider(destProvider)
	assert.NoError(t, err, "Failed to create destination provider")

	// Create a transfer config with incompatible type declarations
	config := &db.TransferConfig{
		Name:            "Test Config with Incompatible Types",
		SourcePath:      "/source/path",
		DestinationPath: "/dest/path",
		CreatedBy:       1,
		SourceType:      "sftp", // This is incompatible with the S3 provider
		DestinationType: "s3",   // This is incompatible with the FTP provider
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Set provider references
	config.SetSourceProvider(sourceProvider)
	config.SetDestinationProvider(destProvider)

	// Save to database
	err = testDB.Create(config).Error
	assert.NoError(t, err, "Failed to create transfer config with incompatible types")

	// Test GetCredentials methods
	sourceCreds, err := config.GetSourceCredentials(testDB)
	assert.NoError(t, err, "Should still get credentials despite type mismatch")

	// Verify we can still get credentials from the provider despite type mismatch
	assert.Equal(t, "sourcekey", sourceCreds["access_key"], "Should get correct credentials from provider despite type mismatch")

	// The config's type is not automatically updated to match the provider
	// Instead, it remains as what was explicitly set
	assert.Equal(t, "sftp", config.SourceType, "Source type should remain as explicitly set")

	destCreds, err := config.GetDestinationCredentials(testDB)
	assert.NoError(t, err, "Should still get credentials despite type mismatch")

	// Verify we can still get credentials from the provider despite type mismatch
	assert.Equal(t, "destuser", destCreds["username"], "Should get correct credentials from provider despite type mismatch")

	// The config's type is not automatically updated to match the provider
	assert.Equal(t, "s3", config.DestinationType, "Destination type should remain as explicitly set")
}
