package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleHistory handles the GET /history route
func (h *Handlers) HandleHistory(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get pagination parameters
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil {
		pageSize = 10
	}
	// Limit page size options
	if pageSize != 10 && pageSize != 25 && pageSize != 50 && pageSize != 100 {
		pageSize = 10
	}

	// Get search term
	searchTerm := c.Query("search")

	// Build the query
	query := h.DB.Model(&db.JobHistory{}).
		Joins("JOIN jobs ON jobs.id = job_histories.job_id").
		Joins("JOIN transfer_configs ON transfer_configs.id = jobs.config_id").
		Where("jobs.created_by = ?", userID)

	// Apply search if provided
	if searchTerm != "" {
		query = query.Where("transfer_configs.name LIKE ? OR job_histories.status LIKE ?",
			"%"+searchTerm+"%", "%"+searchTerm+"%")
	}

	// Count total matching records for pagination
	var total int64
	query.Count(&total)

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	// Ensure page is within bounds
	if page > totalPages {
		page = totalPages
	}

	// Get paginated results
	var history []db.JobHistory
	offset := (page - 1) * pageSize

	query.Offset(offset).
		Limit(pageSize).
		Preload("Job.Config").
		Order("start_time desc").
		Find(&history)

	// If we got no results and we're not on page 1, redirect to page 1
	// Only do this for non-HTMX requests to avoid navigation issues
	isHtmxRequest := c.GetHeader("HX-Request") == "true"
	if len(history) == 0 && page > 1 && total > 0 && !isHtmxRequest {
		redirectURL := fmt.Sprintf("/history?page=1&pageSize=%d", pageSize)
		if searchTerm != "" {
			redirectURL += fmt.Sprintf("&search=%s", url.QueryEscape(searchTerm))
		}
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	data := components.HistoryData{
		History:     history,
		CurrentPage: page,
		TotalPages:  totalPages,
		SearchTerm:  searchTerm,
		PageSize:    pageSize,
		Total:       int(total),
	}

	// If this is an HTMX request, only render the history content component
	if isHtmxRequest {
		components.HistoryContent(c, data).Render(c, c.Writer)
	} else {
		components.History(c, data).Render(c, c.Writer)
	}
}

// HandleDashboardData handles the GET /dashboard/data route
func (h *Handlers) HandleDashboardData(c *gin.Context) {
	// Get recent job runs
	var recentRuns []db.JobHistory
	if err := h.DB.Preload("Job").Order("start_time desc").Limit(5).Find(&recentRuns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve recent runs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recent_runs": recentRuns,
	})
}

// HandleDashboardJobsData handles the GET /dashboard/jobs route
func (h *Handlers) HandleDashboardJobsData(c *gin.Context) {
	// Get active jobs
	var activeJobs []db.Job
	if err := h.DB.Where("enabled = ?", true).Find(&activeJobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve active jobs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active_jobs": activeJobs,
	})
}

// HandleDashboardHistoryData handles the GET /dashboard/history route
func (h *Handlers) HandleDashboardHistoryData(c *gin.Context) {
	// Get job history stats
	var successCount int64
	var failureCount int64
	var pendingCount int64

	h.DB.Model(&db.JobHistory{}).Where("status = ?", "success").Count(&successCount)
	h.DB.Model(&db.JobHistory{}).Where("status = ?", "failure").Count(&failureCount)
	h.DB.Model(&db.JobHistory{}).Where("status = ?", "pending").Count(&pendingCount)

	c.JSON(http.StatusOK, gin.H{
		"success_count": successCount,
		"failure_count": failureCount,
		"pending_count": pendingCount,
	})
}

// RegisterRoutes registers all the routes for the web interface
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	// Public routes
	router.GET("/", h.HandleHome)
	router.GET("/login", h.HandleLoginPage)
	router.POST("/login", h.HandleLogin)
	router.GET("/forgot-password", h.HandleForgotPasswordPage)
	router.POST("/forgot-password", h.HandleForgotPassword)
	router.GET("/reset-password", h.HandleResetPasswordPage)
	router.POST("/reset-password", h.HandleResetPassword)

	// Protected routes
	authorized := router.Group("/")
	authorized.Use(h.AuthMiddleware())

	// Password change route - only accessed from profile page
	authorized.POST("/change-password", h.HandleChangePassword)

	{
		authorized.GET("/dashboard", h.HandleDashboard)
		authorized.GET("/configs", h.HandleConfigs)
		authorized.GET("/configs/new", h.HandleNewConfig)
		authorized.GET("/configs/:id", h.HandleEditConfig)
		authorized.POST("/configs", h.HandleCreateConfig)
		authorized.PUT("/configs/:id", h.HandleUpdateConfig)
		authorized.POST("/configs/:id", h.HandleUpdateConfig) // Add POST route for form submission
		authorized.DELETE("/configs/:id", h.HandleDeleteConfig)
		authorized.GET("/jobs", h.HandleJobs)
		authorized.GET("/jobs/new", h.HandleNewJob)
		authorized.GET("/jobs/:id", h.HandleEditJob)
		authorized.POST("/jobs", h.HandleCreateJob)
		authorized.PUT("/jobs/:id", h.HandleUpdateJob)
		authorized.POST("/jobs/:id", h.HandleUpdateJob) // Add POST route for form submission
		authorized.DELETE("/jobs/:id", h.HandleDeleteJob)
		authorized.POST("/jobs/:id/run", h.HandleRunJob)
		authorized.GET("/history", h.HandleHistory)
		authorized.GET("/job-runs/:id", h.HandleJobRunDetails)
		authorized.GET("/profile", h.HandleProfile)
		authorized.POST("/profile/theme", h.HandleUpdateTheme)
		authorized.POST("/logout", h.HandleLogout)

		// File metadata routes
		fileMetadataHandler := &FileMetadataHandler{DB: h.DB}
		fileMetadataHandler.Register(authorized)

		// AJAX routes for dashboard
		authorized.GET("/dashboard/data", h.HandleDashboardData)
		authorized.GET("/dashboard/jobs", h.HandleDashboardJobsData)
		authorized.GET("/dashboard/history", h.HandleDashboardHistoryData)

		// Test connection routes
		authorized.POST("/test-connection", h.HandleTestConnection)
		authorized.POST("/test-sftp-connection", h.HandleTestSFTPConnection)
		authorized.POST("/browse-directory", h.HandleBrowseDirectory)
	}

	// Admin-only routes
	admin := router.Group("/admin")
	admin.Use(h.AuthMiddleware(), h.AdminMiddleware())
	{
		admin.GET("/users", h.HandleUsers)
		admin.GET("/users/new", h.HandleNewUser)
		admin.POST("/users", h.HandleCreateUser)
		admin.DELETE("/users/:id", h.HandleDeleteUser)
		admin.GET("/register", h.HandleRegisterPage)
		admin.POST("/register", h.HandleRegister)

		// Admin tools routes
		admin.GET("/tools", h.HandleAdminTools)
		admin.POST("/backup-database", h.HandleBackupDatabase)
		admin.POST("/restore-database", h.HandleRestoreDatabase)
		admin.POST("/restore-database/:filename", h.HandleRestoreDatabaseByFilename)
		admin.GET("/export-configs", h.HandleExportConfigs)
		admin.GET("/export-jobs", h.HandleExportJobs)
		admin.POST("/clear-job-history", h.HandleClearJobHistory)
		admin.POST("/vacuum-database", h.HandleVacuumDatabase)
		admin.GET("/download-backup/:filename", h.HandleDownloadBackup)
		admin.DELETE("/delete-backup/:filename", h.HandleDeleteBackup)
		admin.GET("/refresh-backups", h.HandleRefreshBackups)
	}

	// API routes
	api := router.Group("/api")
	{
		api.POST("/login", h.HandleAPILogin)

		// Protected API routes
		apiAuthorized := api.Group("/")
		apiAuthorized.Use(h.APIAuthMiddleware())
		{
			// Config endpoints
			apiAuthorized.GET("/configs", h.HandleAPIConfigs)
			apiAuthorized.GET("/configs/:id", h.HandleAPIConfig)
			apiAuthorized.POST("/configs", h.HandleAPICreateConfig)
			apiAuthorized.PUT("/configs/:id", h.HandleAPIUpdateConfig)
			apiAuthorized.DELETE("/configs/:id", h.HandleAPIDeleteConfig)

			// Job endpoints
			apiAuthorized.GET("/jobs", h.HandleAPIJobs)
			apiAuthorized.GET("/jobs/:id", h.HandleAPIJob)
			apiAuthorized.POST("/jobs", h.HandleAPICreateJob)
			apiAuthorized.PUT("/jobs/:id", h.HandleAPIUpdateJob)
			apiAuthorized.DELETE("/jobs/:id", h.HandleAPIDeleteJob)
			apiAuthorized.POST("/jobs/:id/run", h.HandleAPIRunJob)

			// History endpoints
			apiAuthorized.GET("/history", h.HandleAPIHistory)
			apiAuthorized.GET("/job-runs/:id", h.HandleAPIJobRun)

			// Admin-only API routes
			apiAdmin := apiAuthorized.Group("/admin")
			apiAdmin.Use(h.APIAdminMiddleware())
			{
				// User management
				apiAdmin.GET("/users", h.HandleAPIUsers)
				apiAdmin.GET("/users/:id", h.HandleAPIUser)
				apiAdmin.POST("/users", h.HandleAPICreateUser)
				apiAdmin.PUT("/users/:id", h.HandleAPIUpdateUser)
				apiAdmin.DELETE("/users/:id", h.HandleAPIDeleteUser)
			}
		}
	}
}
