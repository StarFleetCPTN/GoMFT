package rotationmodel

import (
	"time"
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

// RotationOptions contains configuration for the key rotation process
type RotationOptions struct {
	// DryRun performs all operations but doesn't save changes to database
	DryRun bool
	// BatchSize sets the number of records to process in each batch
	BatchSize int
	// MaxErrors sets the threshold of errors before aborting
	MaxErrors int
	// Parallelism controls how many models are processed in parallel
	Parallelism int
	// Timeout specifies a maximum duration for the entire operation
	Timeout time.Duration
	// WorkerTimeout specifies maximum duration for a single batch
	WorkerTimeout time.Duration
	// ProgressCallback receives updates on rotation progress
	ProgressCallback func(modelName string, processed, total int)
}

// EncryptionMigrationPlan contains the complete plan for migration
type EncryptionMigrationPlan struct {
	ModelPlans         map[string]*ModelMigrationPlan `json:"model_plans"`
	EstimatedDuration  time.Duration                  `json:"estimated_duration"`
	EstimatedRecords   int                            `json:"estimated_records"`
	RecommendedOptions RotationOptions                `json:"recommended_options"`
}

// ModelMigrationPlan contains migration details for a specific model
type ModelMigrationPlan struct {
	ModelName       string        `json:"model_name"`
	RecordCount     int           `json:"record_count"`
	EstimatedTime   time.Duration `json:"estimated_time"`
	EncryptedFields []string      `json:"encrypted_fields"`
	BatchSizeRec    int           `json:"batch_size_recommendation"`
}
