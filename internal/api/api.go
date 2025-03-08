package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/auth"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/scheduler"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Email string `json:"email" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID       uint   `json:"id"`
	Email string `json:"email"`
}

func InitializeRoutes(router *gin.Engine, database *db.DB, scheduler *scheduler.Scheduler, jwtSecret string) {
	api := router.Group("/api")

	// Auth routes
	auth := api.Group("/auth")
	{
		auth.POST("/register", handleRegister(database))
		auth.POST("/login", handleLogin(database, jwtSecret))
		auth.POST("/logout", handleLogout())
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(authMiddleware(database, jwtSecret))
	{
		// Transfer config routes
		protected.GET("/configs", handleListConfigs(database))
		protected.POST("/configs", handleCreateConfig(database))
		protected.GET("/configs/:id", handleGetConfig(database))
		protected.PUT("/configs/:id", handleUpdateConfig(database))
		protected.DELETE("/configs/:id", handleDeleteConfig(database))

		// Job routes
		protected.GET("/jobs", handleListJobs(database))
		protected.POST("/jobs", handleCreateJob(database, scheduler))
		protected.GET("/jobs/:id", handleGetJob(database))
		protected.PUT("/jobs/:id", handleUpdateJob(database, scheduler))
		protected.DELETE("/jobs/:id", handleDeleteJob(database, scheduler))
		protected.POST("/jobs/:id/run", handleRunJob(database, scheduler))
		protected.POST("/jobs/:id/enable", handleEnableJob(database, scheduler))
		protected.POST("/jobs/:id/disable", handleDisableJob(database, scheduler))

		// History routes
		protected.GET("/jobs/:id/history", handleGetJobHistory(database))
		protected.GET("/history", handleListHistory(database))
	}
}

func handleRegister(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if email already exists
		if _, err := database.GetUserByEmail(req.Email); err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		// Create user
		user := &db.User{
			Email:     req.Email,
			PasswordHash: string(hashedPassword),
		}

		if err := database.CreateUser(user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
	}
}

func handleLogin(database *db.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Generate JWT token
		token, err := auth.GenerateToken(user.ID, user.Email, jwtSecret, 24*time.Hour)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, LoginResponse{
			Token: token,
			User: UserResponse{
				ID:       user.ID,
				Email: user.Email,
			},
		})
	}
}

func handleLogout() gin.HandlerFunc {
	return func(c *gin.Context) {
		// JWT tokens are stateless, so we don't need to do anything server-side
		// The client should discard the token
		c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
	}
}

func authMiddleware(database *db.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Check if the header has the Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be in the format 'Bearer {token}'"})
			c.Abort()
			return
		}

		// Validate token
		tokenString := parts[1]
		claims, err := auth.ValidateToken(tokenString, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set user ID in context
		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

func handleListConfigs(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userID := c.GetUint("userID")

		configs, err := database.GetTransferConfigs(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch configs"})
			return
		}

		c.JSON(http.StatusOK, configs)
	}
}

func handleCreateConfig(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var config db.TransferConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Set user ID
		config.CreatedBy = c.GetUint("userID")

		if err := database.CreateTransferConfig(&config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
			return
		}

		c.JSON(http.StatusCreated, config)
	}
}

func handleGetConfig(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing config ID"})
			return
		}

		var configID uint
		if _, err := fmt.Sscanf(id, "%d", &configID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
			return
		}

		config, err := database.GetTransferConfig(configID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
			return
		}

		// Check if user has access to this config
		if config.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		c.JSON(http.StatusOK, config)
	}
}

func handleUpdateConfig(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing config ID"})
			return
		}

		var configID uint
		if _, err := fmt.Sscanf(id, "%d", &configID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
			return
		}

		// Get existing config
		existingConfig, err := database.GetTransferConfig(configID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
			return
		}

		// Check if user has access to this config
		if existingConfig.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		// Bind updated fields
		var updatedConfig db.TransferConfig
		if err := c.ShouldBindJSON(&updatedConfig); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Update fields but preserve ID and CreatedBy
		updatedConfig.ID = existingConfig.ID
		updatedConfig.CreatedBy = existingConfig.CreatedBy
		updatedConfig.CreatedAt = existingConfig.CreatedAt

		if err := database.UpdateTransferConfig(&updatedConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
			return
		}

		c.JSON(http.StatusOK, updatedConfig)
	}
}

func handleDeleteConfig(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing config ID"})
			return
		}

		var configID uint
		if _, err := fmt.Sscanf(id, "%d", &configID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
			return
		}

		// Get existing config to check ownership
		config, err := database.GetTransferConfig(configID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
			return
		}

		// Check if user has access to this config
		if config.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		if err := database.DeleteTransferConfig(configID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Config deleted successfully"})
	}
}

func handleListJobs(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userID := c.GetUint("userID")

		jobs, err := database.GetJobs(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch jobs"})
			return
		}

		c.JSON(http.StatusOK, jobs)
	}
}

func handleCreateJob(database *db.DB, scheduler *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var job db.Job
		if err := c.ShouldBindJSON(&job); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Set user ID
		job.CreatedBy = c.GetUint("userID")

		// Validate config exists and user has access
		_, err := database.GetTransferConfig(job.ConfigID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
			return
		}

		// Check if user has access to this config
		config, err := database.GetTransferConfig(job.ConfigID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
			return
		}
		if config.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		if err := database.CreateJob(&job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
			return
		}

		// Schedule the job if enabled
		if job.Enabled {
			if err := scheduler.ScheduleJob(&job); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to schedule job"})
				return
			}
		}

		c.JSON(http.StatusCreated, job)
	}
}

func handleGetJob(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job ID"})
			return
		}

		var jobID uint
		if _, err := fmt.Sscanf(id, "%d", &jobID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		job, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		// Check if user has access to this job
		if job.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		c.JSON(http.StatusOK, job)
	}
}

func handleUpdateJob(database *db.DB, scheduler *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job ID"})
			return
		}

		var jobID uint
		if _, err := fmt.Sscanf(id, "%d", &jobID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		// Get existing job
		existingJob, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		// Check if user has access to this job
		if existingJob.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		// Bind updated fields
		var updatedJob db.Job
		if err := c.ShouldBindJSON(&updatedJob); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Update fields but preserve ID and CreatedBy
		updatedJob.ID = existingJob.ID
		updatedJob.CreatedBy = existingJob.CreatedBy
		updatedJob.CreatedAt = existingJob.CreatedAt

		// Validate config exists and user has access
		if updatedJob.ConfigID != existingJob.ConfigID {
			_, err := database.GetTransferConfig(updatedJob.ConfigID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
				return
			}
			// Check if user has access to this config
			config, err := database.GetTransferConfig(updatedJob.ConfigID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
				return
			}
			if config.CreatedBy != c.GetUint("userID") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
				return
			}
		}

		// Check if schedule or enabled status changed
		scheduleChanged := updatedJob.Schedule != existingJob.Schedule || updatedJob.Enabled != existingJob.Enabled

		if err := database.UpdateJob(&updatedJob); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update job"})
			return
		}

		// Update the scheduler if needed
		if scheduleChanged {
			if updatedJob.Enabled {
				if err := scheduler.ScheduleJob(&updatedJob); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update job schedule"})
					return
				}
			} else {
				scheduler.UnscheduleJob(updatedJob.ID)
			}
		}

		c.JSON(http.StatusOK, updatedJob)
	}
}

func handleDeleteJob(database *db.DB, scheduler *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job ID"})
			return
		}

		var jobID uint
		if _, err := fmt.Sscanf(id, "%d", &jobID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		// Get existing job to check ownership
		job, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		// Check if user has access to this job
		if job.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		// Remove from scheduler first
		scheduler.UnscheduleJob(jobID)

		if err := database.DeleteJob(jobID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete job"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Job deleted successfully"})
	}
}

func handleRunJob(database *db.DB, scheduler *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job ID"})
			return
		}

		var jobID uint
		if _, err := fmt.Sscanf(id, "%d", &jobID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		// Get existing job
		job, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		// Check if user has access to this job
		if job.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		// Run the job immediately
		if err := scheduler.RunJobNow(jobID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run job: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Job started successfully"})
	}
}

func handleEnableJob(database *db.DB, scheduler *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job ID"})
			return
		}

		var jobID uint
		if _, err := fmt.Sscanf(id, "%d", &jobID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		// Get existing job
		job, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		// Check if user has access to this job
		if job.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		// Update job status
		job.Enabled = true
		if err := database.UpdateJob(job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update job"})
			return
		}

		// Add to scheduler
		if err := scheduler.ScheduleJob(job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to schedule job"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Job enabled successfully"})
	}
}

func handleDisableJob(database *db.DB, scheduler *scheduler.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job ID"})
			return
		}

		var jobID uint
		if _, err := fmt.Sscanf(id, "%d", &jobID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		// Get existing job
		job, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		// Check if user has access to this job
		if job.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		// Update job status
		job.Enabled = false
		if err := database.UpdateJob(job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update job"})
			return
		}

		// Remove from scheduler
		scheduler.UnscheduleJob(jobID)

		c.JSON(http.StatusOK, gin.H{"message": "Job disabled successfully"})
	}
}

func handleGetJobHistory(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job ID"})
			return
		}

		var jobID uint
		if _, err := fmt.Sscanf(id, "%d", &jobID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		// Get existing job to check ownership
		_, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		// Check if user has access to this job
		job, err := database.GetJob(jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		if job.CreatedBy != c.GetUint("userID") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}

		history, err := database.GetJobHistory(jobID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch job history"})
			return
		}

		c.JSON(http.StatusOK, history)
	}
}

func handleListHistory(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userID := c.GetUint("userID")

		// TODO: Implement pagination
		// For now, just return the most recent 100 history entries for the user's jobs
		var history []db.JobHistory
		err := database.DB.
			Joins("JOIN jobs ON job_histories.job_id = jobs.id").
			Where("jobs.created_by = ?", userID).
			Order("start_time DESC").
			Limit(100).
			Find(&history).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch history"})
			return
		}

		c.JSON(http.StatusOK, history)
	}
}
