package scheduler

import (
	"github.com/starfleetcptn/gomft/internal/db"
)

// SchedulerInterface defines the interface for job scheduling operations
type SchedulerInterface interface {
	// ScheduleJob schedules a job based on its cron expression
	ScheduleJob(job *db.Job) error

	// RunJobNow runs a job immediately
	RunJobNow(jobID uint) error

	// UnscheduleJob removes a job from the scheduler
	UnscheduleJob(jobID uint)

	// Stop stops the scheduler
	Stop()
}
