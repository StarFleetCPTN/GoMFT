package audit

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/starfleetcptn/gomft/internal/encryption/rotationmodel"
	"gorm.io/gorm"
)

// For backward compatibility
type RotationOptions = rotationmodel.RotationOptions

// RotationUtility provides comprehensive capabilities for rotating encryption keys
// across multiple database models with detailed auditing and progress tracking
type RotationUtility struct {
	db           *gorm.DB
	oldService   *encryption.EncryptionService
	newService   *encryption.EncryptionService
	auditor      *SecurityAuditor
	monitor      *SecurityMonitor
	options      RotationOptions
	testingHooks map[string]func(interface{}) error
	mu           sync.Mutex
}

// NewRotationUtility creates a new RotationUtility
func NewRotationUtility(
	db *gorm.DB,
	oldService, newService *encryption.EncryptionService,
	auditor *SecurityAuditor,
	monitor *SecurityMonitor,
	options RotationOptions,
) (*RotationUtility, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	if oldService == nil {
		return nil, fmt.Errorf("old encryption service is required")
	}

	if newService == nil {
		return nil, fmt.Errorf("new encryption service is required")
	}

	if auditor == nil {
		auditor = GetGlobalAuditor()
	}

	if monitor == nil {
		monitor = NewSecurityMonitor(auditor)
	}

	// Set default options
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}

	if options.MaxErrors <= 0 {
		options.MaxErrors = 50
	}

	if options.Parallelism <= 0 {
		options.Parallelism = 1
	}

	if options.Timeout <= 0 {
		options.Timeout = 24 * time.Hour // Default long timeout
	}

	if options.WorkerTimeout <= 0 {
		options.WorkerTimeout = 30 * time.Minute
	}

	return &RotationUtility{
		db:           db,
		oldService:   oldService,
		newService:   newService,
		auditor:      auditor,
		monitor:      monitor,
		options:      options,
		testingHooks: make(map[string]func(interface{}) error),
	}, nil
}

// RegisterTestingHook registers a hook for testing purposes
func (r *RotationUtility) RegisterTestingHook(name string, hook func(interface{}) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.testingHooks[name] = hook
}

// runHook runs a testing hook if it exists
func (r *RotationUtility) runHook(name string, data interface{}) error {
	r.mu.Lock()
	hook, exists := r.testingHooks[name]
	r.mu.Unlock()

	if exists && hook != nil {
		return hook(data)
	}
	return nil
}

// RotateKeysForModels performs key rotation for multiple model types with detailed monitoring
func (r *RotationUtility) RotateKeysForModels(ctx context.Context, models []interface{}) (*rotationmodel.RotationStats, error) {
	// Create master context with timeout
	masterCtx, cancel := context.WithTimeout(ctx, r.options.Timeout)
	defer cancel()

	// Track overall stats
	overallStats := &rotationmodel.RotationStats{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	// Create key rotator - we'll implement our own version instead of using keyrotation package
	rotator, err := NewKeyRotator(r.db, r.oldService, r.newService, r.auditor)
	if err != nil {
		return overallStats, fmt.Errorf("failed to create key rotator: %w", err)
	}

	// Apply options
	rotator.SetDryRun(r.options.DryRun)
	rotator.SetBatchSize(r.options.BatchSize)
	rotator.SetMaxErrors(r.options.MaxErrors)

	// Log the start of rotation
	r.auditor.LogKeyRotationEventWithDescription(
		"starting",
		"pending",
		true,
		fmt.Sprintf("Starting key rotation for %d model types (dry run: %v)", len(models), r.options.DryRun),
		0,
	)

	// Process all models (sequentially)
	for _, model := range models {
		// Check if context is canceled
		select {
		case <-masterCtx.Done():
			overallStats.Errors = append(overallStats.Errors, fmt.Sprintf("key rotation aborted: %v", masterCtx.Err()))
			return overallStats, masterCtx.Err()
		default:
			// Continue processing
		}

		// Get model type info
		modelType := reflect.TypeOf(model)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}
		modelName := modelType.Name()

		// Run pre-rotation hook if any
		if err := r.runHook("pre_rotation_"+modelName, model); err != nil {
			overallStats.Errors = append(overallStats.Errors, fmt.Sprintf("pre-rotation hook failed for %s: %v", modelName, err))
			continue
		}

		// Log model rotation start
		r.auditor.LogKeyRotationEventWithDescription(
			"starting",
			"pending",
			true,
			fmt.Sprintf("Starting key rotation for model: %s", modelName),
			0,
		)

		// Create a worker context with timeout
		workerCtx, workerCancel := context.WithTimeout(masterCtx, r.options.WorkerTimeout)

		// Create a goroutine to handle timeouts
		rotationDone := make(chan struct{})
		var modelStats *rotationmodel.RotationStats
		var rotationErr error

		go func() {
			// Perform the actual rotation
			modelStats, rotationErr = rotator.RotateKeys(model, "")
			close(rotationDone)
		}()

		// Wait for rotation to complete or timeout
		select {
		case <-workerCtx.Done():
			if workerCtx.Err() == context.DeadlineExceeded {
				errorMsg := fmt.Sprintf("key rotation for model %s timed out after %v", modelName, r.options.WorkerTimeout)
				overallStats.Errors = append(overallStats.Errors, errorMsg)

				// Log timeout error
				r.auditor.LogKeyRotationEventWithDescription(
					"old",
					"new",
					false,
					errorMsg,
					0,
				)
			}
		case <-rotationDone:
			// Rotation completed
		}

		// Clean up the worker context
		workerCancel()

		// Check for rotation errors
		if rotationErr != nil {
			overallStats.Errors = append(overallStats.Errors, fmt.Sprintf("failed to rotate keys for %s: %v", modelName, rotationErr))

			// Log rotation error
			r.auditor.LogKeyRotationEventWithDescription(
				"old",
				"new",
				false,
				fmt.Sprintf("Key rotation failed for model %s: %v", modelName, rotationErr),
				0,
			)

			continue
		}

		// Update overall stats
		if modelStats != nil {
			overallStats.TotalRecords += modelStats.TotalRecords
			overallStats.ProcessedRecords += modelStats.ProcessedRecords
			overallStats.SkippedRecords += modelStats.SkippedRecords
			overallStats.FailedRecords += modelStats.FailedRecords
			overallStats.Errors = append(overallStats.Errors, modelStats.Errors...)

			// Call progress callback if set
			if r.options.ProgressCallback != nil {
				r.options.ProgressCallback(modelName, modelStats.ProcessedRecords, modelStats.TotalRecords)
			}

			// Log progress
			successRate := 0.0
			if modelStats.TotalRecords > 0 {
				successRate = float64(modelStats.ProcessedRecords) / float64(modelStats.TotalRecords) * 100
			}

			r.auditor.LogKeyRotationEventWithDescription(
				"old",
				"new",
				true,
				fmt.Sprintf("Completed key rotation for model %s: %d/%d records (%.1f%%) processed, %d skipped, %d failed",
					modelName, modelStats.ProcessedRecords, modelStats.TotalRecords, successRate,
					modelStats.SkippedRecords, modelStats.FailedRecords),
				0,
			)
		}

		// Run post-rotation hook if any
		if err := r.runHook("post_rotation_"+modelName, model); err != nil {
			overallStats.Errors = append(overallStats.Errors, fmt.Sprintf("post-rotation hook failed for %s: %v", modelName, err))
		}
	}

	// Complete overall stats
	overallStats.EndTime = time.Now()
	overallStats.ElapsedTime = overallStats.EndTime.Sub(overallStats.StartTime)

	// Calculate overall success rate
	successRate := 0.0
	if overallStats.TotalRecords > 0 {
		successRate = float64(overallStats.ProcessedRecords) / float64(overallStats.TotalRecords) * 100
	}

	// Log completion
	r.auditor.LogKeyRotationEventWithDescription(
		"old",
		"new",
		len(overallStats.Errors) == 0,
		fmt.Sprintf("Completed key rotation for all models: %d/%d records (%.1f%%) processed, %d skipped, %d failed, %d errors in %s",
			overallStats.ProcessedRecords, overallStats.TotalRecords, successRate,
			overallStats.SkippedRecords, overallStats.FailedRecords, len(overallStats.Errors),
			overallStats.ElapsedTime),
		0,
	)

	return overallStats, nil
}

// FindModelsWithEncryptedFields automatically finds all database models with encrypted fields
func (r *RotationUtility) FindModelsWithEncryptedFields() ([]interface{}, error) {
	// This is a placeholder - in a real implementation, we would scan the codebase
	// or database schema to automatically detect models with encrypted fields
	// Since that requires knowledge of the codebase structure, this would be
	// customized for the specific application

	return []interface{}{}, fmt.Errorf("automatic model detection not implemented, provide models explicitly")
}

// ValidateRotation tests the key rotation on sample records without saving changes
func (r *RotationUtility) ValidateRotation(models []interface{}) (map[string]bool, error) {
	results := make(map[string]bool)

	// Save current options to restore later
	originalDryRun := r.options.DryRun
	originalBatchSize := r.options.BatchSize

	// Set temporary options for validation
	r.options.DryRun = true
	r.options.BatchSize = 10 // Test with small batch

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run rotation with dry run mode
	stats, err := r.RotateKeysForModels(ctx, models)

	// Restore original options
	r.options.DryRun = originalDryRun
	r.options.BatchSize = originalBatchSize

	if err != nil {
		return results, fmt.Errorf("validation failed: %w", err)
	}

	// Process results for each model
	for _, model := range models {
		modelType := reflect.TypeOf(model)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}
		modelName := modelType.Name()

		// Check if there were errors for this model
		hasModelErrors := false
		for _, errMsg := range stats.Errors {
			if strings.Contains(errMsg, modelName) {
				hasModelErrors = true
				break
			}
		}

		results[modelName] = !hasModelErrors
	}

	return results, nil
}

// CreateEncryptionMigrationPlan creates a detailed plan for migrating data to a new encryption key
func (r *RotationUtility) CreateEncryptionMigrationPlan(models []interface{}) (*EncryptionMigrationPlan, error) {
	plan := &EncryptionMigrationPlan{
		ModelPlans:         make(map[string]*ModelMigrationPlan),
		EstimatedDuration:  0,
		EstimatedRecords:   0,
		RecommendedOptions: r.options, // Start with current options
	}

	// Calculate record counts for each model
	totalRecords := 0
	for _, model := range models {
		modelType := reflect.TypeOf(model)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}
		modelName := modelType.Name()

		// Get record count
		var count int64
		if err := r.db.Model(model).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to count records for %s: %w", modelName, err)
		}

		encryptedFields := r.identifyEncryptedFields(model)

		// Create model plan
		modelPlan := &ModelMigrationPlan{
			ModelName:       modelName,
			RecordCount:     int(count),
			EstimatedTime:   r.estimateMigrationTime(int(count), len(encryptedFields)),
			EncryptedFields: encryptedFields,
			BatchSizeRec:    r.calculateOptimalBatchSize(int(count)),
		}

		plan.ModelPlans[modelName] = modelPlan
		totalRecords += int(count)
		plan.EstimatedDuration += modelPlan.EstimatedTime
	}

	plan.EstimatedRecords = totalRecords

	// Calculate optimal batch size and parallelism based on total record count
	plan.RecommendedOptions.BatchSize = r.calculateOptimalBatchSize(totalRecords)
	plan.RecommendedOptions.Parallelism = r.calculateOptimalParallelism(totalRecords)

	return plan, nil
}

// identifyEncryptedFields finds all encrypted fields in a model
func (r *RotationUtility) identifyEncryptedFields(model interface{}) []string {
	fields := []string{}

	// Get model value and type
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Skip if not a struct
	if modelType.Kind() != reflect.Struct {
		return fields
	}

	// Scan all fields for encrypted ones
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// Look for fields starting with "Encrypted"
		if strings.HasPrefix(field.Name, "Encrypted") && field.Type.Kind() == reflect.String {
			fields = append(fields, field.Name)
		}
	}

	return fields
}

// calculateOptimalBatchSize determines the optimal batch size based on record count
func (r *RotationUtility) calculateOptimalBatchSize(recordCount int) int {
	// This is a simplistic approach - in a real system, this would be based on
	// benchmarking and system characteristics
	if recordCount < 1000 {
		return 100
	} else if recordCount < 10000 {
		return 250
	} else if recordCount < 100000 {
		return 500
	} else {
		return 1000
	}
}

// calculateOptimalParallelism determines the optimal parallelism level
func (r *RotationUtility) calculateOptimalParallelism(recordCount int) int {
	// Simple heuristic - adjust based on actual system performance
	cpuCount := runtime.NumCPU()

	if recordCount < 10000 {
		return 1
	} else if recordCount < 100000 {
		return min(2, cpuCount)
	} else {
		return min(4, cpuCount)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// estimateMigrationTime provides a rough estimate of time needed for migration
func (r *RotationUtility) estimateMigrationTime(recordCount, fieldCount int) time.Duration {
	// This is a very rough estimate - in a real system, this would be based on
	// benchmarking results and system characteristics

	// Assume roughly 10ms per record per field
	msPerRecordField := 10

	// Calculate total time in milliseconds
	totalTimeMs := recordCount * fieldCount * msPerRecordField

	// Add overhead
	totalTimeMs = int(float64(totalTimeMs) * 1.2) // 20% overhead

	return time.Duration(totalTimeMs) * time.Millisecond
}

// For backward compatibility
type EncryptionMigrationPlan = rotationmodel.EncryptionMigrationPlan

// For backward compatibility
type ModelMigrationPlan = rotationmodel.ModelMigrationPlan
