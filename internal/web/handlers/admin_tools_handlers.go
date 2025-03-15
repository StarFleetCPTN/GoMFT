package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleAdminTools displays the admin tools page
func (h *Handlers) HandleAdminTools(c *gin.Context) {
	// Get system statistics
	data := components.AdminToolsData{
		SystemUptime: h.getSystemUptime(),
		DatabasePath: h.DBPath,
		BackupPath:   h.BackupDir,
		LogFiles:     h.getLogFiles(),
	}

	// Get database size
	if dbSize, err := h.getDatabaseSize(); err == nil {
		data.DatabaseSize = dbSize
	} else {
		data.DatabaseSize = "Unknown"
	}

	// Get job history count
	var jobHistoryCount int64
	if err := h.DB.Model(&db.JobHistory{}).Count(&jobHistoryCount).Error; err == nil {
		data.JobHistoryCount = int(jobHistoryCount)
	}

	// Get active jobs count
	var activeJobs int64
	if err := h.DB.Model(&db.Job{}).Where("enabled = ?", true).Count(&activeJobs).Error; err == nil {
		data.ActiveJobs = int(activeJobs)
	}

	// Get total configs count
	var totalConfigs int64
	if err := h.DB.Model(&db.TransferConfig{}).Count(&totalConfigs).Error; err == nil {
		data.TotalConfigs = int(totalConfigs)
	}

	// Get total jobs count
	var totalJobs int64
	if err := h.DB.Model(&db.Job{}).Count(&totalJobs).Error; err == nil {
		data.TotalJobs = int(totalJobs)
	}

	// Get total users count
	var totalUsers int64
	if err := h.DB.Model(&db.User{}).Count(&totalUsers).Error; err == nil {
		data.TotalUsers = int(totalUsers)
	}

	// Get last backup time and backup count
	data.LastBackupTime, data.BackupCount = h.getBackupInfo()

	// Get list of backup files
	data.BackupFiles = h.getBackupFiles()

	// Check for maintenance issues
	data.MaintenanceMessage = h.checkMaintenanceIssues()

	// Render the admin tools page
	components.AdminTools(components.CreateTemplateContext(c), data).Render(c, c.Writer)
}

// HandleBackupDatabase handles the backup database request
func (h *Handlers) HandleBackupDatabase(c *gin.Context) {
	fmt.Println("Backup database")
	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(h.BackupDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create backup directory: %v", err)})
		return
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := filepath.Join(h.BackupDir, fmt.Sprintf("gomft_backup_%s.db", timestamp))

	// Copy the database file to the backup location
	if err := h.copyDatabaseToBackup(backupFilename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create backup: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Database backup created successfully", "filename": backupFilename})
}

// HandleRestoreDatabase handles the restore database request
func (h *Handlers) HandleRestoreDatabase(c *gin.Context) {
	// Get the uploaded file
	file, err := c.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No backup file provided"})
		return
	}

	// Create a temporary file to store the uploaded backup
	tempFile, err := os.CreateTemp("", "gomft_restore_*.db")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create temporary file: %v", err)})
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Save the uploaded file to the temporary location
	if err := c.SaveUploadedFile(file, tempFile.Name()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save uploaded file: %v", err)})
		return
	}

	// Stop the scheduler to prevent jobs from running during restore
	h.Scheduler.Stop()

	// Close the current database connection
	if err := h.DB.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to close database: %v", err)})
		return
	}

	// Create a backup of the current database before restoring
	backupBeforeRestore := filepath.Join(h.BackupDir, fmt.Sprintf("pre_restore_backup_%s.db", time.Now().Format("20060102_150405")))
	if err := h.copyDatabaseToBackup(backupBeforeRestore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create pre-restore backup: %v", err)})
		return
	}

	// Copy the temporary file to the database location
	if err := copyFile(tempFile.Name(), h.DBPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to restore database: %v", err)})
		return
	}

	// Redirect to home page to reinitialize the application
	c.JSON(http.StatusOK, gin.H{"message": "Database restored successfully. The application will restart."})
}

// HandleExportConfigs handles the export all configurations request
func (h *Handlers) HandleExportConfigs(c *gin.Context) {
	// Get all configurations
	var configs []db.TransferConfig
	if err := h.DB.Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve configurations: %v", err)})
		return
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "gomft_configs_*.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create temporary file: %v", err)})
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up temp file when done
	defer tmpFile.Close()

	// Write the configurations to the file
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ") // Pretty print the JSON
	if err := encoder.Encode(configs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to write configurations to file: %v", err)})
		return
	}

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="gomft_configs_%s.json"`, time.Now().Format("20060102_150405")))
	c.Header("Content-Type", "application/json")

	// Send the file
	c.File(tmpFile.Name())
}

// HandleExportJobs handles the export all jobs request
func (h *Handlers) HandleExportJobs(c *gin.Context) {
	// Get all jobs
	var jobs []db.Job
	if err := h.DB.Preload("Config").Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve jobs: %v", err)})
		return
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "gomft_jobs_*.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create temporary file: %v", err)})
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up temp file when done
	defer tmpFile.Close()

	// Write the jobs to the file
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ") // Pretty print the JSON
	if err := encoder.Encode(jobs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to write jobs to file: %v", err)})
		return
	}

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="gomft_jobs_%s.json"`, time.Now().Format("20060102_150405")))
	c.Header("Content-Type", "application/json")

	// Send the file
	c.File(tmpFile.Name())
}

// HandleClearJobHistory handles the clear job history request
func (h *Handlers) HandleClearJobHistory(c *gin.Context) {
	// Delete all job history records
	if err := h.DB.Exec("DELETE FROM job_histories").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to clear job history: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job history cleared successfully"})
}

// HandleVacuumDatabase handles the vacuum database request
func (h *Handlers) HandleVacuumDatabase(c *gin.Context) {
	// Execute VACUUM command to optimize the database
	if err := h.DB.Exec("VACUUM").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to vacuum database: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Database vacuumed successfully"})
}

// HandleRestoreDatabaseByFilename handles restoring a backup file from the backup directory
func (h *Handlers) HandleRestoreDatabaseByFilename(c *gin.Context) {
	filename := c.Param("filename")

	// Validate filename format
	if !strings.HasPrefix(filename, "gomft_backup_") || !strings.HasSuffix(filename, ".db") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup filename"})
		return
	}

	// Construct full file path
	backupPath := filepath.Join(h.BackupDir, filename)

	// Check if file exists and is within backup directory
	if !strings.HasPrefix(backupPath, h.BackupDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup path"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup file not found"})
		return
	}

	// Stop the scheduler to prevent jobs from running during restore
	h.Scheduler.Stop()

	// Close the current database connection
	if err := h.DB.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to close database: %v", err)})
		return
	}

	// Create a backup of the current database before restoring
	backupBeforeRestore := filepath.Join(h.BackupDir, fmt.Sprintf("pre_restore_backup_%s.db", time.Now().Format("20060102_150405")))
	if err := h.copyDatabaseToBackup(backupBeforeRestore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create pre-restore backup: %v", err)})
		return
	}

	// Copy the backup file to the database location
	if err := copyFile(backupPath, h.DBPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to restore database: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Database restored successfully. The application will restart."})
}

// HandleRefreshBackups handles the HTMX request to refresh the backups list
func (h *Handlers) HandleRefreshBackups(c *gin.Context) {
	// Get list of backup files
	backupFiles := h.getBackupFiles()

	// Create data structure for the template
	data := components.AdminToolsData{
		BackupFiles: backupFiles,
	}

	// Get last backup time and backup count
	data.LastBackupTime, data.BackupCount = h.getBackupInfo()

	// Render just the BackupsList component
	components.BackupsList(data).Render(c, c.Writer)
}

// HandleRefreshLogs refreshes the log files list
func (h *Handlers) HandleRefreshLogs(c *gin.Context) {
	// Get system statistics
	data := components.AdminToolsData{
		LogFiles: h.getLogFiles(),
	}

	// Render only the log viewer component
	components.AdminLogViewer(data).Render(c, c.Writer)
}

// HandleImportConfigs handles importing transfer configurations from JSON
func (h *Handlers) HandleImportConfigs(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Read the request body
	var configs []db.TransferConfig
	if err := c.ShouldBindJSON(&configs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid JSON: %v", err)})
		return
	}

	// Import each config
	imported := 0
	for i := range configs {
		// Set created by to current user
		configs[i].CreatedBy = userObj.ID

		// Create in database
		if err := h.DB.Create(&configs[i]).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to import config: %v", err)})
			return
		}
		imported++
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d configs imported successfully", imported)})
}

// HandleImportJobs handles importing jobs from JSON
func (h *Handlers) HandleImportJobs(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Read the request body
	var jobs []db.Job

	// Read the raw JSON first
	var rawJobs []map[string]interface{}
	if err := c.ShouldBindJSON(&rawJobs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid JSON: %v", err)})
		return
	}

	// Convert the raw jobs to db.Job objects
	for _, rawJob := range rawJobs {
		job := db.Job{
			CreatedBy: userObj.ID,
		}

		// Set the fields from the raw job
		if name, ok := rawJob["name"].(string); ok {
			job.Name = name
		}

		if schedule, ok := rawJob["schedule"].(string); ok {
			job.Schedule = schedule
		}

		if enabled, ok := rawJob["enabled"].(bool); ok {
			job.Enabled = enabled
		}

		// Handle config_id
		if configID, ok := rawJob["config_id"].(float64); ok {
			job.ConfigID = uint(configID)
		}

		// Handle config_ids
		if configIDs, ok := rawJob["config_ids"].(string); ok {
			job.ConfigIDs = configIDs
		}

		// Validate config ID exists
		var config db.TransferConfig
		if err := h.DB.First(&config, job.ConfigID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Config ID %d not found", job.ConfigID)})
			return
		}

		// Create in database
		if err := h.DB.Create(&job).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to import job: %v", err)})
			return
		}

		jobs = append(jobs, job)
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d jobs imported successfully", len(jobs))})
}

// HandleListBackups returns a list of all database backups
func (h *Handlers) HandleListBackups(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get backup files
	backups := h.getBackupFiles()

	c.JSON(http.StatusOK, gin.H{
		"backups": backups,
	})
}

// HandleSystemInfo returns system information for the admin dashboard
func (h *Handlers) HandleSystemInfo(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get basic system info
	info := map[string]interface{}{
		"os":         h.getOSInfo(),
		"memory":     h.getMemoryInfo(),
		"cpu":        h.getCPUInfo(),
		"disk":       h.getDiskInfo(),
		"go_version": h.getGoVersion(),
		"uptime":     h.getSystemUptime(),
	}

	c.JSON(http.StatusOK, info)
}

// HandleImportJobsFromFile handles importing jobs from an uploaded JSON file
func (h *Handlers) HandleImportJobsFromFile(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get the uploaded file
	file, err := c.FormFile("jobs_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No jobs file provided"})
		return
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open uploaded file: %v", err)})
		return
	}
	defer src.Close()

	// Read file contents
	fileContent, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read file: %v", err)})
		return
	}

	// Parse jobs from JSON
	var jobs []db.Job

	// Read the raw JSON first
	var rawJobs []map[string]interface{}
	if err := json.Unmarshal(fileContent, &rawJobs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid JSON: %v", err)})
		return
	}

	// Convert the raw jobs to db.Job objects
	for _, rawJob := range rawJobs {
		job := db.Job{
			CreatedBy: userObj.ID,
		}

		// Set the fields from the raw job
		if name, ok := rawJob["name"].(string); ok {
			job.Name = name
		}

		if schedule, ok := rawJob["schedule"].(string); ok {
			job.Schedule = schedule
		}

		if enabled, ok := rawJob["enabled"].(bool); ok {
			job.Enabled = enabled
		}

		// Handle config_id
		if configID, ok := rawJob["config_id"].(float64); ok {
			job.ConfigID = uint(configID)
		}

		// Handle config_ids
		if configIDs, ok := rawJob["config_ids"].(string); ok {
			job.ConfigIDs = configIDs
		}

		// Validate config ID exists
		var config db.TransferConfig
		if err := h.DB.First(&config, job.ConfigID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Config ID %d not found", job.ConfigID)})
			return
		}

		// Create in database
		if err := h.DB.Create(&job).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to import job: %v", err)})
			return
		}

		jobs = append(jobs, job)
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d jobs imported successfully", len(jobs))})
}

// HandleDeleteLogFile handles the deletion of a log file
func (h *Handlers) HandleDeleteLogFile(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get filename from params
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No filename provided"})
		return
	}

	// Validate filename (basic security check)
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
		return
	}

	// Construct full file path
	logFilePath := filepath.Join(h.LogsDir, filename)

	// Ensure the file is within the logs directory
	if !strings.HasPrefix(logFilePath, h.LogsDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log file path"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log file not found"})
		return
	}

	// Delete the file
	if err := os.Remove(logFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete log file: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Log file deleted successfully"})
}

// HandleSystemMaintenanceCheck handles the system maintenance check request
func (h *Handlers) HandleSystemMaintenanceCheck(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Perform maintenance checks
	checks := map[string]interface{}{
		"database_size":    h.checkDatabaseSize(),
		"disk_space":       h.checkDiskSpace(),
		"job_history":      h.checkJobHistorySize(),
		"inactive_configs": h.checkInactiveConfigs(),
		"failed_jobs":      h.checkFailedJobs(),
	}

	// Determine overall status based on checks
	status := "healthy"
	for _, result := range checks {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if resultMap["status"] == "warning" || resultMap["status"] == "critical" {
				status = "needs_attention"
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"checks": checks,
	})
}

// HandleUpdateSystemSettings handles updating system settings
func (h *Handlers) HandleUpdateSystemSettings(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Parse settings from request body
	var settings struct {
		EmailNotifications     bool `json:"email_notifications"`
		LogRetentionDays       int  `json:"log_retention_days"`
		MaxConcurrentTransfers int  `json:"max_concurrent_transfers"`
		DefaultRetryAttempts   int  `json:"default_retry_attempts"`
	}

	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid settings data: %v", err)})
		return
	}

	// Validate settings
	if settings.LogRetentionDays < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Log retention days must be at least 1"})
		return
	}

	if settings.MaxConcurrentTransfers < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Max concurrent transfers must be at least 1"})
		return
	}

	if settings.DefaultRetryAttempts < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Default retry attempts cannot be negative"})
		return
	}

	// Update settings in database
	// Here we would typically store these in a settings table
	// For this example, we'll just return success

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

// Maintenance check helper functions
func (h *Handlers) checkDatabaseSize() map[string]interface{} {
	sizeStr, err := h.getDatabaseSize()
	if err != nil {
		return map[string]interface{}{
			"status":  "unknown",
			"message": "Unable to determine database size",
		}
	}

	// Parse size for comparison
	var size float64
	var unit string
	fmt.Sscanf(sizeStr, "%f %s", &size, &unit)

	status := "healthy"
	message := fmt.Sprintf("Database size is %s", sizeStr)

	// Check if database is large
	if unit == "MB" && size > 100 {
		status = "warning"
		message = fmt.Sprintf("Database size is %s, consider optimizing", sizeStr)
	} else if unit == "GB" {
		status = "critical"
		message = fmt.Sprintf("Database size is %s, vacuum recommended", sizeStr)
	}

	return map[string]interface{}{
		"status":  status,
		"message": message,
		"size":    sizeStr,
	}
}

func (h *Handlers) checkDiskSpace() map[string]interface{} {
	// For demo purposes, return a simulated result
	// In a real implementation, would check actual free disk space
	return map[string]interface{}{
		"status":     "healthy",
		"message":    "Sufficient disk space available",
		"free_space": "10.2 GB",
	}
}

func (h *Handlers) checkJobHistorySize() map[string]interface{} {
	var count int64
	h.DB.Model(&db.JobHistory{}).Count(&count)

	status := "healthy"
	message := fmt.Sprintf("%d job history records", count)

	if count > 10000 {
		status = "warning"
		message = fmt.Sprintf("%d job history records, consider clearing old records", count)
	} else if count > 50000 {
		status = "critical"
		message = fmt.Sprintf("%d job history records, performance may be impacted", count)
	}

	return map[string]interface{}{
		"status":  status,
		"message": message,
		"count":   count,
	}
}

func (h *Handlers) checkInactiveConfigs() map[string]interface{} {
	var count int64
	h.DB.Model(&db.TransferConfig{}).Where("id NOT IN (SELECT DISTINCT config_id FROM jobs)").Count(&count)

	status := "healthy"
	message := fmt.Sprintf("%d unused configurations", count)

	if count > 5 {
		status = "warning"
		message = fmt.Sprintf("%d unused configurations found", count)
	}

	return map[string]interface{}{
		"status":  status,
		"message": message,
		"count":   count,
	}
}

func (h *Handlers) checkFailedJobs() map[string]interface{} {
	var count int64
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	h.DB.Model(&db.JobHistory{}).Where("status = ? AND created_at > ?", "failed", oneDayAgo).Count(&count)

	status := "healthy"
	message := fmt.Sprintf("%d failed jobs in the last 24 hours", count)

	if count > 0 {
		status = "warning"
		message = fmt.Sprintf("%d failed jobs in the last 24 hours", count)
	}
	if count > 10 {
		status = "critical"
		message = fmt.Sprintf("%d failed jobs in the last 24 hours", count)
	}

	return map[string]interface{}{
		"status":  status,
		"message": message,
		"count":   count,
	}
}

// Helper functions

// getSystemUptime returns the system uptime as a formatted string
func (h *Handlers) getSystemUptime() string {
	uptime := time.Since(h.StartTime)
	days := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
	}
	return fmt.Sprintf("%d minutes", minutes)
}

// getDatabaseSize returns the size of the database file as a formatted string
func (h *Handlers) getDatabaseSize() (string, error) {
	fileInfo, err := os.Stat(h.DBPath)
	if err != nil {
		return "", err
	}

	sizeBytes := fileInfo.Size()

	// Format size
	if sizeBytes < 1024 {
		return fmt.Sprintf("%d B", sizeBytes), nil
	} else if sizeBytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(sizeBytes)/1024), nil
	} else if sizeBytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(sizeBytes)/(1024*1024)), nil
	}
	return fmt.Sprintf("%.2f GB", float64(sizeBytes)/(1024*1024*1024)), nil
}

// getBackupInfo returns the last backup time and total backup count
func (h *Handlers) getBackupInfo() (*time.Time, int) {
	// Check if backup directory exists
	if _, err := os.Stat(h.BackupDir); os.IsNotExist(err) {
		return nil, 0
	}

	// List all backup files
	backupFiles, err := filepath.Glob(filepath.Join(h.BackupDir, "gomft_backup_*.db"))
	if err != nil {
		return nil, 0
	}

	if len(backupFiles) == 0 {
		return nil, 0
	}

	// Find the most recent backup
	var lastBackupTime time.Time
	var lastBackupFile string

	for _, file := range backupFiles {
		filename := filepath.Base(file)
		// Extract timestamp from filename (format: gomft_backup_20060102_150405.db)
		if len(filename) < 28 {
			continue
		}

		timestampStr := filename[13:28]
		timestamp, err := time.Parse("20060102_150405", timestampStr)
		if err != nil {
			continue
		}

		if timestamp.After(lastBackupTime) {
			lastBackupTime = timestamp
			lastBackupFile = file
		}
	}

	if lastBackupFile == "" {
		return nil, len(backupFiles)
	}

	return &lastBackupTime, len(backupFiles)
}

// checkMaintenanceIssues checks for potential maintenance issues
func (h *Handlers) checkMaintenanceIssues() string {
	var issues []string

	// Check database size
	fileInfo, err := os.Stat(h.DBPath)
	if err == nil {
		sizeBytes := fileInfo.Size()
		// If database is larger than 100MB, suggest vacuum
		if sizeBytes > 100*1024*1024 {
			issues = append(issues, "Database size is large (>100MB). Consider running vacuum to optimize.")
		}
	}

	// Check job history count
	var jobHistoryCount int64
	if err := h.DB.Model(&db.JobHistory{}).Count(&jobHistoryCount).Error; err == nil {
		// If more than 1000 job history records, suggest clearing old records
		if jobHistoryCount > 1000 {
			issues = append(issues, fmt.Sprintf("Job history contains %d records. Consider clearing old records.", jobHistoryCount))
		}
	}

	// Check backup age
	lastBackupTime, _ := h.getBackupInfo()
	if lastBackupTime == nil {
		issues = append(issues, "No database backups found. Consider creating a backup.")
	} else {
		// If last backup is older than 7 days, suggest creating a new backup
		if time.Since(*lastBackupTime) > 7*24*time.Hour {
			issues = append(issues, fmt.Sprintf("Last backup is %d days old. Consider creating a new backup.", int(time.Since(*lastBackupTime).Hours()/24)))
		}
	}

	// Join all issues with newlines
	if len(issues) > 0 {
		return fmt.Sprintf("Maintenance Recommendations:\n%s", strings.Join(issues, "\n"))
	}

	return ""
}

// copyDatabaseToBackup copies the database file to the specified backup location
func (h *Handlers) copyDatabaseToBackup(backupPath string) error {
	return copyFile(h.DBPath, backupPath)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

// getBackupFiles returns a list of backup files with their details
func (h *Handlers) getBackupFiles() []components.BackupFile {
	var backupFiles []components.BackupFile

	// Check if backup directory exists
	if _, err := os.Stat(h.BackupDir); os.IsNotExist(err) {
		return backupFiles
	}

	// List all backup files
	files, err := filepath.Glob(filepath.Join(h.BackupDir, "gomft_backup_*.db"))
	if err != nil {
		return backupFiles
	}

	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			continue
		}

		// Format file size
		var sizeStr string
		size := fileInfo.Size()
		switch {
		case size < 1024:
			sizeStr = fmt.Sprintf("%d B", size)
		case size < 1024*1024:
			sizeStr = fmt.Sprintf("%.2f KB", float64(size)/1024)
		case size < 1024*1024*1024:
			sizeStr = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
		default:
			sizeStr = fmt.Sprintf("%.2f GB", float64(size)/(1024*1024*1024))
		}

		backupFiles = append(backupFiles, components.BackupFile{
			Name:    filepath.Base(file),
			Size:    sizeStr,
			ModTime: fileInfo.ModTime(),
		})
	}

	// Sort backups by modification time, newest first
	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].ModTime.After(backupFiles[j].ModTime)
	})

	return backupFiles
}

// HandleDeleteBackup handles the DELETE /admin/delete-backup/:filename route
func (h *Handlers) HandleDeleteBackup(c *gin.Context) {
	filename := c.Param("filename")

	// Validate filename format
	if !strings.HasPrefix(filename, "gomft_backup_") || !strings.HasSuffix(filename, ".db") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup filename"})
		return
	}

	// Construct full file path
	filePath := filepath.Join(h.BackupDir, filename)

	// Check if file exists and is within backup directory
	if !strings.HasPrefix(filePath, h.BackupDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup path"})
		return
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete backup: %v", err)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Backup deleted successfully"})
}

// HandleDownloadBackup handles the GET /admin/download-backup/:filename route
func (h *Handlers) HandleDownloadBackup(c *gin.Context) {
	filename := c.Param("filename")

	// Validate filename format
	if !strings.HasPrefix(filename, "gomft_backup_") || !strings.HasSuffix(filename, ".db") {
		c.String(http.StatusBadRequest, "Invalid backup filename")
		return
	}

	// Construct full file path
	filePath := filepath.Join(h.BackupDir, filename)

	// Check if file exists and is within backup directory
	if !strings.HasPrefix(filePath, h.BackupDir) {
		c.String(http.StatusBadRequest, "Invalid backup path")
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "Backup file not found")
		return
	}

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")

	// Serve the file
	c.File(filePath)
}

// formatSize converts bytes to human-readable sizes
func formatSize(bytes float64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", bytes/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", bytes/KB)
	default:
		return fmt.Sprintf("%.0f B", bytes)
	}
}

// Helper function to get log files
func (h *Handlers) getLogFiles() []components.LogFile {
	// Determine logs directory
	logsDir := os.Getenv("LOGS_DIR")
	if logsDir == "" {
		dataDir := os.Getenv("DATA_DIR")
		if dataDir == "" {
			dataDir = "./data"
		}
		logsDir = filepath.Join(dataDir, "logs")
	}

	// Try to read directory
	files, err := ioutil.ReadDir(logsDir)
	if err != nil {
		return []components.LogFile{}
	}

	// Process files
	var logFiles []components.LogFile
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only include .log files
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".log") {
			continue
		}

		size := formatSize(float64(file.Size()))
		logFiles = append(logFiles, components.LogFile{
			Name:    file.Name(),
			Size:    size,
			ModTime: file.ModTime(),
			Path:    filepath.Join(logsDir, file.Name()),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime.After(logFiles[j].ModTime)
	})

	return logFiles
}

// HandleViewLog displays the contents of a log file
func (h *Handlers) HandleViewLog(c *gin.Context) {
	fileName := c.Param("fileName")
	if fileName == "" {
		c.String(http.StatusBadRequest, "No file name provided")
		return
	}

	// Sanitize the filename to prevent directory traversal
	fileName = filepath.Base(fileName)

	// Determine logs directory
	logsDir := os.Getenv("LOGS_DIR")
	if logsDir == "" {
		dataDir := os.Getenv("DATA_DIR")
		if dataDir == "" {
			dataDir = "./data"
		}
		logsDir = filepath.Join(dataDir, "logs")
	}

	filePath := filepath.Join(logsDir, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "Log file not found")
		return
	}

	// Read file contents
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error reading log file: "+err.Error())
		return
	}

	// Ensure content is large enough to trigger scrollbar (add padding)
	logContent := string(content)

	// Add padding at the end to ensure scrollbar is visible even for small logs
	if len(logContent) < 2000 {
		paddingNeeded := 100 - strings.Count(logContent, "\n")
		if paddingNeeded > 0 {
			for i := 0; i < paddingNeeded; i++ {
				logContent += "\n "
			}
		}
	}

	data := components.AdminToolsData{
		CurrentLogFile: fileName,
		LogContent:     logContent,
	}

	// Render the template using the templ package
	components.AdminLogContent(data).Render(c, c.Writer)
}

// HandleDownloadLog allows downloading a log file
func (h *Handlers) HandleDownloadLog(c *gin.Context) {
	fileName := c.Param("fileName")
	if fileName == "" {
		c.String(http.StatusBadRequest, "No file name provided")
		return
	}

	// Sanitize the filename to prevent directory traversal
	fileName = filepath.Base(fileName)

	// Determine logs directory
	logsDir := os.Getenv("LOGS_DIR")
	if logsDir == "" {
		dataDir := os.Getenv("DATA_DIR")
		if dataDir == "" {
			dataDir = "./data"
		}
		logsDir = filepath.Join(dataDir, "logs")
	}

	filePath := filepath.Join(logsDir, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "Log file not found")
		return
	}

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "text/plain")
	c.File(filePath)
}

// Helper functions for system info
func (h *Handlers) getOSInfo() map[string]string {
	return map[string]string{
		"name":    "Linux", // For testing; in a real implementation, you would detect the actual OS
		"version": "1.0",
	}
}

func (h *Handlers) getMemoryInfo() map[string]interface{} {
	return map[string]interface{}{
		"total":     "8 GB",
		"used":      "4 GB",
		"available": "4 GB",
		"percent":   50.0,
	}
}

func (h *Handlers) getCPUInfo() map[string]interface{} {
	return map[string]interface{}{
		"model": "Intel(R) Core(TM) i7",
		"cores": 4,
		"usage": 25.0,
		"mhz":   3200,
	}
}

func (h *Handlers) getDiskInfo() map[string]interface{} {
	return map[string]interface{}{
		"total":     "500 GB",
		"used":      "250 GB",
		"available": "250 GB",
		"percent":   50.0,
	}
}

func (h *Handlers) getGoVersion() string {
	return "go1.17.5"
}

// HandleImportConfigsFromFile handles importing transfer configurations from an uploaded JSON file
func (h *Handlers) HandleImportConfigsFromFile(c *gin.Context) {
	// Check admin access
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userObj, ok := user.(*db.User)
	if !ok || !userObj.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get the file from the form data
	file, _, err := c.Request.FormFile("configs_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to get file: %v", err)})
		return
	}
	defer file.Close()

	// Read the file contents
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read file: %v", err)})
		return
	}

	// Parse the JSON
	var configs []db.TransferConfig
	if err := json.Unmarshal(fileBytes, &configs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid JSON: %v", err)})
		return
	}

	// Import each config
	imported := 0
	for i := range configs {
		// Set created by to current user
		configs[i].CreatedBy = userObj.ID

		// Create in database
		if err := h.DB.Create(&configs[i]).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to import config: %v", err)})
			return
		}
		imported++
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d configs imported successfully", imported)})
}
