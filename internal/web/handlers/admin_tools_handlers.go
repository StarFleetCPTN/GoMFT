package handlers

import (
	"encoding/json"
	"fmt"
	"io"
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
