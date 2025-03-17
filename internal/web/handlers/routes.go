package handlers

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all the routes for the web interface
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	// Public routes
	router.GET("/", h.HandleHome)
	router.GET("/login", h.HandleLoginPage)
	router.POST("/login", h.HandleLogin)
	router.GET("/login/verify", h.Handle2FAVerifyPage)
	router.POST("/login/verify", h.Handle2FAVerify)
	router.GET("/forgot-password", h.HandleForgotPasswordPage)
	router.POST("/forgot-password", h.HandleForgotPassword)
	router.GET("/reset-password", h.HandleResetPasswordPage)
	router.POST("/reset-password", h.HandleResetPassword)

	// Protected routes
	authorized := router.Group("/")
	authorized.Use(h.AuthMiddleware())

	// Password change route - only accessed from profile page
	authorized.POST("/change-password", h.HandleChangePassword)

	// 2FA routes - under profile
	authorized.GET("/profile/2fa/setup", h.Handle2FASetup)
	authorized.POST("/profile/2fa/verify", h.Handle2FAVerifySetup)
	authorized.POST("/profile/2fa/disable", h.Handle2FADisable)

	{
		authorized.GET("/dashboard", h.HandleDashboard)
		authorized.GET("/configs", h.HandleConfigs)
		authorized.GET("/configs/new", h.HandleNewConfig)
		authorized.GET("/configs/:id", h.HandleEditConfig)
		authorized.POST("/configs", h.HandleCreateConfig)
		authorized.PUT("/configs/:id", h.HandleUpdateConfig)
		authorized.POST("/configs/:id", h.HandleUpdateConfig)
		authorized.DELETE("/configs/:id", h.HandleDeleteConfig)

		// Path validation endpoint
		authorized.GET("/check-path", h.HandleCheckPath)

		// Google Drive authentication routes
		authorized.GET("/configs/:id/gdrive-auth", h.HandleGDriveAuth)
		authorized.GET("/configs/gdrive-callback", h.HandleGDriveAuthCallback)
		authorized.GET("/configs/gdrive-token", h.HandleGDriveTokenProcess)

		authorized.GET("/jobs", h.HandleJobs)
		authorized.GET("/jobs/new", h.HandleNewJob)
		authorized.GET("/jobs/:id", h.HandleEditJob)
		authorized.POST("/jobs", h.HandleCreateJob)
		authorized.PUT("/jobs/:id", h.HandleUpdateJob)
		authorized.POST("/jobs/:id", h.HandleUpdateJob)
		authorized.DELETE("/jobs/:id", h.HandleDeleteJob)
		authorized.POST("/jobs/:id/run", h.HandleRunJob)
		authorized.GET("/history", h.HandleHistory)
		authorized.GET("/job-runs/:id", h.HandleJobRunDetails)
		authorized.GET("/profile", h.HandleProfile)
		authorized.POST("/profile/theme", h.HandleUpdateTheme)
		authorized.POST("/logout", h.HandleLogout)

		// File metadata routes
		fileMetadataHandler := &FileMetadataHandler{DB: h.DB}
		fileGroup := authorized.Group("/files")
		fileGroup.GET("", fileMetadataHandler.ListFileMetadata)
		fileGroup.GET("/:id", fileMetadataHandler.GetFileMetadataDetails)
		fileGroup.GET("/job/:job_id", fileMetadataHandler.GetFileMetadataForJob)
		fileGroup.GET("/search", fileMetadataHandler.SearchFileMetadata)
		fileGroup.GET("/search/partial", fileMetadataHandler.HandleFileMetadataSearchPartial)
		fileGroup.DELETE("/:id", fileMetadataHandler.DeleteFileMetadata)
		fileGroup.GET("/partial", fileMetadataHandler.HandleFileMetadataPartial)

		// AJAX routes for dashboard
		authorized.GET("/dashboard/data", h.HandleDashboardData)
		authorized.GET("/dashboard/jobs", h.HandleDashboardJobsData)
		authorized.GET("/dashboard/history", h.HandleDashboardHistoryData)

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

		// Log viewer routes
		admin.GET("/logs/refresh", h.HandleRefreshLogs)
		admin.GET("/logs/view/:fileName", h.HandleViewLog)
		admin.GET("/logs/download/:fileName", h.HandleDownloadLog)
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
