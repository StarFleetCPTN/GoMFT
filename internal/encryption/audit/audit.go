package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/starfleetcptn/gomft/internal/encryption"
)

// EventType represents the type of encryption-related event
type EventType string

// Event types for encryption operations
const (
	EventEncrypt           EventType = "encrypt"
	EventDecrypt           EventType = "decrypt"
	EventKeyAccess         EventType = "key_access"
	EventKeyRotation       EventType = "key_rotation"
	EventKeyGeneration     EventType = "key_generation"
	EventDecryptionFailure EventType = "decryption_failure"
	EventEncryptionFailure EventType = "encryption_failure"
)

// SecurityLevel represents the severity/importance of an audit event
type SecurityLevel string

// Security levels for events
const (
	LevelInfo    SecurityLevel = "info"
	LevelWarning SecurityLevel = "warning"
	LevelAlert   SecurityLevel = "alert"
	LevelError   SecurityLevel = "error"
)

// AuditEvent represents a single encryption-related security event
type AuditEvent struct {
	Timestamp   time.Time     `json:"timestamp"`
	EventType   EventType     `json:"event_type"`
	Level       SecurityLevel `json:"level"`
	Operation   string        `json:"operation"`
	FieldType   string        `json:"field_type,omitempty"`
	ModelType   string        `json:"model_type,omitempty"`
	Description string        `json:"description"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	KeyVersion  string        `json:"key_version,omitempty"`
	UserID      uint          `json:"user_id,omitempty"`
	RemoteIP    string        `json:"remote_ip,omitempty"`
	Duration    int64         `json:"duration_ns,omitempty"` // Operation duration in nanoseconds
}

// SecurityAuditor is responsible for logging security-related events
type SecurityAuditor struct {
	enabled       bool
	logWriter     io.Writer
	errorWriter   io.Writer
	mutex         sync.Mutex
	detailedMode  bool
	logFilePath   string
	errorFilePath string
}

// New creates a new SecurityAuditor with default configuration
func New() (*SecurityAuditor, error) {
	return &SecurityAuditor{
		enabled:      true,
		logWriter:    os.Stdout, // Default to stdout for regular logs
		errorWriter:  os.Stderr, // Default to stderr for error logs
		detailedMode: false,
	}, nil
}

// NewWithFileLogging creates a new SecurityAuditor with file-based logging
func NewWithFileLogging(logFilePath, errorFilePath string) (*SecurityAuditor, error) {
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	var errorWriter io.Writer
	if errorFilePath == logFilePath {
		errorWriter = logFile
	} else {
		errorFile, err := os.OpenFile(errorFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logFile.Close()
			return nil, fmt.Errorf("failed to open error log file: %w", err)
		}
		errorWriter = errorFile
	}

	return &SecurityAuditor{
		enabled:       true,
		logWriter:     logFile,
		errorWriter:   errorWriter,
		logFilePath:   logFilePath,
		errorFilePath: errorFilePath,
		detailedMode:  false,
	}, nil
}

// Close properly closes any open resources
func (a *SecurityAuditor) Close() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if we need to close file writers
	if closer, ok := a.logWriter.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			return err
		}
	}

	// Don't close errorWriter if it's the same as logWriter
	if a.errorFilePath != a.logFilePath {
		if closer, ok := a.errorWriter.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Enable turns on the auditor
func (a *SecurityAuditor) Enable() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.enabled = true
}

// Disable turns off the auditor
func (a *SecurityAuditor) Disable() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.enabled = false
}

// SetDetailedMode toggles detailed logging mode
func (a *SecurityAuditor) SetDetailedMode(detailed bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.detailedMode = detailed
}

// IsEnabled returns whether auditing is enabled
func (a *SecurityAuditor) IsEnabled() bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.enabled
}

// LogEvent records a security event to the audit log
func (a *SecurityAuditor) LogEvent(event AuditEvent) {
	if !a.IsEnabled() {
		return
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Ensure timestamp is set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Convert the event to JSON
	jsonData, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(a.errorWriter, "Error marshaling audit event: %v\n", err)
		return
	}

	// Choose the right writer based on event level
	writer := a.logWriter
	if event.Level == LevelError || event.Level == LevelAlert {
		writer = a.errorWriter
	}

	// Write to the appropriate log
	fmt.Fprintln(writer, string(jsonData))
}

// LogEncryptionEvent logs an encryption operation event
func (a *SecurityAuditor) LogEncryptionEvent(operation string, fieldType, modelType string, success bool, err error, keyVersion string, userID uint, duration time.Duration) {
	if !a.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp:  time.Now(),
		EventType:  EventEncrypt,
		Level:      LevelInfo,
		Operation:  operation,
		FieldType:  fieldType,
		ModelType:  modelType,
		Success:    success,
		KeyVersion: keyVersion,
		UserID:     userID,
		Duration:   duration.Nanoseconds(),
	}

	if !success {
		event.EventType = EventEncryptionFailure
		event.Level = LevelWarning
		if err != nil {
			event.Error = encryption.SanitizeError(err.Error())
		}
	}

	a.LogEvent(event)
}

// LogDecryptionEvent logs a decryption operation event
func (a *SecurityAuditor) LogDecryptionEvent(operation string, fieldType, modelType string, success bool, err error, keyVersion string, userID uint, duration time.Duration) {
	if !a.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp:  time.Now(),
		EventType:  EventDecrypt,
		Level:      LevelInfo,
		Operation:  operation,
		FieldType:  fieldType,
		ModelType:  modelType,
		Success:    success,
		KeyVersion: keyVersion,
		UserID:     userID,
		Duration:   duration.Nanoseconds(),
	}

	if !success {
		event.EventType = EventDecryptionFailure
		event.Level = LevelWarning
		if err != nil {
			event.Error = encryption.SanitizeError(err.Error())
		}
	}

	a.LogEvent(event)
}

// LogKeyAccessEvent logs when an encryption key is accessed
func (a *SecurityAuditor) LogKeyAccessEvent(keyVersion string, success bool, err error, userID uint) {
	if !a.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp:  time.Now(),
		EventType:  EventKeyAccess,
		Level:      LevelInfo,
		Operation:  "key_access",
		Success:    success,
		KeyVersion: keyVersion,
		UserID:     userID,
	}

	if !success {
		event.Level = LevelAlert
		if err != nil {
			event.Error = encryption.SanitizeError(err.Error())
		}
	}

	// Key access failures are security-critical and should be logged at a higher level
	if !success {
		event.Description = "Failed key access attempt"
	}

	a.LogEvent(event)
}

// LogKeyRotationEvent logs when encryption keys are rotated
func (a *SecurityAuditor) LogKeyRotationEvent(oldVersion, newVersion string, success bool, err error, userID uint) {
	if !a.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp:   time.Now(),
		EventType:   EventKeyRotation,
		Level:       LevelInfo,
		Operation:   "key_rotation",
		Description: fmt.Sprintf("Key rotation from version %s to %s", oldVersion, newVersion),
		Success:     success,
		KeyVersion:  newVersion,
		UserID:      userID,
	}

	if !success {
		event.Level = LevelError
		if err != nil {
			event.Error = encryption.SanitizeError(err.Error())
		}
	}

	a.LogEvent(event)
}

// LogKeyRotationEventWithDescription logs when encryption keys are rotated with a custom description
func (a *SecurityAuditor) LogKeyRotationEventWithDescription(oldVersion, newVersion string, success bool, description string, userID uint) {
	if !a.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp:   time.Now(),
		EventType:   EventKeyRotation,
		Level:       LevelInfo,
		Operation:   "key_rotation",
		Description: description,
		Success:     success,
		KeyVersion:  newVersion,
		UserID:      userID,
	}

	if !success {
		event.Level = LevelError
	}

	a.LogEvent(event)
}

// LogKeyGenerationEvent logs when a new encryption key is generated
func (a *SecurityAuditor) LogKeyGenerationEvent(keyVersion string, success bool, err error, userID uint) {
	if !a.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp:   time.Now(),
		EventType:   EventKeyGeneration,
		Level:       LevelInfo,
		Operation:   "key_generation",
		Description: "New encryption key generated",
		Success:     success,
		KeyVersion:  keyVersion,
		UserID:      userID,
	}

	if !success {
		event.Level = LevelError
		if err != nil {
			event.Error = encryption.SanitizeError(err.Error())
		}
	}

	a.LogEvent(event)
}

// global is the default security auditor instance
var global *SecurityAuditor
var globalOnce sync.Once

// GetGlobalAuditor returns the global security auditor instance
func GetGlobalAuditor() *SecurityAuditor {
	globalOnce.Do(func() {
		var err error
		global, err = New()
		if err != nil {
			// Fall back to a disabled auditor if there's an error
			global = &SecurityAuditor{enabled: false}
		}
	})
	return global
}

// InitializeWithFileLogging initializes the global auditor with file logging
func InitializeWithFileLogging(logFilePath, errorFilePath string) error {
	auditor, err := NewWithFileLogging(logFilePath, errorFilePath)
	if err != nil {
		return err
	}

	globalOnce.Do(func() {
		global = auditor
	})

	// If global auditor was already initialized, replace it
	if global != auditor {
		if closer, ok := global.logWriter.(io.Closer); ok {
			closer.Close()
		}
		if global.errorFilePath != global.logFilePath {
			if closer, ok := global.errorWriter.(io.Closer); ok {
				closer.Close()
			}
		}
		global = auditor
	}

	return nil
}
