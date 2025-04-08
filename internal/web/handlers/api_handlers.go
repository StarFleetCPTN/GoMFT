package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// HandleAPILogin handles the POST /api/login route
func (h *Handlers) HandleAPILogin(c *gin.Context) {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&loginData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Get user by email
	var user db.User
	if err := h.DB.Where("email = ?", loginData.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(loginData.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT token
	token, err := h.GenerateJWT(user.ID, user.Email, user.GetIsAdmin())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"is_admin": user.IsAdmin,
		},
	})
}

// HandleAPIConfigs handles the GET /api/configs route
func (h *Handlers) HandleAPIConfigs(c *gin.Context) {
	userID := c.GetUint("userID")

	var configs []db.TransferConfig
	h.DB.Where("created_by = ?", userID).Find(&configs)

	c.JSON(http.StatusOK, gin.H{"configs": configs})
}

// HandleAPIConfig handles the GET /api/configs/:id route
func (h *Handlers) HandleAPIConfig(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")

	var config db.TransferConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}

	// Check if user owns this config
	if config.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}

// HandleAPICreateConfig handles the POST /api/configs route
func (h *Handlers) HandleAPICreateConfig(c *gin.Context) {
	var config db.TransferConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request data: %v", err)})
		return
	}

	userID := c.GetUint("userID")
	config.CreatedBy = userID

	if err := h.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create config: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"config": config})
}

// HandleAPIUpdateConfig handles the PUT /api/configs/:id route
func (h *Handlers) HandleAPIUpdateConfig(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")

	var config db.TransferConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}

	// Check if user owns this config
	if config.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to update this config"})
			return
		}
	}

	// Get the old config values for comparison
	oldConfig := config

	// Bind JSON data to config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request data: %v", err)})
		return
	}

	// Preserve fields that shouldn't be updated
	config.CreatedBy = oldConfig.CreatedBy

	if err := h.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update config: %v", err)})
		return
	}

	// Regenerate the rclone config file
	if err := h.DB.GenerateRcloneConfig(&config); err != nil {
		log.Printf("Warning: Failed to regenerate rclone config after API update: %v", err)
		// Continue anyway, as the config was updated in the database
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}

// HandleAPIDeleteConfig handles the DELETE /api/configs/:id route
func (h *Handlers) HandleAPIDeleteConfig(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")

	var config db.TransferConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}

	// Check if user owns this config
	if config.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to delete this config"})
			return
		}
	}

	// Check if config is in use by any jobs
	var jobCount int64
	h.DB.Model(&db.Job{}).Where("config_id = ?", config.ID).Count(&jobCount)
	if jobCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Config is in use by jobs and cannot be deleted"})
		return
	}

	// Delete config
	if err := h.DB.Delete(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete config: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Config deleted successfully"})
}

// HandleAPIJobs handles the API jobs request
func (h *Handlers) HandleAPIJobs(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API jobs handler stub"})
}

// HandleAPIJob handles the API job request
func (h *Handlers) HandleAPIJob(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API job handler stub"})
}

// HandleAPICreateJob handles the API create job request
func (h *Handlers) HandleAPICreateJob(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API create job handler stub"})
}

// HandleAPIUpdateJob handles the API update job request
func (h *Handlers) HandleAPIUpdateJob(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API update job handler stub"})
}

// HandleAPIDeleteJob handles the API delete job request
func (h *Handlers) HandleAPIDeleteJob(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API delete job handler stub"})
}

// HandleAPIRunJob handles the API run job request
func (h *Handlers) HandleAPIRunJob(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run job: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Job started successfully",
		"jobId":   job.ID,
		"jobName": jobName,
	})
}

// HandleAPIHistory handles the API history request
func (h *Handlers) HandleAPIHistory(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API history handler stub"})
}

// HandleAPIJobRun handles the API job run request
func (h *Handlers) HandleAPIJobRun(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API job run handler stub"})
}

// HandleAPIUsers handles the API users request
func (h *Handlers) HandleAPIUsers(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API users handler stub"})
}

// HandleAPIUser handles the API user request
func (h *Handlers) HandleAPIUser(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API user handler stub"})
}

// HandleAPICreateUser handles the API create user request
func (h *Handlers) HandleAPICreateUser(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API create user handler stub"})
}

// HandleAPIUpdateUser handles the API update user request
func (h *Handlers) HandleAPIUpdateUser(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API update user handler stub"})
}

// HandleAPIDeleteUser handles the API delete user request
func (h *Handlers) HandleAPIDeleteUser(c *gin.Context) {
	// Implementation will be moved from the old handlers.go
	c.JSON(http.StatusOK, gin.H{"message": "API delete user handler stub"})
}
