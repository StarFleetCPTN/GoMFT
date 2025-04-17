// Package encryptionsecurity provides a security framework for encryption operations.
package encryptionsecurity

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/starfleetcptn/gomft/internal/encryption/keyrotation"
	"gorm.io/gorm"
)

// SecurityAuditor defines the interface for the security auditing component
type SecurityAuditor interface {
	LogEncryptionEvent(operation string, fieldType, modelType string, success bool, err error, keyVersion string, userID uint, duration time.Duration)
	LogDecryptionEvent(operation string, fieldType, modelType string, success bool, err error, keyVersion string, userID uint, duration time.Duration)
	LogKeyRotationEvent(oldVersion, newVersion string, success bool, err error, userID uint)
	LogKeyRotationEventWithDescription(oldVersion, newVersion string, success bool, description string, userID uint)
	SetDetailedMode(detailed bool)
	Close() error
}

// SecurityMonitor defines the interface for the security monitoring component
type SecurityMonitor interface {
	SetAlertHandler(handler AlertHandler)
	SetAlertThreshold(eventType string, threshold int)
	AttachToAuditor() func(interface{})
	GenerateReport(startTime, endTime time.Time, writer io.Writer) error
}

// AlertHandler defines the interface for handling security alerts
type AlertHandler interface {
	HandleAlert(interface{})
}

// RotationOptions contains configuration for the key rotation process
type RotationOptions struct {
	BatchSize   int
	Parallelism int
	DryRun      bool
	Timeout     time.Duration
}

// RotationUtility defines the interface for the key rotation component
type RotationUtility interface {
	RotateKeysForModels(ctx context.Context, models []interface{}) (*keyrotation.RotationStats, error)
}

// SecurityTestingFramework defines the interface for the security testing component
type SecurityTestingFramework interface {
	SetOutputDirectory(dir string)
	SetTestingLevel(level int)
	RunAllTests(encryptionService *encryption.EncryptionService) ([]*TestResult, error)
	BenchmarkEncryptionPerformance(service *encryption.EncryptionService, dataSize int, duration time.Duration) (*PerformanceMetrics, error)
}

// TestResult represents the outcome of a security test
type TestResult struct {
	Name        string        `json:"name"`
	Success     bool          `json:"success"`
	ElapsedTime time.Duration `json:"elapsed_time"`
	Error       string        `json:"error,omitempty"`
	Details     string        `json:"details,omitempty"`
}

// PerformanceMetrics contains performance data for encryption operations
type PerformanceMetrics struct {
	OperationsPerSecond float64       `json:"operations_per_second"`
	AverageLatency      time.Duration `json:"average_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	MemoryUsageMB       float64       `json:"memory_usage_mb"`
	CPUUsagePercent     float64       `json:"cpu_usage_percent"`
}

// SecurityFramework provides a unified interface to the security audit, monitoring,
// key rotation, and testing capabilities.
type SecurityFramework struct {
	encryptionService *encryption.EncryptionService
	auditor           SecurityAuditor
	monitor           SecurityMonitor
	rotationUtil      RotationUtility
	testingFramework  SecurityTestingFramework
	db                *gorm.DB
}

// SecurityFrameworkOptions configures the security framework
type SecurityFrameworkOptions struct {
	EnableDetailedAuditing bool
	AuditLogPath           string
	AlertLogPath           string
	EnableMonitoring       bool
	RotationBatchSize      int
	RotationParallelism    int
	EnableTestingFramework bool
	TestOutputDirectory    string
	TestingLevel           int // Using int instead of audit.TestingLevel
}

// DefaultSecurityFrameworkOptions returns sensible defaults
func DefaultSecurityFrameworkOptions() *SecurityFrameworkOptions {
	return &SecurityFrameworkOptions{
		EnableDetailedAuditing: true,
		AuditLogPath:           "logs/encryption_audit.log",
		AlertLogPath:           "logs/encryption_alerts.log",
		EnableMonitoring:       true,
		RotationBatchSize:      100,
		RotationParallelism:    2,
		EnableTestingFramework: true,
		TestOutputDirectory:    "test_results",
		TestingLevel:           0, // BasicTesting
	}
}

// FrameworkDependencies defines the functions needed to create the components of the security framework
type FrameworkDependencies struct {
	CreateAuditor          func(logPath string, enableDetailed bool) (SecurityAuditor, error)
	CreateMonitor          func(auditor SecurityAuditor) SecurityMonitor
	CreateAlertHandler     func(logPath string) (AlertHandler, error)
	CreateTestingFramework func(auditor SecurityAuditor, monitor SecurityMonitor) SecurityTestingFramework
	CreateDummyService     func() (*encryption.EncryptionService, error)
	CreateRotationUtility  func(db *gorm.DB, oldService, newService *encryption.EncryptionService, auditor SecurityAuditor, monitor SecurityMonitor, options RotationOptions) (RotationUtility, error)
}

// NewSecurityFramework creates a new SecurityFramework
func NewSecurityFramework(
	db *gorm.DB,
	encryptionService *encryption.EncryptionService,
	options *SecurityFrameworkOptions,
	deps FrameworkDependencies,
) (*SecurityFramework, error) {
	if encryptionService == nil {
		return nil, fmt.Errorf("encryption service is required")
	}

	// Use default options if none provided
	if options == nil {
		options = DefaultSecurityFrameworkOptions()
	}

	// Create the auditor
	var auditor SecurityAuditor
	var err error

	if options.AuditLogPath != "" {
		// Create directories if they don't exist
		dir := getDirectoryPath(options.AuditLogPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %w", err)
		}
	}

	auditor, err = deps.CreateAuditor(options.AuditLogPath, options.EnableDetailedAuditing)
	if err != nil {
		return nil, fmt.Errorf("failed to create auditor: %w", err)
	}

	// Create the monitor
	monitor := deps.CreateMonitor(auditor)

	// Configure alert handler if path is specified
	if options.AlertLogPath != "" {
		dir := getDirectoryPath(options.AlertLogPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create alert log directory: %w", err)
		}

		alertHandler, err := deps.CreateAlertHandler(options.AlertLogPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create alert handler: %w", err)
		}
		monitor.SetAlertHandler(alertHandler)
	}

	// Set up alert thresholds
	monitor.SetAlertThreshold("decryption_failure", 5)
	monitor.SetAlertThreshold("encryption_failure", 5)
	monitor.SetAlertThreshold("key_rotation", 1)

	// Wire up monitor to auditor
	// This would be implemented by the consumer
	_ = monitor.AttachToAuditor()

	// Create testing framework
	testingFramework := deps.CreateTestingFramework(auditor, monitor)

	if options.EnableTestingFramework {
		// Configure testing framework
		if options.TestOutputDirectory != "" {
			testingFramework.SetOutputDirectory(options.TestOutputDirectory)
		}
		testingFramework.SetTestingLevel(options.TestingLevel)
	}

	// Set up rotation utility if database is provided
	var rotationUtil RotationUtility
	if db != nil {
		// For rotation, we'll need a dummy service for testing initially
		// This will be replaced with actual services during rotation
		dummyService, err := deps.CreateDummyService()
		if err != nil {
			return nil, fmt.Errorf("failed to create dummy encryption service: %w", err)
		}

		// Create rotation options
		rotationOptions := RotationOptions{
			BatchSize:   options.RotationBatchSize,
			Parallelism: options.RotationParallelism,
			DryRun:      false,
			Timeout:     24 * time.Hour,
		}

		// Create rotation utility
		rotationUtil, err = deps.CreateRotationUtility(
			db,
			dummyService,
			dummyService,
			auditor,
			monitor,
			rotationOptions,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create rotation utility: %w", err)
		}
	}

	return &SecurityFramework{
		encryptionService: encryptionService,
		auditor:           auditor,
		monitor:           monitor,
		rotationUtil:      rotationUtil,
		testingFramework:  testingFramework,
		db:                db,
	}, nil
}

// EncryptWithAudit encrypts data with auditing
func (sf *SecurityFramework) EncryptWithAudit(
	data []byte,
	fieldType string,
	modelType string,
	userID uint,
) ([]byte, error) {
	startTime := time.Now()
	encrypted, err := sf.encryptionService.Encrypt(data)
	duration := time.Since(startTime)

	// Use a placeholder for key version if not available in EncryptionService
	keyVersion := "current"
	sf.auditor.LogEncryptionEvent(
		"Encrypt",
		fieldType,
		modelType,
		err == nil,
		err,
		keyVersion,
		userID,
		duration,
	)

	return encrypted, err
}

// DecryptWithAudit decrypts data with auditing
func (sf *SecurityFramework) DecryptWithAudit(
	encryptedData []byte,
	fieldType string,
	modelType string,
	userID uint,
) ([]byte, error) {
	startTime := time.Now()
	decrypted, err := sf.encryptionService.Decrypt(encryptedData)
	duration := time.Since(startTime)

	// Use a placeholder for key version if not available in EncryptionService
	keyVersion := "current"
	sf.auditor.LogDecryptionEvent(
		"Decrypt",
		fieldType,
		modelType,
		err == nil,
		err,
		keyVersion,
		userID,
		duration,
	)

	return decrypted, err
}

// EncryptStringWithAudit encrypts a string with auditing
func (sf *SecurityFramework) EncryptStringWithAudit(
	data string,
	fieldType string,
	modelType string,
	userID uint,
) (string, error) {
	startTime := time.Now()
	encrypted, err := sf.encryptionService.EncryptString(data)
	duration := time.Since(startTime)

	// Use a placeholder for key version if not available in EncryptionService
	keyVersion := "current"
	sf.auditor.LogEncryptionEvent(
		"EncryptString",
		fieldType,
		modelType,
		err == nil,
		err,
		keyVersion,
		userID,
		duration,
	)

	return encrypted, err
}

// DecryptStringWithAudit decrypts a string with auditing
func (sf *SecurityFramework) DecryptStringWithAudit(
	encryptedData string,
	fieldType string,
	modelType string,
	userID uint,
) (string, error) {
	startTime := time.Now()
	decrypted, err := sf.encryptionService.DecryptString(encryptedData)
	duration := time.Since(startTime)

	// Use a placeholder for key version if not available in EncryptionService
	keyVersion := "current"
	sf.auditor.LogDecryptionEvent(
		"DecryptString",
		fieldType,
		modelType,
		err == nil,
		err,
		keyVersion,
		userID,
		duration,
	)

	return decrypted, err
}

// RotateEncryptionKeys rotates encryption keys for models with encrypted fields
func (sf *SecurityFramework) RotateEncryptionKeys(
	ctx context.Context,
	oldService, newService *encryption.EncryptionService,
	models []interface{},
	userID uint,
) (*keyrotation.RotationStats, error) {
	if sf.rotationUtil == nil || sf.db == nil {
		return nil, fmt.Errorf("database and rotation utility are required for key rotation")
	}

	// Check services
	if oldService == nil || newService == nil {
		return nil, fmt.Errorf("both old and new encryption services are required")
	}

	// Log key rotation start
	oldVersion := "previous"
	newVersion := "current"
	sf.auditor.LogKeyRotationEvent(oldVersion, newVersion, true, nil, userID)

	// Perform key rotation
	stats, err := sf.rotationUtil.RotateKeysForModels(ctx, models)

	// Log key rotation completion
	sf.auditor.LogKeyRotationEventWithDescription(
		oldVersion,
		newVersion,
		err == nil,
		fmt.Sprintf("Key rotation completed: processed %d records, failed %d",
			stats.ProcessedRecords, stats.FailedRecords),
		userID,
	)

	return stats, err
}

// RunSecurityTests runs encryption security tests
func (sf *SecurityFramework) RunSecurityTests() ([]*TestResult, error) {
	return sf.testingFramework.RunAllTests(sf.encryptionService)
}

// BenchmarkEncryptionPerformance measures encryption performance
func (sf *SecurityFramework) BenchmarkEncryptionPerformance(
	dataSize int,
	duration time.Duration,
) (*PerformanceMetrics, error) {
	return sf.testingFramework.BenchmarkEncryptionPerformance(
		sf.encryptionService,
		dataSize,
		duration,
	)
}

// GenerateSecurityReport generates a security report
func (sf *SecurityFramework) GenerateSecurityReport(
	startTime, endTime time.Time,
	writer io.Writer,
) error {
	return sf.monitor.GenerateReport(startTime, endTime, writer)
}

// Close properly closes any resources
func (sf *SecurityFramework) Close() error {
	if sf.auditor != nil {
		return sf.auditor.Close()
	}
	return nil
}

// getDirectoryPath extracts the directory path from a file path
func getDirectoryPath(filePath string) string {
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' || filePath[i] == '\\' {
			return filePath[:i]
		}
	}
	return ""
}
