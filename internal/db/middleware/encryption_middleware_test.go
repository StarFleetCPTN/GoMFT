package middleware

import (
	"testing"
	"time"

	"strings"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestModel is a simple model for testing encryption middleware
type TestModel struct {
	ID                uint      `gorm:"primarykey"`
	Name              string    `gorm:"not null"`
	Password          string    `gorm:"-"` // Not stored in DB, only for form input
	EncryptedPassword string    `gorm:"column:encrypted_password"`
	APIKey            string    `gorm:"-"` // Not stored in DB, only for form input
	EncryptedAPIKey   string    `gorm:"column:encrypted_api_key"`
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

// StorageProvider is a simplified version of the real model for testing
type StorageProvider struct {
	ID                    uint      `gorm:"primarykey"`
	Name                  string    `gorm:"not null"`
	Type                  string    `gorm:"not null"`
	Password              string    `gorm:"-"` // Not stored in DB
	EncryptedPassword     string    `gorm:"column:encrypted_password"`
	SecretKey             string    `gorm:"-"` // Not stored in DB
	EncryptedSecretKey    string    `gorm:"column:encrypted_secret_key"`
	ClientSecret          string    `gorm:"-"` // Not stored in DB
	EncryptedClientSecret string    `gorm:"column:encrypted_client_secret"`
	RefreshToken          string    `gorm:"-"` // Not stored in DB
	EncryptedRefreshToken string    `gorm:"column:encrypted_refresh_token"`
	CreatedAt             time.Time `gorm:"not null"`
	UpdatedAt             time.Time `gorm:"not null"`
}

// GetSensitiveFields returns a map of field names to values that need encryption
func (sp *StorageProvider) GetSensitiveFields() map[string]string {
	sensitiveFields := make(map[string]string)

	if sp.Password != "" {
		sensitiveFields["Password"] = sp.Password
	}
	if sp.SecretKey != "" {
		sensitiveFields["SecretKey"] = sp.SecretKey
	}
	if sp.ClientSecret != "" {
		sensitiveFields["ClientSecret"] = sp.ClientSecret
	}
	if sp.RefreshToken != "" {
		sensitiveFields["RefreshToken"] = sp.RefreshToken
	}

	return sensitiveFields
}

func setupTestDB(t *testing.T) *gorm.DB {
	// Initialize in-memory SQLite database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to in-memory database")

	// Migrate the test models
	err = db.AutoMigrate(&TestModel{}, &StorageProvider{})
	require.NoError(t, err, "Failed to migrate test models")

	return db
}

func setupEncryptionMiddleware(t *testing.T) (*EncryptionMiddleware, error) {
	// Initialize the encryption key manager for testing
	err := encryption.InitializeKeyManager("test-key")
	require.NoError(t, err, "Failed to initialize key manager")

	// Create the encryption middleware
	return NewEncryptionMiddleware()
}

func TestEncryptionMiddlewareWithGenericModel(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	middleware, err := setupEncryptionMiddleware(t)
	require.NoError(t, err, "Failed to setup encryption middleware")

	// Register hooks with GORM
	middleware.RegisterHooks(db)

	// Create a test model
	testModel := &TestModel{
		Name:     "Test User",
		Password: "securePassword123",
		APIKey:   "api-key-12345",
	}

	// Save the model - should trigger encryption
	err = db.Create(testModel).Error
	require.NoError(t, err, "Failed to save test model")

	// Verify encrypted fields are set and original fields are cleared
	assert.Empty(t, testModel.Password, "Password should be cleared after save")
	assert.Empty(t, testModel.APIKey, "APIKey should be cleared after save")
	assert.NotEmpty(t, testModel.EncryptedPassword, "EncryptedPassword should be set")
	assert.NotEmpty(t, testModel.EncryptedAPIKey, "EncryptedAPIKey should be set")
	assert.True(t, strings.HasPrefix(testModel.EncryptedPassword, encryption.EncryptedPrefix), "EncryptedPassword should have encryption prefix")
	assert.True(t, strings.HasPrefix(testModel.EncryptedAPIKey, encryption.EncryptedPrefix), "EncryptedAPIKey should have encryption prefix")

	// Test retrieval and automatic decryption
	retrievedModel := new(TestModel)
	err = db.First(retrievedModel, testModel.ID).Error
	require.NoError(t, err, "Failed to retrieve test model")

	// Verify decryption
	assert.Equal(t, "securePassword123", retrievedModel.Password, "Password should be automatically decrypted")
	assert.Equal(t, "api-key-12345", retrievedModel.APIKey, "APIKey should be automatically decrypted")
	assert.NotEmpty(t, retrievedModel.EncryptedPassword, "EncryptedPassword should remain set")
	assert.NotEmpty(t, retrievedModel.EncryptedAPIKey, "EncryptedAPIKey should remain set")
}

func TestEncryptionMiddlewareWithStorageProvider(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	middleware, err := setupEncryptionMiddleware(t)
	require.NoError(t, err, "Failed to setup encryption middleware")

	// Register hooks with GORM
	middleware.RegisterHooks(db)

	// Create a storage provider
	provider := &StorageProvider{
		Name:         "Test S3",
		Type:         "s3",
		Password:     "testPassword",
		SecretKey:    "testSecretKey",
		ClientSecret: "testClientSecret",
		RefreshToken: "testRefreshToken",
	}

	// Save the provider - should trigger encryption
	err = db.Create(provider).Error
	require.NoError(t, err, "Failed to save storage provider")

	// Verify encrypted fields are set and original fields are cleared
	assert.Empty(t, provider.Password, "Password should be cleared after save")
	assert.Empty(t, provider.SecretKey, "SecretKey should be cleared after save")
	assert.Empty(t, provider.ClientSecret, "ClientSecret should be cleared after save")
	assert.Empty(t, provider.RefreshToken, "RefreshToken should be cleared after save")
	assert.NotEmpty(t, provider.EncryptedPassword, "EncryptedPassword should be set")
	assert.NotEmpty(t, provider.EncryptedSecretKey, "EncryptedSecretKey should be set")
	assert.NotEmpty(t, provider.EncryptedClientSecret, "EncryptedClientSecret should be set")
	assert.NotEmpty(t, provider.EncryptedRefreshToken, "EncryptedRefreshToken should be set")

	// Test retrieval and automatic decryption
	retrievedProvider := new(StorageProvider)
	err = db.First(retrievedProvider, provider.ID).Error
	require.NoError(t, err, "Failed to retrieve storage provider")

	// Verify decryption
	assert.Equal(t, "testPassword", retrievedProvider.Password, "Password should be automatically decrypted")
	assert.Equal(t, "testSecretKey", retrievedProvider.SecretKey, "SecretKey should be automatically decrypted")
	assert.Equal(t, "testClientSecret", retrievedProvider.ClientSecret, "ClientSecret should be automatically decrypted")
	assert.Equal(t, "testRefreshToken", retrievedProvider.RefreshToken, "RefreshToken should be automatically decrypted")
}

func TestEncryptionMiddlewareWithMultipleRecords(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	middleware, err := setupEncryptionMiddleware(t)
	require.NoError(t, err, "Failed to setup encryption middleware")

	// Register hooks with GORM
	middleware.RegisterHooks(db)

	// Create multiple test models
	models := []TestModel{
		{Name: "User 1", Password: "password1", APIKey: "apikey1"},
		{Name: "User 2", Password: "password2", APIKey: "apikey2"},
		{Name: "User 3", Password: "password3", APIKey: "apikey3"},
	}

	// Save all models
	err = db.Create(&models).Error
	require.NoError(t, err, "Failed to save multiple test models")

	// Retrieve all models
	var retrievedModels []TestModel
	err = db.Find(&retrievedModels).Error
	require.NoError(t, err, "Failed to retrieve all test models")

	// Verify count
	assert.Equal(t, 3, len(retrievedModels), "Should retrieve 3 models")

	// Verify each model was properly decrypted
	expectedPasswords := []string{"password1", "password2", "password3"}
	expectedAPIKeys := []string{"apikey1", "apikey2", "apikey3"}

	for i, model := range retrievedModels {
		assert.Equal(t, expectedPasswords[i], model.Password, "Password should be automatically decrypted")
		assert.Equal(t, expectedAPIKeys[i], model.APIKey, "APIKey should be automatically decrypted")
		assert.NotEmpty(t, model.EncryptedPassword, "EncryptedPassword should remain set")
		assert.NotEmpty(t, model.EncryptedAPIKey, "EncryptedAPIKey should remain set")
	}
}

func TestEncryptionMiddlewareDisabled(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	middleware, err := setupEncryptionMiddleware(t)
	require.NoError(t, err, "Failed to setup encryption middleware")

	// Register hooks with GORM
	middleware.RegisterHooks(db)

	// Disable the middleware
	middleware.Disable()
	assert.False(t, middleware.IsEnabled(), "Middleware should be disabled")

	// Create a test model
	testModel := &TestModel{
		Name:     "Test User",
		Password: "securePassword123",
		APIKey:   "api-key-12345",
	}

	// Save the model - should NOT trigger encryption since middleware is disabled
	err = db.Create(testModel).Error
	require.NoError(t, err, "Failed to save test model")

	// Verify sensitive fields are NOT encrypted
	assert.Equal(t, "securePassword123", testModel.Password, "Password should not be cleared when middleware is disabled")
	assert.Equal(t, "api-key-12345", testModel.APIKey, "APIKey should not be cleared when middleware is disabled")
	assert.Empty(t, testModel.EncryptedPassword, "EncryptedPassword should not be set when middleware is disabled")
	assert.Empty(t, testModel.EncryptedAPIKey, "EncryptedAPIKey should not be set when middleware is disabled")

	// Re-enable the middleware for subsequent operations
	middleware.Enable()
	assert.True(t, middleware.IsEnabled(), "Middleware should be enabled")

	// Update the model - should now trigger encryption
	testModel.Password = "newPassword456"
	err = db.Save(testModel).Error
	require.NoError(t, err, "Failed to update test model")

	// Verify encryption now happened
	assert.Empty(t, testModel.Password, "Password should be cleared after update with middleware enabled")
	assert.NotEmpty(t, testModel.EncryptedPassword, "EncryptedPassword should be set after update with middleware enabled")
}
