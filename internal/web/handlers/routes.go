package handlers

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all the routes for the web interface
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	// Register error handlers first
	h.RegisterErrorHandlers(router)

	// Public routes
	router.GET("/", h.HandleHome)
	router.GET("/login", h.HandleLoginPage)
	router.POST("/login", h.HandleLogin)
	router.GET("/login/verify", h.Handle2FAVerifyPage)
	router.POST("/login/verify", h.Handle2FAVerify)
	router.GET("/login/backup-code", h.Handle2FABackupCodePage)
	router.GET("/forgot-password", h.HandleForgotPasswordPage)
	router.POST("/forgot-password", h.HandleForgotPassword)
	router.GET("/reset-password", h.HandleResetPasswordPage)
	router.POST("/reset-password", h.HandleResetPassword)

	// External Authentication Provider routes for login page
	router.GET("/auth/providers", h.GetAuthProviders)
	router.GET("/auth/provider/:id", h.HandleAuthProviderInit)
	router.GET("/auth/callback", h.HandleAuthProviderCallback)

	// Protected routes
	authorized := router.Group("/")
	authorized.Use(h.AuthMiddleware())

	// Password change route - only accessed from profile page
	authorized.POST("/change-password", h.HandleChangePassword)

	// 2FA routes - under profile
	authorized.GET("/profile/2fa/setup", h.Handle2FASetup)
	authorized.POST("/profile/2fa/verify", h.Handle2FAVerifySetup)
	authorized.POST("/profile/2fa/disable", h.Handle2FADisable)
	authorized.GET("/profile/2fa/backup-codes", h.Handle2FABackupCodes)
	authorized.POST("/profile/2fa/regenerate-codes", h.Handle2FARegenerateCodes)

	// Add notifications routes
	authorized.GET("/notifications", h.HandleNotifications)
	authorized.GET("/notifications/dropdown", h.HandleLoadNotifications)
	authorized.GET("/notifications/count", h.HandleNotificationCount)
	authorized.POST("/notifications/:id/read", h.HandleMarkNotificationAsRead)
	authorized.POST("/notifications/mark-all-read", h.HandleMarkAllNotificationsAsRead)

	// Storage provider routes
	authorized.GET("/storage-providers", h.HandleListStorageProviders)
	authorized.GET("/storage-providers/new", h.HandleNewStorageProvider)
	authorized.POST("/storage-providers", h.HandleCreateStorageProvider)
	authorized.GET("/storage-providers/:id", h.HandleEditStorageProvider)
	authorized.PUT("/storage-providers/:id", h.HandleUpdateStorageProvider)
	authorized.POST("/storage-providers/:id", h.HandleUpdateStorageProvider)
	authorized.DELETE("/storage-providers/:id", h.HandleDeleteStorageProvider)
	authorized.POST("/storage-providers/:id/test", h.HandleTestStorageProvider)
	authorized.POST("/storage-providers/:id/duplicate", h.HandleDuplicateStorageProvider)
	
	// Google Drive authentication routes for storage providers
	authorized.GET("/storage-providers/:id/gdrive-auth", h.HandleStorageProviderGDriveAuth)
	authorized.GET("/storage-providers/gdrive-callback", h.HandleStorageProviderGDriveAuthCallback)
	authorized.GET("/storage-providers/gdrive-token", h.HandleStorageProviderGDriveTokenProcess)
	
	// Google Drive headless authentication routes for storage providers
	authorized.GET("/storage-providers/:id/gdrive-headless-auth", h.HandleStorageProviderGDriveHeadlessAuth)
	authorized.POST("/storage-providers/gdrive-headless-token", h.HandleStorageProviderGDriveHeadlessTokenSubmit)

	{
		authorized.GET("/dashboard", h.HandleDashboard)
		authorized.GET("/configs", h.HandleConfigs)
		authorized.GET("/configs/new", h.HandleNewConfig)
		authorized.GET("/configs/:id", h.HandleEditConfig)
		authorized.POST("/configs", h.HandleCreateConfig)
		authorized.PUT("/configs/:id", h.HandleUpdateConfig)
		authorized.POST("/configs/:id", h.HandleUpdateConfig)
		authorized.DELETE("/configs/:id", h.HandleDeleteConfig)
		authorized.POST("/configs/:id/duplicate", h.HandleDuplicateConfig)
		authorized.POST("/configs/test-connection", h.HandleTestProviderConnection)

		// Path validation endpoint
		authorized.GET("/check-path", h.HandleCheckPath)

		// Google Drive authentication routes
		authorized.GET("/configs/:id/gdrive-auth", h.HandleGDriveAuth)
		authorized.GET("/configs/gdrive-callback", h.HandleGDriveAuthCallback)
		authorized.GET("/configs/gdrive-token", h.HandleGDriveTokenProcess)

		// Duplicate rclone endpoints for web interface to access - same path as API
		rcloneHandler := NewRcloneHandler(h.DB)
		authorized.GET("api/rclone/commands", func(c *gin.Context) { rcloneHandler.RcloneCommandOptions(c) })
		authorized.GET("api/rclone/command-flags", func(c *gin.Context) { rcloneHandler.RcloneCommandFlags(c) })
		authorized.GET("api/rclone/command/:id/usage", func(c *gin.Context) { rcloneHandler.RcloneCommandUsage(c) })

		authorized.GET("/jobs", h.HandleJobs)
		authorized.GET("/jobs/new", h.HandleNewJob)
		authorized.GET("/jobs/:id", h.HandleEditJob)
		authorized.POST("/jobs", h.HandleCreateJob)
		authorized.PUT("/jobs/:id", h.HandleUpdateJob)
		authorized.POST("/jobs/:id", h.HandleUpdateJob)
		authorized.DELETE("/jobs/:id", h.HandleDeleteJob)
		authorized.POST("/jobs/:id/duplicate", h.HandleDuplicateJob)
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
	admin.Use(h.AuthMiddleware())
	{
		// Main admin dashboard - requires admin role
		// admin.GET("", h.AdminMiddleware(), h.HandleAdminDashboard)

		// User management routes
		userGroup := admin.Group("/users")
		userGroup.Use(h.PermissionMiddleware("users.view"))
		{
			userGroup.GET("", h.HandleUsers)
			userGroup.GET("/new", h.PermissionMiddleware("users.create"), h.HandleNewUser)
			userGroup.POST("", h.PermissionMiddleware("users.create"), h.HandleCreateUser)
			userGroup.GET("/:id/edit", h.PermissionMiddleware("users.edit"), h.HandleEditUser)
			userGroup.PUT("/:id", h.PermissionMiddleware("users.edit"), h.HandleUpdateUser)
			userGroup.DELETE("/:id", h.PermissionMiddleware("users.delete"), h.HandleDeleteUser)
			userGroup.PUT("/:id/toggle-lock", h.PermissionMiddleware("users.edit"), h.AdminToggleLockUser)
		}

		// Admin routes for role management
		adminRoles := admin.Group("/roles")
		adminRoles.Use(h.PermissionMiddleware("roles.admin"))
		{
			adminRoles.GET("", h.HandleRoles)
			adminRoles.GET("/new", h.HandleNewRole)
			adminRoles.GET("/:id", h.HandleEditRole)
			adminRoles.GET("/:id/edit", h.HandleRoles)
			adminRoles.POST("", h.HandleCreateRole)
			adminRoles.POST("/:id", h.HandleUpdateRole)
			adminRoles.PUT("/:id", h.HandleUpdateRole)
			adminRoles.DELETE("/:id", h.HandleDeleteRole)
		}

		// Audit log routes
		auditGroup := admin.Group("/audit")
		auditGroup.Use(h.PermissionMiddleware("audit.view"))
		{
			auditGroup.GET("", h.HandleAuditLogs)
			auditGroup.GET("/export", h.PermissionMiddleware("audit.export"), h.HandleExportAuditLogs)
		}

		// Log viewer routes
		logsGroup := admin.Group("/logs")
		logsGroup.Use(h.PermissionMiddleware("logs.view"))
		{
			logsGroup.GET("", h.HandleLogViewer)
			logsGroup.GET("/ws", h.HandleLogStream)
		}

		// System settings routes
		settingsGroup := admin.Group("/settings")
		settingsGroup.Use(h.PermissionMiddleware("system.settings"))
		{
			settingsGroup.GET("", h.HandleSettings)
			// settingsGroup.GET("/backups", h.HandleBackupsPage)
			// settingsGroup.POST("/backups", h.HandleCreateBackup)
			// settingsGroup.GET("/logs", h.HandleLogsPage)
			// settingsGroup.POST("/logs/download", h.HandleDownloadLogs)
			// settingsGroup.DELETE("/logs", h.HandlePurgeLogs)

			// Auth Provider routes
			authProviderGroup := settingsGroup.Group("/auth-providers")
			authProviderGroup.GET("", h.AuthProvidersPage)
			authProviderGroup.GET("/new", h.NewAuthProviderPage)
			authProviderGroup.POST("", h.HandleCreateAuthProvider)
			authProviderGroup.GET("/:id/edit", h.EditAuthProviderPage)
			authProviderGroup.POST("/:id", h.HandleUpdateAuthProvider)
			authProviderGroup.DELETE("/:id", h.HandleDeleteAuthProvider)
			authProviderGroup.POST("/:id/test", h.HandleTestAuthProviderConnection)

			// Notification routes
			settingsGroup.GET("/notifications", h.HandleNotificationsPage)
			settingsGroup.GET("/notifications/new", h.HandleNewNotificationPage)
			settingsGroup.GET("/notifications/:id/edit", h.HandleEditNotificationPage)
			settingsGroup.POST("/notifications", h.HandleCreateNotificationService)
			settingsGroup.PUT("/notifications/:id", h.HandleUpdateNotificationService)
			settingsGroup.DELETE("/notifications/:id", h.HandleDeleteNotificationService)
			settingsGroup.POST("/notifications/test", h.HandleTestNotification)

			settingsGroup.POST("/general", h.HandleSettings)  // Placeholder for future implementation
			settingsGroup.POST("/security", h.HandleSettings) // Placeholder for future implementation
		}

		// Database tools routes
		dbGroup := admin.Group("/database")
		dbGroup.Use(h.PermissionMiddleware("system.backup"))
		{
			dbGroup.GET("", h.HandleDatabaseTools)
			dbGroup.POST("/backup-database", h.HandleBackupDatabase)
			dbGroup.POST("/restore-database", h.PermissionMiddleware("system.restore"), h.HandleRestoreDatabase)
			dbGroup.GET("/restore-database/:filename", h.PermissionMiddleware("system.restore"), h.HandleRestoreDatabase)
			dbGroup.GET("/download-backup/:filename", h.HandleDownloadBackup)
			dbGroup.POST("/delete-backup/:filename", h.HandleDeleteBackup)
			dbGroup.GET("/refresh-backups", h.HandleRefreshBackups)
			dbGroup.POST("/vacuum-database", h.HandleVacuumDatabase)
			dbGroup.POST("/clear-job-history", h.HandleClearJobHistory)
			dbGroup.GET("/export-configs", h.PermissionMiddleware("system.export"), h.HandleExportConfigs)
			dbGroup.GET("/export-jobs", h.PermissionMiddleware("system.export"), h.HandleExportJobs)
		}

	}

	// API routes
	api := router.Group("/api")
	{
		api.POST("/login", h.HandleAPILogin)

		// Protected API routes
		apiAuthorized := api.Group("/")
		apiAuthorized.Use(h.APIAuthMiddleware())
		{
			// Storage providers options endpoints for dropdown selection
			apiAuthorized.GET("/storage-providers/options", h.HandleStorageProviderOptions)

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

				// Auth providers API routes
				apiAdmin.GET("/auth-providers", h.HandleAPIUsers)             // Placeholder for now
				apiAdmin.GET("/auth-providers/:id", h.HandleAPIUser)          // Placeholder for now
				apiAdmin.POST("/auth-providers", h.HandleAPICreateUser)       // Placeholder for now
				apiAdmin.PUT("/auth-providers/:id", h.HandleAPIUpdateUser)    // Placeholder for now
				apiAdmin.DELETE("/auth-providers/:id", h.HandleAPIDeleteUser) // Placeholder for now
				apiAdmin.POST("/auth-providers/:id/test", h.HandleAPIUser)    // Placeholder for now
			}
		}
	}
}
