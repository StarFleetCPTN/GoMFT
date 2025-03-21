package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleJobs handles the GET /jobs route
func (h *Handlers) HandleJobs(c *gin.Context) {
	userID := c.GetUint("userID")

	var jobs []db.Job
	h.DB.Where("created_by = ?", userID).Preload("Config").Find(&jobs)

	// Create a map to store config counts for each job
	configCount := make(map[uint]int)

	// Count configurations for each job
	for _, job := range jobs {
		// Get all configurations for this job
		configs, err := h.DB.GetConfigsForJob(job.ID)
		if err != nil {
			c.Error(fmt.Errorf("error loading configurations for job %d: %v", job.ID, err))
			configCount[job.ID] = 0
		} else {
			configCount[job.ID] = len(configs)
		}
	}

	data := components.JobsData{
		Jobs:        jobs,
		ConfigCount: configCount,
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

	// First try to get the specific config used in this job history record
	configID := jobHistory.ConfigID

	// If no ConfigID is set in the history, fall back to the job's primary ConfigID
	if configID == 0 {
		configID = job.ConfigID
	}

	if err := h.DB.First(&config, configID).Error; err != nil {
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
	userID := c.GetUint("userID")

	// Debug logging
	log.Printf("HandleCreateJob: Form data received: %v", c.Request.PostForm)

	// Parse form data
	var job db.Job
	if err := c.ShouldBind(&job); err != nil {
		c.String(http.StatusBadRequest, "Invalid form data")
		return
	}

	// Debug logging
	log.Printf("HandleCreateJob: Job after binding: %+v", job)

	// Get multiple config IDs from form
	configIDs := c.PostFormArray("config_ids[]")
	if len(configIDs) == 0 {
		c.String(http.StatusBadRequest, "At least one configuration must be selected")
		return
	}

	// Debug logging
	log.Printf("HandleCreateJob: config_ids[]: %v", configIDs)

	// Process config IDs
	var configIDsList []uint

	// Check if we have an explicit order specified
	configOrder := c.PostForm("config_order")
	log.Printf("HandleCreateJob: config_order: %s", configOrder)

	if configOrder != "" {
		// Parse the ordered list
		orderStrings := strings.Split(configOrder, ",")
		log.Printf("HandleCreateJob: order strings: %v", orderStrings)

		for _, configIDStr := range orderStrings {
			configID, err := strconv.ParseUint(configIDStr, 10, 32)
			if err != nil {
				log.Printf("HandleCreateJob: Error parsing config ID: %v", err)
				c.String(http.StatusBadRequest, "Invalid configuration ID format in order")
				return
			}

			// Verify that the config exists and belongs to the user
			var config db.TransferConfig
			if err := h.DB.First(&config, configID).Error; err != nil {
				log.Printf("HandleCreateJob: Invalid config ID: %d, error: %v", configID, err)
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

			configIDsList = append(configIDsList, uint(configID))
		}
	} else {
		// Fall back to unordered config IDs
		log.Printf("HandleCreateJob: No config_order found, using checkbox order")
		for _, configIDStr := range configIDs {
			configID, err := strconv.ParseUint(configIDStr, 10, 32)
			if err != nil {
				c.String(http.StatusBadRequest, "Invalid configuration ID format")
				return
			}

			// Verify that the config exists and belongs to the user
			var config db.TransferConfig
			if err := h.DB.First(&config, configID).Error; err != nil {
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

			configIDsList = append(configIDsList, uint(configID))
		}
	}

	// Debug logging
	log.Printf("HandleCreateJob: Final configIDsList: %v", configIDsList)

	// Set the first config ID for backward compatibility
	if len(configIDsList) > 0 {
		job.ConfigID = configIDsList[0]

		// Verify that the config exists and belongs to the user (using the first config as primary)
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

		// If job name is empty, use the primary config name
		if job.Name == "" {
			job.Name = config.Name
		}
	}

	// Set the config IDs list
	job.SetConfigIDsList(configIDsList)

	// Debug logging
	log.Printf("HandleCreateJob: Job after setting ConfigIDsList: %+v", job)
	log.Printf("HandleCreateJob: Job.ConfigIDs: %s", job.ConfigIDs)

	// Set the boolean fields - handle both "on" and "true" values for checkboxes
	enabledVal := c.Request.FormValue("enabled")
	jobEnabledValue := enabledVal == "on" || enabledVal == "true"
	job.SetEnabled(jobEnabledValue)

	webhookEnabledVal := c.Request.FormValue("webhook_enabled")
	webhookEnabledValue := webhookEnabledVal == "on" || webhookEnabledVal == "true"
	job.SetWebhookEnabled(webhookEnabledValue)

	notifySuccessVal := c.Request.FormValue("notify_on_success")
	notifyOnSuccessValue := notifySuccessVal == "on" || notifySuccessVal == "true"
	job.SetNotifyOnSuccess(notifyOnSuccessValue)

	notifyFailureVal := c.Request.FormValue("notify_on_failure")
	notifyOnFailureValue := notifyFailureVal == "on" || notifyFailureVal == "true"
	job.SetNotifyOnFailure(notifyOnFailureValue)

	// Set created by user
	job.CreatedBy = userID

	// Clear the Config field to prevent GORM from creating a new config
	job.Config = db.TransferConfig{}

	// Create the job
	if err := h.DB.CreateJob(&job); err != nil {
		log.Printf("HandleCreateJob: Error creating job: %v", err)
		c.String(http.StatusInternalServerError, "Failed to create job")
		return
	}

	log.Printf("HandleCreateJob: Job successfully created with ID: %d", job.ID)

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

	// Debug logging
	log.Printf("HandleUpdateJob: Updating job ID: %s", id)
	log.Printf("HandleUpdateJob: Form data received: %v", c.Request.PostForm)

	var job db.Job
	if err := h.DB.First(&job, id).Error; err != nil {
		log.Printf("HandleUpdateJob: Job not found: %v", err)
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
	log.Printf("HandleUpdateJob: Original job: %+v", oldJob)
	log.Printf("HandleUpdateJob: Original job ConfigIDs: %s", oldJob.ConfigIDs)

	// Parse form data
	if err := c.ShouldBind(&job); err != nil {
		log.Printf("HandleUpdateJob: Error binding form data: %v", err)
		c.String(http.StatusBadRequest, "Invalid form data")
		return
	}

	log.Printf("HandleUpdateJob: Job after binding: %+v", job)

	// Get multiple config IDs from form
	configIDs := c.PostFormArray("config_ids[]")
	if len(configIDs) == 0 {
		log.Printf("HandleUpdateJob: No config_ids[] found in form data")
		c.String(http.StatusBadRequest, "At least one configuration must be selected")
		return
	}

	log.Printf("HandleUpdateJob: config_ids[]: %v", configIDs)

	// Process config IDs
	var configIDsList []uint

	// Check if we have an explicit order specified
	configOrder := c.PostForm("config_order")
	log.Printf("HandleUpdateJob: config_order: %s", configOrder)

	if configOrder != "" {
		// Parse the ordered list
		orderStrings := strings.Split(configOrder, ",")
		log.Printf("HandleUpdateJob: order strings: %v", orderStrings)

		for _, configIDStr := range orderStrings {
			configID, err := strconv.ParseUint(configIDStr, 10, 32)
			if err != nil {
				log.Printf("HandleUpdateJob: Error parsing config ID: %v", err)
				c.String(http.StatusBadRequest, "Invalid configuration ID format in order")
				return
			}

			// Verify that the config exists
			var config db.TransferConfig
			if err := h.DB.First(&config, configID).Error; err != nil {
				log.Printf("HandleUpdateJob: Invalid config ID: %d, error: %v", configID, err)
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

			configIDsList = append(configIDsList, uint(configID))
		}
	} else {
		// Fall back to unordered config IDs
		log.Printf("HandleUpdateJob: No config_order found, using checkbox order")
		for _, configIDStr := range configIDs {
			configID, err := strconv.ParseUint(configIDStr, 10, 32)
			if err != nil {
				c.String(http.StatusBadRequest, "Invalid configuration ID format")
				return
			}

			// Verify that the config exists
			var config db.TransferConfig
			if err := h.DB.First(&config, configID).Error; err != nil {
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

			configIDsList = append(configIDsList, uint(configID))
		}
	}

	// Debug logging
	log.Printf("HandleUpdateJob: Final configIDsList: %v", configIDsList)

	// Set the first config ID for backward compatibility
	if len(configIDsList) > 0 {
		job.ConfigID = configIDsList[0]

		// If job name is empty, use the primary config name
		var config db.TransferConfig
		if err := h.DB.First(&config, job.ConfigID).Error; err == nil && job.Name == "" {
			job.Name = config.Name
		}
	}

	// Set the config IDs list
	job.SetConfigIDsList(configIDsList)

	// Debug logging
	log.Printf("HandleUpdateJob: Job after setting ConfigIDsList: %+v", job)
	log.Printf("HandleUpdateJob: Job.ConfigIDs: %s", job.ConfigIDs)

	// Set the boolean fields - handle both "on" and "true" values for checkboxes
	enabledVal := c.Request.FormValue("enabled")
	jobEnabledValue := enabledVal == "on" || enabledVal == "true"
	job.SetEnabled(jobEnabledValue)

	webhookEnabledVal := c.Request.FormValue("webhook_enabled")
	webhookEnabledValue := webhookEnabledVal == "on" || webhookEnabledVal == "true"
	job.SetWebhookEnabled(webhookEnabledValue)

	notifySuccessVal := c.Request.FormValue("notify_on_success")
	notifyOnSuccessValue := notifySuccessVal == "on" || notifySuccessVal == "true"
	job.SetNotifyOnSuccess(notifyOnSuccessValue)

	notifyFailureVal := c.Request.FormValue("notify_on_failure")
	notifyOnFailureValue := notifyFailureVal == "on" || notifyFailureVal == "true"
	job.SetNotifyOnFailure(notifyOnFailureValue)

	// Preserve fields that shouldn't be updated
	job.CreatedBy = oldJob.CreatedBy
	job.ID = oldJob.ID

	// Clear the Config field to prevent GORM from updating or creating a new config
	job.Config = db.TransferConfig{}

	if err := h.DB.UpdateJob(&job); err != nil {
		log.Printf("HandleUpdateJob: Error updating job: %v", err)
		c.String(http.StatusInternalServerError, "Failed to update job")
		return
	}

	log.Printf("HandleUpdateJob: Job successfully updated")

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
		c.Header("Content-Type", "text/html")
		c.String(http.StatusNotFound, "<script>window.notyfInstance.error('Job not found')</script>")
		return
	}

	// Check if user owns this job
	if job.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusForbidden, "<script>window.notyfInstance.error('You do not have permission to run this job')</script>")
			return
		}
	}

	// Determine job name for response
	jobName := job.Name
	if jobName == "" {
		// If job name is empty, try to get config name
		var config db.TransferConfig
		if err := h.DB.First(&config, job.ConfigID).Error; err == nil {
			jobName = config.Name
		} else {
			jobName = fmt.Sprintf("Job #%d", job.ID)
		}
	}

	// Run the job immediately using the scheduler
	if err := h.Scheduler.RunJobNow(job.ID); err != nil {
		c.Header("Content-Type", "text/html")
		errorMsg := fmt.Sprintf("<script>window.notyfInstance.error('Failed to run job: %s')</script>", err.Error())
		c.String(http.StatusInternalServerError, errorMsg)
		return
	}

	// Set custom header with job name for HTMX to use in the toast notification
	c.Header("HX-Job-Name", jobName)
	c.Header("Content-Type", "text/html")

	// Return HTML with JavaScript to trigger the notification
	successScript := fmt.Sprintf("<script>window.notyfInstance.success('Job \"%s\" has been started successfully')</script>", jobName)
	c.String(http.StatusOK, successScript)
}
