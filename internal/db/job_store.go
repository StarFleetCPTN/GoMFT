package db

import (
	"fmt"
	"log"
)

// --- Job Store Methods ---

// CreateJob creates a new job record
func (db *DB) CreateJob(job *Job) error {
	// Use Omit to prevent GORM from creating a new config
	return db.Omit("Config").Create(job).Error
}

// GetJobs retrieves all jobs for a user, preloading the associated config
func (db *DB) GetJobs(userID uint) ([]Job, error) {
	var jobs []Job
	err := db.Preload("Config").Where("created_by = ?", userID).Find(&jobs).Error
	return jobs, err
}

// GetJob retrieves a single job by ID, preloading the associated config
func (db *DB) GetJob(id uint) (*Job, error) {
	var job Job
	err := db.Preload("Config").First(&job, id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateJob updates an existing job record
func (db *DB) UpdateJob(job *Job) error {
	log.Printf("UpdateJob: Updating job ID: %d, ConfigIDs: %s", job.ID, job.ConfigIDs)

	// Use Omit to prevent GORM from updating or creating a new config
	// Explicitly update fields that can be changed
	return db.Model(&Job{}).
		Where("id = ?", job.ID).
		Omit("Config"). // Omit the nested Config struct
		Updates(map[string]interface{}{
			"name":              job.Name,
			"config_id":         job.ConfigID,  // Update the foreign key if needed
			"config_ids":        job.ConfigIDs, // Explicitly update config_ids string
			"schedule":          job.Schedule,
			"enabled":           job.Enabled,
			"webhook_enabled":   job.WebhookEnabled,
			"webhook_url":       job.WebhookURL,
			"webhook_secret":    job.WebhookSecret,
			"webhook_headers":   job.WebhookHeaders,
			"notify_on_success": job.NotifyOnSuccess,
			"notify_on_failure": job.NotifyOnFailure,
			// Do not update LastRun, NextRun, CreatedBy, CreatedAt, UpdatedAt here
			// GORM handles UpdatedAt automatically
		}).Error
}

// DeleteJob deletes a job and its associated history records
func (db *DB) DeleteJob(id uint) error {
	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Delete associated job history records first
	if err := tx.Where("job_id = ?", id).Delete(&JobHistory{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete job history: %v", err)
	}

	// Delete the job
	if err := tx.Delete(&Job{}, id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete job: %v", err)
	}

	return tx.Commit().Error
}

// UpdateJobStatus updates the LastRun and NextRun fields of a job
func (db *DB) UpdateJobStatus(job *Job) error {
	// Only update specific fields related to run status
	return db.Model(job).Updates(map[string]interface{}{
		"last_run": job.LastRun,
		"next_run": job.NextRun,
	}).Error
}

// GetActiveJobs returns all active (enabled) jobs
func (db *DB) GetActiveJobs() ([]Job, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	var jobs []Job
	// For boolean pointer fields, need to check either NULL (for default) or true value
	err := db.Preload("Config").Where("enabled IS NULL OR enabled = ?", true).Find(&jobs).Error
	return jobs, err
}

// GetConfigsForJob returns all transfer configurations associated with a job, in the order specified by ConfigIDs
func (db *DB) GetConfigsForJob(jobID uint) ([]TransferConfig, error) {
	var job Job
	if err := db.First(&job, jobID).Error; err != nil {
		return nil, fmt.Errorf("failed to get job %d: %w", jobID, err)
	}

	configIDs := job.GetConfigIDsList()
	if len(configIDs) == 0 {
		return []TransferConfig{}, nil // No configs associated
	}

	var configs []TransferConfig
	if err := db.Where("id IN ?", configIDs).Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to get configs for job %d: %w", jobID, err)
	}

	// Order the fetched configs according to the job.ConfigIDs list
	configMap := make(map[uint]TransferConfig, len(configs))
	for _, cfg := range configs {
		configMap[cfg.ID] = cfg
	}

	orderedConfigs := make([]TransferConfig, 0, len(configIDs))
	for _, id := range configIDs {
		if cfg, ok := configMap[id]; ok {
			orderedConfigs = append(orderedConfigs, cfg)
		} else {
			log.Printf("Warning: Config ID %d listed in job %d not found in database", id, jobID)
		}
	}

	return orderedConfigs, nil
}

// --- JobHistory Store Methods ---

// CreateJobHistory creates a new job history record
func (db *DB) CreateJobHistory(history *JobHistory) error {
	return db.Create(history).Error
}

// UpdateJobHistory updates an existing job history record
func (db *DB) UpdateJobHistory(history *JobHistory) error {
	return db.Save(history).Error
}

// GetJobHistory retrieves all history records for a specific job, ordered by start time descending
func (db *DB) GetJobHistory(jobID uint) ([]JobHistory, error) {
	var histories []JobHistory
	err := db.Where("job_id = ?", jobID).Order("start_time desc").Find(&histories).Error
	return histories, err
}
