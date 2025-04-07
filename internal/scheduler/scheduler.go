package scheduler

import (
	"context"
	"fmt"
	"sync"

	// Needed for Job.NextRun update
	"github.com/robfig/cron/v3"
	"github.com/starfleetcptn/gomft/internal/db"
)

// --- Interfaces for Dependencies ---

// SchedulerDB defines the database methods needed directly by Scheduler.
type SchedulerDB interface {
	GetActiveJobs() ([]db.Job, error)
	UpdateJobStatus(job *db.Job) error
}

// SchedulerCron defines the cron methods needed directly by Scheduler.
type SchedulerCron interface {
	AddFunc(spec string, cmd func()) (cron.EntryID, error)
	Remove(id cron.EntryID)
	Entry(id cron.EntryID) cron.Entry
	Stop() context.Context // Changed from Stop() to match cron/v3, returns context
}

// SchedulerLogger defines the logger methods needed directly by Scheduler.
type SchedulerLogger interface {
	LogInfo(format string, v ...interface{})
	LogError(format string, v ...interface{})
	LogDebug(format string, v ...interface{})
	Close()
	RotateLogs() error
	// Println removed - use LogInfo instead
}

// SchedulerJobExecutor defines the job executor methods needed directly by Scheduler.
type SchedulerJobExecutor interface {
	executeJob(jobID uint)
}

// --- Scheduler Implementation ---

type Scheduler struct {
	cron     SchedulerCron         // Use interface
	db       SchedulerDB           // Use interface
	jobMutex *sync.Mutex           // Keep using pointer for shared mutex
	jobs     map[uint]cron.EntryID // Keep using shared map
	logger   SchedulerLogger       // Use interface
	executor SchedulerJobExecutor  // Use interface
}

// New creates a new Scheduler with injected dependencies.
// Initialization of logger, cron instance, executor, etc., should happen outside
// and the required components (or interfaces) passed in.
func New(
	database SchedulerDB,
	cronInstance SchedulerCron,
	logger SchedulerLogger,
	executor SchedulerJobExecutor,
	jobsMap map[uint]cron.EntryID, // Pass in the shared map
	jobMutex *sync.Mutex, // Pass in the shared mutex
) *Scheduler {

	logger.LogInfo("Initializing scheduler") // Use LogInfo instead of Println

	// Cron instance should be started outside and passed in.
	// c := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))
	// c.Start()

	// Dependencies like Notifier, MetadataHandler, TransferExecutor are now
	// dependencies of the JobExecutor passed in, not initialized here.

	s := &Scheduler{
		cron:     cronInstance,
		db:       database,
		jobMutex: jobMutex, // Use the passed-in mutex
		jobs:     jobsMap,  // Use the passed-in map
		logger:   logger,
		executor: executor,
	}

	// Load existing jobs using the injected dependencies
	s.loadJobs()

	return s
}

func (s *Scheduler) loadJobs() {
	s.logger.LogInfo("Loading scheduled jobs")

	// Get all jobs from the database
	jobs, err := s.db.GetActiveJobs() // Calls interface method
	if err != nil {
		s.logger.LogError("Error loading jobs: %v", err)
		return
	}

	// Clear the job map (passed in by reference, so this affects the shared map)
	s.jobMutex.Lock()
	// Re-initialize the map passed by the caller if needed, or assume caller manages it.
	// Let's assume the caller provides a ready-to-use map. We just clear entries for this scheduler instance.
	// for k := range s.jobs { // This would require iterating, simpler to just re-make if needed.
	// 	delete(s.jobs, k)
	// }
	// If the map should be fully reset here:
	// s.jobs = make(map[uint]cron.EntryID) // This replaces the map, might not be desired if shared
	// Let's stick to removing entries managed by this scheduler instance if they existed.
	// However, the original code cleared the *entire* map. Let's replicate that for now.
	for k := range s.jobs {
		delete(s.jobs, k)
	}
	s.jobMutex.Unlock()

	// Initialize job count to track successfully loaded jobs
	loadedCount := 0

	for _, job := range jobs {
		// Create a local copy for the closure
		jobCopy := job
		// Skip disabled jobs
		if !jobCopy.GetEnabled() {
			s.logger.LogInfo("Job %d (%s) is disabled, skipping scheduling", jobCopy.ID, jobCopy.Name)
			continue
		}

		// ScheduleJob now uses the local jobCopy
		if err := s.ScheduleJob(&jobCopy); err != nil {
			s.logger.LogError("Error scheduling job %d: %v", jobCopy.ID, err)
		} else {
			s.logger.LogInfo("Loaded job %d: %s", jobCopy.ID, jobCopy.Name)
			loadedCount++
		}
	}

	s.logger.LogInfo("Loaded %d jobs", loadedCount)
}

func (s *Scheduler) ScheduleJob(job *db.Job) error {
	s.logger.LogDebug("Attempting to schedule job ID %d: %+v", job.ID, job)

	// Use local variable for job ID within the closure
	jobID := job.ID

	s.logger.LogInfo("Scheduling job %d: %s with schedule %s", jobID, job.Name, job.Schedule)

	// Remove existing job if it exists
	s.jobMutex.Lock() // Lock before accessing shared map
	if entryID, exists := s.jobs[jobID]; exists {
		s.logger.LogInfo("Removing existing schedule for job %d", jobID)
		s.cron.Remove(entryID) // Calls interface method
		delete(s.jobs, jobID)
	}
	s.jobMutex.Unlock() // Unlock after accessing shared map

	// Only schedule if job is enabled
	if !job.GetEnabled() {
		s.logger.LogInfo("Job %d is disabled, skipping scheduling", jobID)
		return nil
	}

	// Rely on the cron instance's AddFunc for validation based on its configuration (5 or 6 fields)
	scheduleToUse := job.Schedule // Use the original schedule string
	s.logger.LogDebug("Using schedule '%s' for job %d", scheduleToUse, jobID)
	// Schedule the job using the original schedule string. AddFunc will validate it.
	entryID, err := s.cron.AddFunc(scheduleToUse, func() { // Calls interface method
		s.executor.executeJob(jobID) // Calls interface method
	})
	if err != nil {
		// Log and return a more informative error if AddFunc fails validation
		s.logger.LogError("Error scheduling job %d with schedule '%s': %v", jobID, scheduleToUse, err)
		return fmt.Errorf("invalid cron expression '%s' for the configured scheduler: %w", scheduleToUse, err)
	}
	s.logger.LogDebug("Scheduled job %d with cron entry ID %d", jobID, entryID)

	// Store mapping of job ID to cron entry ID
	s.jobMutex.Lock()
	s.jobs[jobID] = entryID
	s.jobMutex.Unlock()

	// Get next run time
	entry := s.cron.Entry(entryID) // Calls interface method
	nextRunTime := entry.Next      // Capture time before pointer assignment
	job.NextRun = &nextRunTime
	if err := s.db.UpdateJobStatus(job); err != nil { // Calls interface method
		s.logger.LogError("Error updating job status for job %d: %v", jobID, err)
		// Don't return error here? Original code returned error. Let's keep that.
		return err
	}

	return nil
}

func (s *Scheduler) UnscheduleJob(jobID uint) {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	if entryID, exists := s.jobs[jobID]; exists {
		s.logger.LogInfo("Unscheduling job %d (entry ID %d)", jobID, entryID)
		s.cron.Remove(entryID) // Calls interface method
		delete(s.jobs, jobID)
	} else {
		s.logger.LogInfo("Job %d not found in scheduler map, cannot unschedule", jobID)
	}
}

func (s *Scheduler) Stop() {
	s.logger.LogInfo("Stopping scheduler")
	_ = s.cron.Stop() // Calls interface method, ignore context for now
	s.logger.Close()  // Calls interface method
}

// RotateLogs manually triggers log rotation
func (s *Scheduler) RotateLogs() error {
	s.logger.LogInfo("Manually rotating logs")
	return s.logger.RotateLogs() // Calls interface method
}

func (s *Scheduler) RunJobNow(jobID uint) error {
	s.logger.LogInfo("Running job %d now", jobID)
	// Run in a goroutine as before
	go s.executor.executeJob(jobID) // Calls interface method
	return nil
}
