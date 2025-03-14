package scheduler

import (
	"testing"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestMockScheduler_MultiConfig(t *testing.T) {
	// Create a new mock scheduler
	mockScheduler := NewMockScheduler()

	// Create a job with multiple configurations
	job := &db.Job{
		ID:       1,
		Name:     "Multi-Config Test Job",
		Schedule: "*/5 * * * *",
		ConfigID: 1, // Primary config ID
		Enabled:  true,
	}

	// Set multiple config IDs
	job.SetConfigIDsList([]uint{1, 2, 3})

	// Schedule the job
	err := mockScheduler.ScheduleJob(job)
	assert.NoError(t, err)

	// Check if the job is marked as scheduled
	assert.True(t, mockScheduler.ScheduledJobs[job.ID])

	// Verify that the job is detected as having multiple configs
	assert.True(t, mockScheduler.IsJobWithMultipleConfigs(job.ID))

	// Verify the configs associated with the job
	configs := mockScheduler.GetConfigsForJob(job.ID)
	assert.Len(t, configs, 3)
	assert.Contains(t, configs, uint(1))
	assert.Contains(t, configs, uint(2))
	assert.Contains(t, configs, uint(3))

	// Test unscheduling the job
	mockScheduler.UnscheduleJob(job.ID)
	assert.True(t, mockScheduler.UnscheduledJobs[job.ID])
	assert.False(t, mockScheduler.ScheduledJobs[job.ID])

	// Verify the job is no longer tracked in multi-config jobs
	assert.False(t, mockScheduler.IsJobWithMultipleConfigs(job.ID))
	assert.Empty(t, mockScheduler.GetConfigsForJob(job.ID))

	// Test a job with a single config
	singleConfigJob := &db.Job{
		ID:       2,
		Name:     "Single Config Job",
		Schedule: "0 0 * * *",
		ConfigID: 4,
		Enabled:  true,
	}

	// Set a single config ID
	singleConfigJob.SetConfigIDsList([]uint{4})

	// Schedule the job
	err = mockScheduler.ScheduleJob(singleConfigJob)
	assert.NoError(t, err)

	// Not considered a multi-config job if it has only one config
	assert.False(t, mockScheduler.IsJobWithMultipleConfigs(singleConfigJob.ID))

	// Should still contain the single config
	singleConfigs := mockScheduler.GetConfigsForJob(singleConfigJob.ID)
	assert.Len(t, singleConfigs, 1)
	assert.Contains(t, singleConfigs, uint(4))
}
