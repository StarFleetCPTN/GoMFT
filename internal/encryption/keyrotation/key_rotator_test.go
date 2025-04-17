package keyrotation

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/starfleetcptn/gomft/internal/encryption/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestModel is a simple model with encrypted fields for testing
type TestModel struct {
	ID             uint `gorm:"primaryKey"`
	Name           string
	EncryptedField string
	EncryptedData  string
	EncryptedKey   string
	StandardField  string
}

// setupTestAuditor creates an auditor for testing with buffer for capturing logs
func setupTestAuditor(t testing.TB) (*audit.SecurityAuditor, *bytes.Buffer) {
	logBuffer := new(bytes.Buffer)
	errorBuffer := new(bytes.Buffer)

	auditor, err := audit.New()
	if err != nil {
		t.Fatal(err)
	}

	// Set log writers to capture output
	auditValue := reflect.ValueOf(auditor).Elem()
	if logField := auditValue.FieldByName("logWriter"); logField.IsValid() && logField.CanSet() {
		logField.Set(reflect.ValueOf(logBuffer))
	}
	if errorField := auditValue.FieldByName("errorWriter"); errorField.IsValid() && errorField.CanSet() {
		errorField.Set(reflect.ValueOf(errorBuffer))
	}

	return auditor, logBuffer
}

// setupTestDB creates a test database with the TestModel
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate the schema
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	return db
}

// setupEncryptionServices creates old and new encryption services for testing
func setupEncryptionServices(t testing.TB) (*encryption.EncryptionService, *encryption.EncryptionService) {
	// Setup old key
	oldKeyEnv := "TEST_OLD_KEY"
	oldKey := make([]byte, encryption.AES256KeySize)
	for i := range oldKey {
		oldKey[i] = byte(i % 256)
	}
	os.Setenv(oldKeyEnv, base64.StdEncoding.EncodeToString(oldKey))

	// Setup new key
	newKeyEnv := "TEST_NEW_KEY"
	newKey := make([]byte, encryption.AES256KeySize)
	for i := range newKey {
		newKey[i] = byte((i + 128) % 256) // Different key
	}
	os.Setenv(newKeyEnv, base64.StdEncoding.EncodeToString(newKey))

	if t, ok := t.(*testing.T); ok {
		t.Cleanup(func() {
			os.Unsetenv(oldKeyEnv)
			os.Unsetenv(newKeyEnv)
		})
	}

	// Create key managers
	oldKM := encryption.NewKeyManager(oldKeyEnv)
	err := oldKM.Initialize()
	if err != nil {
		t.Fatal(err)
	}

	newKM := encryption.NewKeyManager(newKeyEnv)
	err = newKM.Initialize()
	if err != nil {
		t.Fatal(err)
	}

	// Create encryption services
	oldService, err := encryption.NewEncryptionService(oldKM)
	if err != nil {
		t.Fatal(err)
	}

	newService, err := encryption.NewEncryptionService(newKM)
	if err != nil {
		t.Fatal(err)
	}

	return oldService, newService
}

// createTestData creates test records with encrypted fields
func createTestData(t testing.TB, db *gorm.DB, oldService *encryption.EncryptionService, count int) {
	for i := 1; i <= count; i++ {
		// Create encrypted values with the old key
		field1, err := oldService.EncryptString(fmt.Sprintf("secret-field-%d", i))
		if err != nil {
			t.Fatal(err)
		}

		field2, err := oldService.EncryptString(fmt.Sprintf("secret-data-%d", i))
		if err != nil {
			t.Fatal(err)
		}

		field3, err := oldService.EncryptString(fmt.Sprintf("secret-key-%d", i))
		if err != nil {
			t.Fatal(err)
		}

		// Create a test record
		record := TestModel{
			Name:           fmt.Sprintf("Test Record %d", i),
			EncryptedField: encryption.EncryptedPrefix + field1,
			EncryptedData:  encryption.EncryptedPrefix + field2,
			EncryptedKey:   encryption.EncryptedPrefix + field3,
			StandardField:  fmt.Sprintf("standard-field-%d", i),
		}

		// Save to DB
		result := db.Create(&record)
		if err := result.Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewKeyRotator(t *testing.T) {
	db := setupTestDB(t)
	oldService, newService := setupEncryptionServices(t)
	auditor, _ := setupTestAuditor(t)

	t.Run("Valid rotator creation", func(t *testing.T) {
		rotator, err := NewKeyRotator(db, oldService, newService, auditor)
		require.NoError(t, err)
		assert.NotNil(t, rotator)
		assert.False(t, rotator.dryRun)
		assert.Equal(t, 100, rotator.batchSize)
		assert.Equal(t, 50, rotator.maxErrors)
	})

	t.Run("Nil DB", func(t *testing.T) {
		rotator, err := NewKeyRotator(nil, oldService, newService, auditor)
		require.Error(t, err)
		assert.Nil(t, rotator)
		assert.Equal(t, ErrNilDB, err)
	})

	t.Run("Nil old service", func(t *testing.T) {
		rotator, err := NewKeyRotator(db, nil, newService, auditor)
		require.Error(t, err)
		assert.Nil(t, rotator)
		assert.Equal(t, ErrNoOldKey, err)
	})

	t.Run("Nil new service", func(t *testing.T) {
		rotator, err := NewKeyRotator(db, oldService, nil, auditor)
		require.Error(t, err)
		assert.Nil(t, rotator)
		assert.Equal(t, ErrNoNewKey, err)
	})

	t.Run("Same service", func(t *testing.T) {
		rotator, err := NewKeyRotator(db, oldService, oldService, auditor)
		require.Error(t, err)
		assert.Nil(t, rotator)
		assert.Equal(t, ErrSameKey, err)
	})

	t.Run("Default auditor", func(t *testing.T) {
		rotator, err := NewKeyRotator(db, oldService, newService, nil)
		require.NoError(t, err)
		assert.NotNil(t, rotator)
		assert.NotNil(t, rotator.auditor)
	})
}

func TestKeyRotatorConfigMethods(t *testing.T) {
	db := setupTestDB(t)
	oldService, newService := setupEncryptionServices(t)
	auditor, _ := setupTestAuditor(t)

	rotator, err := NewKeyRotator(db, oldService, newService, auditor)
	require.NoError(t, err)

	t.Run("SetDryRun", func(t *testing.T) {
		rotator.SetDryRun(true)
		assert.True(t, rotator.dryRun)

		rotator.SetDryRun(false)
		assert.False(t, rotator.dryRun)
	})

	t.Run("SetBatchSize", func(t *testing.T) {
		rotator.SetBatchSize(200)
		assert.Equal(t, 200, rotator.batchSize)

		// Test with invalid value
		rotator.SetBatchSize(0)
		assert.Equal(t, 200, rotator.batchSize) // Shouldn't change

		rotator.SetBatchSize(-10)
		assert.Equal(t, 200, rotator.batchSize) // Shouldn't change
	})

	t.Run("SetMaxErrors", func(t *testing.T) {
		rotator.SetMaxErrors(100)
		assert.Equal(t, 100, rotator.maxErrors)

		rotator.SetMaxErrors(0)
		assert.Equal(t, 0, rotator.maxErrors) // 0 is valid (no max)

		// Test with invalid value
		rotator.SetMaxErrors(-10)
		assert.Equal(t, 0, rotator.maxErrors) // Shouldn't change
	})
}

func TestRotateKeys(t *testing.T) {
	db := setupTestDB(t)
	oldService, newService := setupEncryptionServices(t)
	auditor, logBuffer := setupTestAuditor(t)

	rotator, err := NewKeyRotator(db, oldService, newService, auditor)
	require.NoError(t, err)

	t.Run("Rotate keys for model with no records", func(t *testing.T) {
		stats, err := rotator.RotateKeys(&TestModel{}, "")
		require.Error(t, err)
		assert.Equal(t, ErrNoDataToMigrate, err)
		assert.Equal(t, 0, stats.TotalRecords)
	})

	t.Run("Rotate keys for non-struct model", func(t *testing.T) {
		stats, err := rotator.RotateKeys("not a struct", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a struct")
		assert.Equal(t, 0, stats.TotalRecords, "Expected total records to be 0 for non-struct model")
	})

	t.Run("Rotate keys for model with records", func(t *testing.T) {
		// Reset log buffer
		logBuffer.Reset()

		// Create test data
		createTestData(t, db, oldService, 10)

		// Perform key rotation
		stats, err := rotator.RotateKeys(&TestModel{}, "")
		require.NoError(t, err)
		assert.Equal(t, 10, stats.TotalRecords)
		assert.Equal(t, 10, stats.ProcessedRecords)
		assert.Equal(t, 0, stats.FailedRecords)
		assert.NotZero(t, stats.ElapsedTime)
		assert.Empty(t, stats.Errors)

		// Verify that records were updated with re-encrypted values
		var records []TestModel
		result := db.Find(&records)
		require.NoError(t, result.Error)
		assert.Equal(t, 10, len(records))

		// Test a sample record to ensure it was re-encrypted properly
		record := records[0]

		// Verify the old key can't decrypt the new values
		_, err = oldService.DecryptString(strings.TrimPrefix(record.EncryptedField, encryption.EncryptedPrefix))
		assert.Error(t, err, "Old key should not be able to decrypt new values")

		// Verify the new key can decrypt the values
		decryptedField, err := newService.DecryptString(strings.TrimPrefix(record.EncryptedField, encryption.EncryptedPrefix))
		require.NoError(t, err)
		assert.Equal(t, "secret-field-1", decryptedField)

		// Verify audit logs were created
		logContent := logBuffer.String()
		assert.Contains(t, logContent, "key_rotation")
		assert.Contains(t, logContent, "TestModel")

		// Verify no sensitive data in logs
		assert.NotContains(t, logContent, "secret-field")
		assert.NotContains(t, logContent, "secret-data")
		assert.NotContains(t, logContent, "secret-key")
	})

	t.Run("Dry run mode", func(t *testing.T) {
		// Reset the database
		db.Exec("DELETE FROM test_models")
		createTestData(t, db, oldService, 5)

		// Create a new rotator with dry run enabled
		rotator, err := NewKeyRotator(db, oldService, newService, auditor)
		require.NoError(t, err)
		rotator.SetDryRun(true)

		// Perform key rotation
		stats, err := rotator.RotateKeys(&TestModel{}, "")
		require.NoError(t, err)
		assert.Equal(t, 5, stats.TotalRecords)

		// Verify that records were NOT updated with re-encrypted values
		var records []TestModel
		result := db.Find(&records)
		require.NoError(t, result.Error)

		// Test a sample record to ensure it was NOT re-encrypted
		record := records[0]

		// Verify the old key CAN decrypt the values (because they weren't changed)
		decryptedField, err := oldService.DecryptString(strings.TrimPrefix(record.EncryptedField, encryption.EncryptedPrefix))
		require.NoError(t, err)
		assert.Equal(t, "secret-field-1", decryptedField)
	})
}

func TestRotateKeysWithErrors(t *testing.T) {
	db := setupTestDB(t)
	oldService, newService := setupEncryptionServices(t)
	auditor, _ := setupTestAuditor(t)

	rotator, err := NewKeyRotator(db, oldService, newService, auditor)
	require.NoError(t, err)

	// Create test data with one corrupted record
	createTestData(t, db, oldService, 5)

	// Create a corrupted record that can't be decrypted
	corruptedRecord := TestModel{
		Name:           "Corrupted Record",
		EncryptedField: encryption.EncryptedPrefix + "corrupted-data",
		EncryptedData:  encryption.EncryptedPrefix + "corrupted-data",
		StandardField:  "standard-field",
	}
	result := db.Create(&corruptedRecord)
	require.NoError(t, result.Error)

	// Perform key rotation
	stats, err := rotator.RotateKeys(&TestModel{}, "")
	require.NoError(t, err) // Should still succeed overall
	assert.Equal(t, 6, stats.TotalRecords)
	assert.Equal(t, 5, stats.ProcessedRecords) // Only 5 should be processed successfully
	assert.Equal(t, 0, stats.FailedRecords)    // Failure to decrypt is skipped, not counted as error

	// Verify that the valid records were updated
	var records []TestModel
	db.Where("name LIKE ?", "Test Record%").Find(&records)
	require.Equal(t, 5, len(records))

	for _, record := range records {
		// Verify the new key can decrypt
		_, err = newService.DecryptString(strings.TrimPrefix(record.EncryptedField, encryption.EncryptedPrefix))
		assert.NoError(t, err)
	}

	// Verify the corrupted record wasn't changed
	var corrupted TestModel
	db.Where("name = ?", "Corrupted Record").First(&corrupted)
	assert.Equal(t, encryption.EncryptedPrefix+"corrupted-data", corrupted.EncryptedField)
}

func TestGetPrimaryKeyValue(t *testing.T) {
	type TestStruct struct {
		ID        uint
		CustomID  string
		NotAnID   string
		OtherData string
	}

	t.Run("Default ID field", func(t *testing.T) {
		test := TestStruct{ID: 123, OtherData: "test"}
		val := getPrimaryKeyValue(reflect.ValueOf(test), "")
		assert.Equal(t, uint(123), val)
	})

	t.Run("Custom ID field", func(t *testing.T) {
		test := TestStruct{ID: 123, CustomID: "ABC123", OtherData: "test"}
		val := getPrimaryKeyValue(reflect.ValueOf(test), "CustomID")
		assert.Equal(t, "ABC123", val)
	})

	t.Run("Non-existent ID field", func(t *testing.T) {
		test := TestStruct{ID: 123, OtherData: "test"}
		val := getPrimaryKeyValue(reflect.ValueOf(test), "NonExistentID")
		assert.Equal(t, "<unknown>", val)
	})
}

// BenchmarkKeyRotation measures the performance of key rotation
func BenchmarkKeyRotation(b *testing.B) {
	// Setup
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}
	db.AutoMigrate(&TestModel{})

	oldService, newService := setupEncryptionServices(b)
	auditor, _ := setupTestAuditor(b)

	rotator, err := NewKeyRotator(db, oldService, newService, auditor)
	if err != nil {
		b.Fatal(err)
	}

	// Create benchmark data sets of different sizes
	benchmarks := []struct {
		name       string
		numRecords int
	}{
		{"Small (10 records)", 10},
		{"Medium (100 records)", 100},
		{"Large (500 records)", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Reset the database for each benchmark iteration
			db.Exec("DELETE FROM test_models")
			createTestData(b, db, oldService, bm.numRecords)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				stats, err := rotator.RotateKeys(&TestModel{}, "")
				if err != nil {
					b.Fatal(err)
				}
				if stats.ProcessedRecords != bm.numRecords {
					b.Fatalf("Expected %d records, got %d", bm.numRecords, stats.ProcessedRecords)
				}

				// Reset for the next iteration
				if i < b.N-1 {
					db.Exec("DELETE FROM test_models")
					createTestData(b, db, oldService, bm.numRecords)
				}
			}
		})
	}
}

// Benchmarks for different batch sizes
func BenchmarkBatchSizes(b *testing.B) {
	// Setup
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}
	db.AutoMigrate(&TestModel{})

	oldService, newService := setupEncryptionServices(b)
	auditor, _ := setupTestAuditor(b)

	// Create a dataset of 500 records
	const numRecords = 500
	createTestData(b, db, oldService, numRecords)

	// Test different batch sizes
	batchSizes := []int{10, 50, 100, 200, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			rotator, err := NewKeyRotator(db, oldService, newService, auditor)
			if err != nil {
				b.Fatal(err)
			}
			rotator.SetBatchSize(batchSize)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset data before each run
				if i > 0 {
					db.Exec("DELETE FROM test_models")
					createTestData(b, db, oldService, numRecords)
				}

				stats, err := rotator.RotateKeys(&TestModel{}, "")
				if err != nil {
					b.Fatal(err)
				}
				if stats.ProcessedRecords != numRecords {
					b.Fatalf("Expected %d records, got %d", numRecords, stats.ProcessedRecords)
				}
			}
		})
	}
}
