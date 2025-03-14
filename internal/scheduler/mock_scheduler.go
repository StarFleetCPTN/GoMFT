package scheduler

import (
	"github.com/starfleetcptn/gomft/internal/db"
)

// MockScheduler implements the Scheduler interface for testing
type MockScheduler struct {
	ScheduledJobs      map[uint]bool
	UnscheduledJobs    map[uint]bool
	RunJobsNow         map[uint]bool
	ScheduleJobErr     error
	RunJobNowErr       error
	UnscheduleJobCalls int
}

// NewMockScheduler creates a new mock scheduler
func NewMockScheduler() *MockScheduler {
	return &MockScheduler{
		ScheduledJobs:   make(map[uint]bool),
		UnscheduledJobs: make(map[uint]bool),
		RunJobsNow:      make(map[uint]bool),
	}
}

// ScheduleJob mocks scheduling a job
func (m *MockScheduler) ScheduleJob(job *db.Job) error {
	if m.ScheduleJobErr != nil {
		return m.ScheduleJobErr
	}

	if job.Enabled {
		m.ScheduledJobs[job.ID] = true
		delete(m.UnscheduledJobs, job.ID)
	} else {
		m.UnscheduledJobs[job.ID] = true
		delete(m.ScheduledJobs, job.ID)
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
}

// Stop mocks stopping the scheduler
func (m *MockScheduler) Stop() {
	// Nothing to do
}
