package keyrotation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/starfleetcptn/gomft/internal/encryption/audit"
	"gorm.io/gorm"
)

// Common errors
var (
	ErrNoOldKey        = errors.New("old encryption key not found")
	ErrNoNewKey        = errors.New("new encryption key not found")
	ErrSameKey         = errors.New("old and new keys are the same")
	ErrNoDataToMigrate = errors.New("no data to migrate")
	ErrNilDB           = errors.New("database connection is nil")
)

// RotationStats represents statistics about the key rotation process
type RotationStats struct {
	TotalRecords     int           `json:"total_records"`
	ProcessedRecords int           `json:"processed_records"`
	SkippedRecords   int           `json:"skipped_records"`
	FailedRecords    int           `json:"failed_records"`
	ElapsedTime      time.Duration `json:"elapsed_time"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	Errors           []string      `json:"errors,omitempty"`
}

// KeyRotator manages the process of changing encryption keys and re-encrypting data
type KeyRotator struct {
	db         *gorm.DB
	oldService *encryption.EncryptionService
	newService *encryption.EncryptionService
	auditor    *audit.SecurityAuditor
	dryRun     bool
	batchSize  int
	maxErrors  int
}

// NewKeyRotator creates a new KeyRotator
func NewKeyRotator(db *gorm.DB, oldService, newService *encryption.EncryptionService, auditor *audit.SecurityAuditor) (*KeyRotator, error) {
	if db == nil {
		return nil, ErrNilDB
	}

	if oldService == nil {
		return nil, ErrNoOldKey
	}

	if newService == nil {
		return nil, ErrNoNewKey
	}

	if oldService == newService {
		return nil, ErrSameKey
	}

	if auditor == nil {
		// Use the global auditor if none provided
		auditor = audit.GetGlobalAuditor()
	}

	return &KeyRotator{
		db:         db,
		oldService: oldService,
		newService: newService,
		auditor:    auditor,
		dryRun:     false,
		batchSize:  100,
		maxErrors:  50,
	}, nil
}

// SetDryRun enables or disables dry run mode
func (r *KeyRotator) SetDryRun(dryRun bool) {
	r.dryRun = dryRun
}

// SetBatchSize sets the batch size for processing records
func (r *KeyRotator) SetBatchSize(size int) {
	if size > 0 {
		r.batchSize = size
	}
}

// SetMaxErrors sets the maximum number of errors allowed before aborting
func (r *KeyRotator) SetMaxErrors(max int) {
	if max >= 0 {
		r.maxErrors = max
	}
}

// RotateKeys rotates encryption keys for a specific model type
func (r *KeyRotator) RotateKeys(modelType interface{}, primaryKeyName string) (*RotationStats, error) {
	stats := &RotationStats{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	// Get the model type
	modelValue := reflect.ValueOf(modelType)
	if modelValue.Kind() == reflect.Ptr {
		modelValue = modelValue.Elem()
	}

	// Skip if the value is not a struct
	if modelValue.Kind() != reflect.Struct {
		return stats, errors.New("model type must be a struct")
	}

	modelName := modelValue.Type().Name()

	// Count total records
	var count int64
	if err := r.db.Model(modelType).Count(&count).Error; err != nil {
		return stats, fmt.Errorf("failed to count records: %w", err)
	}

	stats.TotalRecords = int(count)

	if count == 0 {
		return stats, ErrNoDataToMigrate
	}

	// Process in batches
	offset := 0
	for offset < int(count) {
		// Get a batch of records
		records := reflect.New(reflect.SliceOf(modelValue.Type())).Interface()

		if err := r.db.Model(modelType).Offset(offset).Limit(r.batchSize).Find(records).Error; err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("failed to fetch batch at offset %d: %v", offset, err))
			if len(stats.Errors) >= r.maxErrors {
				return stats, fmt.Errorf("too many errors (%d), aborting key rotation", len(stats.Errors))
			}
			offset += r.batchSize
			continue
		}

		// Process this batch
		batchRecords := reflect.ValueOf(records).Elem()
		for i := 0; i < batchRecords.Len(); i++ {
			record := batchRecords.Index(i)
			if record.Kind() == reflect.Ptr {
				record = record.Elem()
			}

			if err := r.rotateKeysForRecord(record, modelName, primaryKeyName); err != nil {
				pkValue := getPrimaryKeyValue(record, primaryKeyName)
				stats.Errors = append(stats.Errors, fmt.Sprintf("failed to rotate keys for %s with ID %v: %v", modelName, pkValue, err))
				stats.FailedRecords++

				if len(stats.Errors) >= r.maxErrors {
					stats.EndTime = time.Now()
					stats.ElapsedTime = stats.EndTime.Sub(stats.StartTime)
					return stats, fmt.Errorf("too many errors (%d), aborting key rotation", len(stats.Errors))
				}
			} else {
				stats.ProcessedRecords++
			}
		}

		offset += r.batchSize
	}

	stats.EndTime = time.Now()
	stats.ElapsedTime = stats.EndTime.Sub(stats.StartTime)

	return stats, nil
}

// rotateKeysForRecord processes a single record
func (r *KeyRotator) rotateKeysForRecord(record reflect.Value, modelName, primaryKeyName string) error {
	if !record.IsValid() || record.Kind() != reflect.Struct {
		return errors.New("invalid record")
	}

	// Check if there are any encrypted fields to migrate
	encryptedFieldsFound := false
	recordType := record.Type()

	// Track changes for audit
	pkValue := getPrimaryKeyValue(record, primaryKeyName)
	changes := make(map[string]struct{})

	// Process each field in the struct
	for i := 0; i < recordType.NumField(); i++ {
		field := recordType.Field(i)

		// Look for encrypted fields
		fieldName := field.Name
		if strings.HasPrefix(fieldName, "Encrypted") {
			// Get the field value
			fieldValue := record.Field(i)
			if !fieldValue.CanInterface() || !fieldValue.CanSet() {
				continue
			}

			// Get the encrypted value
			encryptedValue, ok := fieldValue.Interface().(string)
			if !ok || encryptedValue == "" {
				continue
			}

			// If it's not encrypted with our old key, skip it
			if !strings.HasPrefix(encryptedValue, encryption.EncryptedPrefix) {
				continue
			}

			encryptedFieldsFound = true

			// Try to decrypt with the old key
			trimmedValue := strings.TrimPrefix(encryptedValue, encryption.EncryptedPrefix)
			plaintext, err := r.oldService.DecryptString(trimmedValue)
			if err != nil {
				// Skip this field if we can't decrypt it (might be encrypted with a different key)
				continue
			}

			// Re-encrypt with the new key
			newEncrypted, err := r.newService.EncryptString(plaintext)
			if err != nil {
				return fmt.Errorf("failed to re-encrypt field %s: %w", fieldName, err)
			}

			// Only update if different
			newValue := encryption.EncryptedPrefix + newEncrypted
			if newValue != encryptedValue {
				if !r.dryRun {
					fieldValue.SetString(newValue)
				}
				changes[fieldName] = struct{}{}
			}
		}
	}

	// If no encrypted fields were found or modified, return
	if !encryptedFieldsFound || len(changes) == 0 {
		return nil
	}

	// Save the changes to the database
	if !r.dryRun {
		if err := r.db.Save(record.Addr().Interface()).Error; err != nil {
			return fmt.Errorf("failed to save record: %w", err)
		}
	}

	// Log the rotation
	if r.auditor != nil {
		changedFields := make([]string, 0, len(changes))
		for field := range changes {
			changedFields = append(changedFields, field)
		}

		description := fmt.Sprintf("Rotated keys for %s (ID: %v) - fields: %s",
			modelName, pkValue, strings.Join(changedFields, ", "))

		r.auditor.LogKeyRotationEventWithDescription(
			"old", "new", true, description, 0,
		)
	}

	return nil
}

// getPrimaryKeyValue gets the value of the primary key field
func getPrimaryKeyValue(record reflect.Value, pkName string) interface{} {
	if pkName == "" {
		pkName = "ID" // Default primary key name
	}

	pkField := record.FieldByName(pkName)
	if !pkField.IsValid() {
		return "<unknown>"
	}

	return pkField.Interface()
}
