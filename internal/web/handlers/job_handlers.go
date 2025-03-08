package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleJobs handles the GET /jobs route
func (h *Handlers) HandleJobs(c *gin.Context) {
	userID := c.GetUint("userID")
	
	var jobs []db.Job
	h.DB.Where("created_by = ?", userID).Preload("Config").Find(&jobs)

	data := components.JobsData{
		Jobs: jobs,
	}
	components.Jobs(c, data).Render(c, c.Writer)
}

// HandleJobRunDetails handles the GET /job/:id route
func (h *Handlers) HandleJobRunDetails(c *gin.Context) {
	userID := c.GetUint("userID")
	jobID := c.Param("id")
	
	// Get job history
	var jobHistory db.JobHistory
	if err := h.DB.First(&jobHistory, jobID).Error; err != nil {
		c.String(http.StatusNotFound, "Job not found")
		return
	}
	
	// Get job
	var job db.Job
	if err := h.DB.First(&job, jobHistory.JobID).Error; err != nil {
		c.String(http.StatusNotFound, "Job not found")
		return
	}
	
	// Verify that the user owns this job
	if job.CreatedBy != userID {
		c.String(http.StatusForbidden, "You don't have permission to view this job run")
		return
	}
	
	// Get the config
	var config db.TransferConfig
	if err := h.DB.First(&config, job.ConfigID).Error; err != nil {
		c.String(http.StatusNotFound, "Configuration not found")
		return
	}
	
	data := components.JobRunDetailsData{
		JobHistory: jobHistory,
		Job:        job,
		Config:     config,
	}
	
	components.JobRunDetails(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleNewJob handles the GET /jobs/new route
func (h *Handlers) HandleNewJob(c *gin.Context) {
	// Get available configs for the user
	userID := c.GetUint("userID")
	var configs []db.TransferConfig
	h.DB.Where("created_by = ?", userID).Find(&configs)

	data := components.JobFormData{
		Job:     &db.Job{},
		Configs: configs,
		IsNew:   true,
	}
	components.JobForm(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleEditJob handles the GET /jobs/:id/edit route
func (h *Handlers) HandleEditJob(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	
	var job db.Job
	if err := h.DB.First(&job, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/jobs")
		return
	}

	// Check if user owns this job
	if job.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.Redirect(http.StatusFound, "/jobs")
			return
		}
	}

	// Get available configs for the user
	var configs []db.TransferConfig
	h.DB.Where("created_by = ?", userID).Find(&configs)

	data := components.JobFormData{
		Job:     &job,
		Configs: configs,
		IsNew:   false,
	}
	components.JobForm(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleCreateJob handles the POST /jobs route
func (h *Handlers) HandleCreateJob(c *gin.Context) {
	var job db.Job
	if err := c.ShouldBind(&job); err != nil {
		c.String(http.StatusBadRequest, "Invalid form data")
		return
	}

	userID := c.GetUint("userID")
	job.CreatedBy = userID

	// Verify that the config exists and belongs to the user
	var config db.TransferConfig
	if err := h.DB.First(&config, job.ConfigID).Error; err != nil {
		c.String(http.StatusBadRequest, "Invalid configuration selected")
		return
	}

	// Check if the config belongs to the user
	if config.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.String(http.StatusForbidden, "You do not have permission to use this configuration")
			return
		}
	}

	// If job name is empty, use the config name
	if job.Name == "" {
		job.Name = config.Name
	}

	// Clear the Config field to prevent GORM from creating a new config
	job.Config = db.TransferConfig{}

	// Create the job
	if err := h.DB.CreateJob(&job); err != nil {
		c.String(http.StatusInternalServerError, "Failed to create job")
		return
	}

	// Schedule the job with the scheduler
	if err := h.Scheduler.ScheduleJob(&job); err != nil {
		c.String(http.StatusInternalServerError, "Job created but scheduling failed: "+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/jobs")
}

// HandleUpdateJob handles the PUT /jobs/:id route
func (h *Handlers) HandleUpdateJob(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	
	var job db.Job
	if err := h.DB.First(&job, id).Error; err != nil {
		c.String(http.StatusNotFound, "Job not found")
		return
	}

	// Check if user owns this job
	if job.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.String(http.StatusForbidden, "You do not have permission to update this job")
			return
		}
	}

	// Get the old job values for comparison
	oldJob := job

	// Bind form data to job
	if err := c.ShouldBind(&job); err != nil {
		c.String(http.StatusBadRequest, "Invalid form data")
		return
	}

	// Verify that the config exists and belongs to the user
	var config db.TransferConfig
	if err := h.DB.First(&config, job.ConfigID).Error; err != nil {
		c.String(http.StatusBadRequest, "Invalid configuration selected")
		return
	}

	// Check if the config belongs to the user
	if config.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.String(http.StatusForbidden, "You do not have permission to use this configuration")
			return
		}
	}

	// If job name is empty, use the config name
	if job.Name == "" {
		job.Name = config.Name
	}

	// Preserve fields that shouldn't be updated
	job.CreatedBy = oldJob.CreatedBy
	job.ID = oldJob.ID
	
	// Clear the Config field to prevent GORM from updating or creating a new config
	job.Config = db.TransferConfig{}

	if err := h.DB.UpdateJob(&job); err != nil {
		c.String(http.StatusInternalServerError, "Failed to update job")
		return
	}

	// Reschedule the job with the scheduler
	if err := h.Scheduler.ScheduleJob(&job); err != nil {
		c.String(http.StatusInternalServerError, "Job updated but scheduling failed: "+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/jobs")
}

// HandleDeleteJob handles the DELETE /jobs/:id route
func (h *Handlers) HandleDeleteJob(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	
	var job db.Job
	if err := h.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	// Check if user owns this job
	if job.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to delete this job"})
			return
		}
	}

	// Unschedule the job from the scheduler
	h.Scheduler.UnscheduleJob(job.ID)

	// Delete job
	if err := h.DB.Delete(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job deleted successfully"})
}

// HandleRunJob handles the POST /jobs/:id/run route
func (h *Handlers) HandleRunJob(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	
	var job db.Job
	if err := h.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	// Check if user owns this job
	if job.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to run this job"})
			return
		}
	}

	// Run the job immediately using the scheduler
	if err := h.Scheduler.RunJobNow(job.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run job: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Job started successfully",
		"jobId":   job.ID,
	})
} 