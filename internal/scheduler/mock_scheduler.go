package scheduler

import (
	"github.com/starfleetcptn/gomft/internal/db"
)

// MockScheduler is a mock implementation of a scheduler for testing
type MockScheduler struct {
	ScheduledJobs      map[uint]bool
	UnscheduledJobs    map[uint]bool
	RunJobsNow         map[uint]bool
	ScheduleJobErr     error
	RunJobNowErr       error
	UnscheduleJobCalls int
	MultiConfigJobs    map[uint][]uint // Track jobs with multiple configs (job ID -> config IDs)
}

// NewMockScheduler creates a new mock scheduler
func NewMockScheduler() *MockScheduler {
	return &MockScheduler{
		ScheduledJobs:   make(map[uint]bool),
		UnscheduledJobs: make(map[uint]bool),
		RunJobsNow:      make(map[uint]bool),
		MultiConfigJobs: make(map[uint][]uint),
	}
}

// ScheduleJob mocks scheduling a job
func (m *MockScheduler) ScheduleJob(job *db.Job) error {
	if m.ScheduleJobErr != nil {
		return m.ScheduleJobErr
	}

	if job.GetEnabled() {
		m.ScheduledJobs[job.ID] = true
		delete(m.UnscheduledJobs, job.ID)
	} else {
		m.UnscheduledJobs[job.ID] = true
		delete(m.ScheduledJobs, job.ID)
	}

	// Track jobs with multiple configurations
	if job.ConfigIDs != "" {
		m.MultiConfigJobs[job.ID] = job.GetConfigIDsList()
	}

	return nil
}

// RunJobNow mocks running a job immediately
func (m *MockScheduler) RunJobNow(jobID uint) error {
	if m.RunJobNowErr != nil {
		return m.RunJobNowErr
	}

	m.RunJobsNow[jobID] = true

	// In a real implementation, this would execute the job
	// But for testing, we just record that it was called
	return nil
}

// UnscheduleJob mocks unscheduling a job
func (m *MockScheduler) UnscheduleJob(jobID uint) {
	m.UnscheduleJobCalls++
	m.UnscheduledJobs[jobID] = true
	delete(m.ScheduledJobs, jobID)
	delete(m.MultiConfigJobs, jobID)
}

// Stop mocks stopping the scheduler
func (m *MockScheduler) Stop() {
	// Nothing to do
}

// RotateLogs mocks log rotation
func (m *MockScheduler) RotateLogs() error {
	return nil
}

// IsJobWithMultipleConfigs checks if a job is scheduled with multiple configs
func (m *MockScheduler) IsJobWithMultipleConfigs(jobID uint) bool {
	configs, exists := m.MultiConfigJobs[jobID]
	return exists && len(configs) > 1
}

// GetConfigsForJob returns the configs for a job
func (m *MockScheduler) GetConfigsForJob(jobID uint) []uint {
	return m.MultiConfigJobs[jobID]
}
