package middleware

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/starfleetcptn/gomft/internal/encryption"
	"gorm.io/gorm"
)

// EncryptionMiddleware handles automatic encryption and decryption of model fields
type EncryptionMiddleware struct {
	encryptor *encryption.CredentialEncryptor
	enabled   bool
}

// NewEncryptionMiddleware creates a new middleware instance for encrypting/decrypting fields
func NewEncryptionMiddleware() (*EncryptionMiddleware, error) {
	// Get the global credential encryptor
	encryptor, err := encryption.GetGlobalCredentialEncryptor()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryption middleware: %w", err)
	}

	return &EncryptionMiddleware{
		encryptor: encryptor,
		enabled:   true,
	}, nil
}

// Enable turns on automatic encryption/decryption
func (m *EncryptionMiddleware) Enable() {
	m.enabled = true
}

// Disable turns off automatic encryption/decryption
func (m *EncryptionMiddleware) Disable() {
	m.enabled = false
}

// IsEnabled returns whether the middleware is enabled
func (m *EncryptionMiddleware) IsEnabled() bool {
	return m.enabled
}

// RegisterHooks registers the encryption/decryption hooks with the GORM instance
func (m *EncryptionMiddleware) RegisterHooks(db *gorm.DB) {
	// Register BeforeSave hook to encrypt sensitive fields
	db.Callback().Create().Before("gorm:create").Register("encrypt_before_create", m.encryptBeforeSave)
	db.Callback().Update().Before("gorm:update").Register("encrypt_before_update", m.encryptBeforeSave)

	// Register AfterFind hook to decrypt sensitive fields
	db.Callback().Query().After("gorm:after_query").Register("decrypt_after_find", m.decryptAfterFind)
}

// encryptBeforeSave encrypts sensitive fields before saving to the database
func (m *EncryptionMiddleware) encryptBeforeSave(db *gorm.DB) {
	if !m.enabled {
		return
	}

	// Get the model value
	value := db.Statement.ReflectValue
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// Skip if the value is not a struct
	if value.Kind() != reflect.Struct {
		return
	}

	// Process the model
	if err := m.processModelForEncryption(value); err != nil {
		db.AddError(fmt.Errorf("encryption middleware error: %w", err))
	}
}

// decryptAfterFind decrypts sensitive fields after retrieving from the database
func (m *EncryptionMiddleware) decryptAfterFind(db *gorm.DB) {
	if !m.enabled {
		return
	}

	// Get the model value
	value := db.Statement.ReflectValue
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// Handle slice of models
	if value.Kind() == reflect.Slice {
		for i := 0; i < value.Len(); i++ {
			item := value.Index(i)
			if item.Kind() == reflect.Ptr {
				item = item.Elem()
			}

			if item.Kind() == reflect.Struct {
				if err := m.processModelForDecryption(item); err != nil {
					db.AddError(fmt.Errorf("decryption middleware error [index %d]: %w", i, err))
					return
				}
			}
		}
		return
	}

	// Skip if the value is not a struct
	if value.Kind() != reflect.Struct {
		return
	}

	// Process the model
	if err := m.processModelForDecryption(value); err != nil {
		db.AddError(fmt.Errorf("decryption middleware error: %w", err))
	}
}

// processModelForEncryption encrypts sensitive fields in a model
func (m *EncryptionMiddleware) processModelForEncryption(value reflect.Value) error {
	modelType := value.Type()

	// Special handling for StorageProvider type
	if modelType.Name() == "StorageProvider" {
		return m.encryptStorageProvider(value)
	}

	// Generic handling for models with encryptable fields
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// Check if field requires encryption based on its name
		fieldName := field.Name
		if requiresEncryption, credType := encryption.RequiresEncryption(fieldName); requiresEncryption {
			// Get the field value
			fieldValue := value.Field(i)
			if !fieldValue.CanInterface() || !fieldValue.CanSet() {
				continue
			}

			// Get string value
			strValue, ok := fieldValue.Interface().(string)
			if !ok || strValue == "" {
				continue
			}

			// If already encrypted, skip
			if m.encryptor.IsEncrypted(strValue) {
				continue
			}

			// Encrypt the field
			encryptedValue, err := m.encryptor.Encrypt(strValue, credType)
			if err != nil {
				return fmt.Errorf("failed to encrypt field %s: %w", fieldName, err)
			}

			// Find the corresponding encrypted field
			encryptedFieldName := "Encrypted" + fieldName
			encryptedField := value.FieldByName(encryptedFieldName)

			// If encrypted field exists and can be set, set it
			if encryptedField.IsValid() && encryptedField.CanSet() {
				encryptedField.SetString(encryptedValue)

				// If the original field is marked with gorm:"-", we should clear it to prevent leaking it
				if field.Tag.Get("gorm") == "-" {
					fieldValue.SetString("")
				}
			}
		}
	}

	return nil
}

// processModelForDecryption decrypts encrypted fields in a model
func (m *EncryptionMiddleware) processModelForDecryption(value reflect.Value) error {
	modelType := value.Type()

	// Special handling for StorageProvider type
	if modelType.Name() == "StorageProvider" {
		return m.decryptStorageProvider(value)
	}

	// Generic handling for models with encrypted fields
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// Look for encrypted fields based on naming pattern
		fieldName := field.Name
		if strings.HasPrefix(fieldName, "Encrypted") {
			originalFieldName := strings.TrimPrefix(fieldName, "Encrypted")

			// Get the encrypted field value
			encryptedFieldValue := value.Field(i)
			if !encryptedFieldValue.CanInterface() {
				continue
			}

			// Get encrypted string value
			encryptedValue, ok := encryptedFieldValue.Interface().(string)
			if !ok || encryptedValue == "" {
				continue
			}

			// Decrypt the field
			decryptedValue, err := m.encryptor.DecryptField(encryptedValue)
			if err != nil {
				// Log the error but continue
				fmt.Printf("Warning: failed to decrypt field %s: %v\n", fieldName, err)
				continue
			}

			// Find the corresponding original field
			originalField := value.FieldByName(originalFieldName)

			// If original field exists and can be set, set it
			if originalField.IsValid() && originalField.CanSet() {
				originalField.SetString(decryptedValue)
			}
		}
	}

	return nil
}

// encryptStorageProvider handles encryption for StorageProvider model fields
func (m *EncryptionMiddleware) encryptStorageProvider(value reflect.Value) error {
	// Check if model implements GetSensitiveFields method
	modelInterface := value.Addr().Interface()

	// Type assertion to access the GetSensitiveFields method
	model, ok := modelInterface.(interface {
		GetSensitiveFields() map[string]string
	})

	if !ok {
		return errors.New("StorageProvider model does not implement GetSensitiveFields")
	}

	// Get sensitive fields that need encryption
	sensitiveFields := model.GetSensitiveFields()

	// Encrypt each sensitive field
	for fieldName, fieldValue := range sensitiveFields {
		if fieldValue == "" {
			continue
		}

		// Skip already encrypted values
		if m.encryptor.IsEncrypted(fieldValue) {
			continue
		}

		// Determine the credential type based on field name
		_, credType := encryption.RequiresEncryption(fieldName)

		// Encrypt the value
		encryptedValue, err := m.encryptor.Encrypt(fieldValue, credType)
		if err != nil {
			return fmt.Errorf("failed to encrypt StorageProvider field %s: %w", fieldName, err)
		}

		// Find the corresponding encrypted field
		encryptedFieldName := "Encrypted" + fieldName
		encryptedField := value.FieldByName(encryptedFieldName)

		// Set the encrypted value
		if encryptedField.IsValid() && encryptedField.CanSet() {
			encryptedField.SetString(encryptedValue)

			// Clear the original field if it shouldn't be stored
			originalField := value.FieldByName(fieldName)
			if originalField.IsValid() && originalField.CanSet() {
				// Find the field in the struct type to check its gorm tag
				modelType := reflect.TypeOf(model).Elem()
				if field, found := modelType.FieldByName(fieldName); found && field.Tag.Get("gorm") == "-" {
					originalField.SetString("")
				}
			}
		}
	}

	return nil
}

// decryptStorageProvider handles decryption for StorageProvider model fields
func (m *EncryptionMiddleware) decryptStorageProvider(value reflect.Value) error {
	// Fields to decrypt
	encryptedFields := []string{
		"EncryptedPassword",
		"EncryptedSecretKey",
		"EncryptedClientSecret",
		"EncryptedRefreshToken",
	}

	// Process each encrypted field
	for _, fieldName := range encryptedFields {
		encryptedField := value.FieldByName(fieldName)
		if !encryptedField.IsValid() || !encryptedField.CanInterface() {
			continue
		}

		// Get encrypted value
		encryptedValue, ok := encryptedField.Interface().(string)
		if !ok || encryptedValue == "" {
			continue
		}

		// Decrypt value
		decryptedValue, err := m.encryptor.DecryptField(encryptedValue)
		if err != nil {
			// Log warning but continue with other fields
			fmt.Printf("Warning: failed to decrypt StorageProvider field %s: %v\n", fieldName, err)
			continue
		}

		// Set decrypted value to the original field
		originalFieldName := strings.TrimPrefix(fieldName, "Encrypted")
		originalField := value.FieldByName(originalFieldName)
		if originalField.IsValid() && originalField.CanSet() {
			originalField.SetString(decryptedValue)
		}
	}

	return nil
}
