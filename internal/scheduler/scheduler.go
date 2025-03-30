package scheduler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/starfleetcptn/gomft/internal/db"
)

type Scheduler struct {
	cron     *cron.Cron
	db       *db.DB
	jobMutex sync.Mutex
	jobs     map[uint]cron.EntryID
	logger   *Logger // Renamed from log
	executor *JobExecutor
	// transfer *TransferExecutor // May be needed by executor
	// notifier *Notifier // May be needed by executor/transfer
	// metadata *MetadataHandler // May be needed by executor/transfer
}

func New(database *db.DB) *Scheduler {
	// Create a new logger
	logger := NewLogger()

	logger.Info.Println("Initializing scheduler")
	c := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger))) // Consider using our logger
	c.Start()

	// Initialize components
	jobsMap := make(map[uint]cron.EntryID)
	var jobMutex sync.Mutex
	notifier := NewNotifier(database, logger)
	metadataHandler := NewMetadataHandler(database, logger)
	transferExecutor := NewTransferExecutor(database, logger, metadataHandler, notifier)
	jobExecutor := NewJobExecutor(database, logger, c, jobsMap, &jobMutex, transferExecutor, notifier)

	s := &Scheduler{
		cron:     c,
		db:       database, // Keep DB here for now for LoadJobs/ScheduleJob, or refactor further
		jobMutex: jobMutex, // Use the initialized mutex
		jobs:     jobsMap,  // Use the initialized map
		logger:   logger,
		executor: jobExecutor, // Assign the initialized executor
	}

	// Load existing jobs
	s.loadJobs()

	return s
}

func (s *Scheduler) loadJobs() {
	s.logger.LogInfo("Loading scheduled jobs")

	// Get all jobs from the database
	jobs, err := s.db.GetActiveJobs()
	if err != nil {
		s.logger.LogError("Error loading jobs: %v", err)
		return
	}

	// Clear the job map to ensure we're starting fresh
	s.jobMutex.Lock()
	s.jobs = make(map[uint]cron.EntryID)
	s.jobMutex.Unlock()

	// Initialize job count to track successfully loaded jobs
	loadedCount := 0

	for _, job := range jobs {
		// Skip disabled jobs
		if !job.GetEnabled() {
			s.logger.LogInfo("Job %d (%s) is disabled, skipping scheduling", job.ID, job.Name)
			continue
		}

		if err := s.ScheduleJob(&job); err != nil {
			s.logger.LogError("Error scheduling job %d: %v", job.ID, err)
		} else {
			s.logger.LogInfo("Loaded job %d: %s", job.ID, job.Name)
			loadedCount++
		}
	}

	s.logger.LogInfo("Loaded %d jobs", loadedCount)
}

func (s *Scheduler) ScheduleJob(job *db.Job) error {
	s.logger.LogDebug("Attempting to schedule job ID %d: %+v", job.ID, job)

	s.logger.LogInfo("Scheduling job %d: %s with schedule %s", job.ID, job.Name, job.Schedule)

	// Remove existing job if it exists
	if entryID, exists := s.jobs[job.ID]; exists {
		s.logger.LogInfo("Removing existing schedule for job %d", job.ID)
		s.cron.Remove(entryID)
		delete(s.jobs, job.ID)
	}

	// Only schedule if job is enabled
	if !job.GetEnabled() {
		s.logger.LogInfo("Job %d is disabled, skipping scheduling", job.ID)
		return nil
	}

	// Convert 5-field cron to 6-field by prepending '0' for seconds
	schedule := job.Schedule
	if len(strings.Fields(schedule)) == 5 {
		schedule = "0 " + schedule
	}

	s.logger.LogDebug("Converted schedule from '%s' to '%s'", job.Schedule, schedule)

	// Validate cron expression
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	if err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", job.Schedule, err)
	}

	s.logger.LogDebug("Validated cron expression '%s' for job %d", schedule, job.ID)

	// Schedule the job
	entryID, err := s.cron.AddFunc(job.Schedule, func() {
		// TODO: Refactor executeJob to be a method on JobExecutor and pass necessary context
		s.executor.executeJob(job.ID)
	})

	if err != nil {
		s.logger.LogError("Error scheduling job %d: %v", job.ID, err)
		return err
	}

	s.logger.LogDebug("Scheduled job %d with cron entry ID %d", job.ID, entryID)

	// Store mapping of job ID to cron entry ID
	s.jobMutex.Lock()
	s.jobs[job.ID] = entryID
	s.jobMutex.Unlock()

	// Get next run time
	entry := s.cron.Entry(entryID)
	job.NextRun = &entry.Next
	if err := s.db.UpdateJobStatus(job); err != nil {
		s.logger.LogError("Error updating job status for job %d: %v", job.ID, err)
		return err
	}

	return nil
}

func (s *Scheduler) UnscheduleJob(jobID uint) {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	if entryID, exists := s.jobs[jobID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, jobID)
	}
}

func (s *Scheduler) Stop() {
	s.logger.LogInfo("Stopping scheduler")
	s.cron.Stop()
	s.logger.Close()
}

// RotateLogs manually triggers log rotation
func (s *Scheduler) RotateLogs() error {
	s.logger.LogInfo("Manually rotating logs")
	return s.logger.RotateLogs()
}

func (s *Scheduler) RunJobNow(jobID uint) error {
	// TODO: Refactor executeJob to be a method on JobExecutor and pass necessary context
	go s.executor.executeJob(jobID)
	return nil
}
