package scheduler

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/starfleetcptn/gomft/internal/db"
)

// JobExecutor handles the execution logic for a single job run.
type JobExecutor struct {
	db               *db.DB
	logger           *Logger
	cron             *cron.Cron
	jobs             map[uint]cron.EntryID // Shared map from Scheduler
	jobMutex         *sync.Mutex           // Shared mutex from Scheduler
	transferExecutor *TransferExecutor     // TransferExecutor component
	notifier         *Notifier             // Notifier component
	// metadataHandler *MetadataHandler // Placeholder if needed directly
}

// NewJobExecutor creates a new JobExecutor.
func NewJobExecutor(
	database *db.DB,
	logger *Logger,
	cron *cron.Cron,
	jobsMap map[uint]cron.EntryID,
	jobMutex *sync.Mutex,
	transferExec *TransferExecutor,
	notify *Notifier,
	// metadata *MetadataHandler,
) *JobExecutor {
	return &JobExecutor{
		db:               database,
		logger:           logger,
		cron:             cron,
		jobs:             jobsMap,
		jobMutex:         jobMutex,
		transferExecutor: transferExec,
		notifier:         notify,
		// metadataHandler: metadata,
	}
}

// executeJob orchestrates the execution of a job by processing its configurations.
func (je *JobExecutor) executeJob(jobID uint) {
	je.logger.LogDebug("Entering executeJob for job ID %d", jobID)
	defer je.logger.LogDebug("Exiting executeJob for job ID %d", jobID)

	je.logger.LogInfo("Starting execution of job %d", jobID)

	// Get job details
	var job db.Job
	if err := je.db.First(&job, jobID).Error; err != nil {
		je.logger.LogError("Error loading job %d: %v", jobID, err)
		return
	}

	je.logger.LogDebug("Loaded job details: %+v", job)

	// Get all configurations associated with this job
	configs, err := je.db.GetConfigsForJob(jobID)
	if err != nil {
		je.logger.LogError("Error loading configurations for job %d: %v", jobID, err)
		return
	}

	je.logger.LogDebug("Loaded %d configurations for job %d", len(configs), jobID)

	if len(configs) == 0 {
		je.logger.LogError("Error: job %d has no associated configurations", jobID)
		return
	}

	// Get the ordered config IDs from the job
	orderedConfigIDs := job.GetConfigIDsList()
	je.logger.LogDebug("Ordered config IDs for job %d: %v", jobID, orderedConfigIDs)

	// Create a map of configs for easy lookup
	configMap := make(map[uint]db.TransferConfig)
	for _, config := range configs {
		configMap[config.ID] = config
	}

	// Process configurations in the specified order
	var orderedConfigs []db.TransferConfig

	// First, add configs in the order specified in the job's ConfigIDs
	for _, configID := range orderedConfigIDs {
		if config, exists := configMap[configID]; exists {
			orderedConfigs = append(orderedConfigs, config)
			delete(configMap, configID) // Remove from map to avoid duplicates
		}
	}

	// Add any remaining configs not in the ordered list (shouldn't happen, but just in case)
	for _, config := range configMap {
		orderedConfigs = append(orderedConfigs, config)
	}

	je.logger.LogInfo("Processing job %d with %d configurations in specified order", jobID, len(orderedConfigs))

	// Log the order of execution
	for i, config := range orderedConfigs {
		je.logger.LogDebug("Execution order %d/%d: Config ID %d (%s)", i+1, len(orderedConfigs), config.ID, config.Name)
	}

	// Update job last run time
	startTime := time.Now()
	job.LastRun = &startTime
	if err := je.db.UpdateJobStatus(&job); err != nil {
		je.logger.LogError("Error updating job last run time for job %d: %v", jobID, err)
	}

	// Process each configuration in the specified order
	for i, config := range orderedConfigs {
		je.processConfiguration(&job, &config, i+1, len(orderedConfigs))
	}

	// Update next run time after execution
	// Need access to the shared jobs map and mutex from Scheduler
	je.jobMutex.Lock()
	entryID, exists := je.jobs[jobID]
	je.jobMutex.Unlock()

	if exists {
		entry := je.cron.Entry(entryID)
		nextRun := entry.Next
		job.NextRun = &nextRun
		je.logger.LogInfo("Next run time for job %d: %v", jobID, nextRun)
		if err := je.db.UpdateJobStatus(&job); err != nil {
			je.logger.LogError("Error updating job next run time for job %d: %v", jobID, err)
		}
	}
}

// processConfiguration processes a single configuration step within a job.
func (je *JobExecutor) processConfiguration(job *db.Job, config *db.TransferConfig, index int, totalConfigs int) {
	je.logger.LogDebug("Processing configuration %d: %+v", config.ID, config)

	je.logger.LogInfo("Processing configuration %d (%d/%d) for job %d: source=%s:%s, dest=%s:%s",
		config.ID,
		index,
		totalConfigs,
		job.ID,
		config.SourceType,
		config.SourcePath,
		config.DestinationType,
		config.DestinationPath,
	)

	// Create job history entry for this configuration
	history := &db.JobHistory{
		JobID:            job.ID,
		ConfigID:         config.ID,
		StartTime:        time.Now(),
		Status:           "running",
		FilesTransferred: 0,
		BytesTransferred: 0,
		ErrorMessage:     "",
	}
	if err := je.db.CreateJobHistory(history); err != nil {
		je.logger.LogError("Error creating job history for job %d, config %d: %v", job.ID, config.ID, err)
		return
	}

	je.logger.LogDebug("Creating job history record: %+v", history)

	// Send webhook notification for job start
	// TODO: Ensure Notifier struct and its methods are correctly defined and initialized
	je.notifier.SendNotifications(job, history, config) // Assuming sendWebhookNotification is a method on Notifier

	// Execute the configuration transfer
	// TODO: Ensure TransferExecutor struct and its methods are correctly defined and initialized
	je.transferExecutor.executeConfigTransfer(*job, *config, history) // Assuming executeConfigTransfer is a method on TransferExecutor
}
