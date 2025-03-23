package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
	"gorm.io/gorm"
)

// HandleDatabaseTools renders the database tools page
func (h *Handlers) HandleDatabaseTools(c *gin.Context) {
	backups, err := h.GetBackupFiles()
	if err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Database Error", "Failed to list backup files", err)
		return
	}

	ctx := components.CreateTemplateContext(c)
	_ = components.AdminDatabaseTools(ctx, backups).Render(ctx, c.Writer)
}

// GetBackupFiles retrieves a list of database backup files
func (h *Handlers) GetBackupFiles() ([]components.BackupFile, error) {
	var backups []components.BackupFile

	// Read the backup directory
	files, err := os.ReadDir(h.BackupDir)
	if err != nil {
		return nil, err
	}

	// Filter and sort backup files (newest first)
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".db") {
			continue
		}

		fileInfo, err := file.Info()
		if err != nil {
			continue
		}

		// Get file size in human-readable format
		size := formatFileSize(fileInfo.Size())

		backups = append(backups, components.BackupFile{
			Name:     file.Name(),
			Size:     size,
			Created:  fileInfo.ModTime(),
			FullPath: filepath.Join(h.BackupDir, file.Name()),
		})
	}

	// Sort backups by creation time (most recent first)
	// Simple bubble sort for now, can be optimized for larger lists
	for i := 0; i < len(backups); i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].Created.Before(backups[j].Created) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	return backups, nil
}

// HandleBackupDatabase creates a backup of the current database
func (h *Handlers) HandleBackupDatabase(c *gin.Context) {
	// Instead of closing the database connection, get a new connection to the database
	// This will use the underlying connection pool - just checking connection availability
	_, err := h.DB.DB.DB()
	if err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Database Error", "Failed to access database", err)
		return
	}

	// Generate a backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02-150405")
	backupName := fmt.Sprintf("backup-%s.db", timestamp)
	backupPath := filepath.Join(h.BackupDir, backupName)

	// Copy the database file without closing the main connection
	if err := backupDatabaseFile(h.DBPath, backupPath); err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+create+backup&details="+err.Error())
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/database?status=Backup+created+successfully")
}

// HandleRestoreDatabase restores the database from a backup file
func (h *Handlers) HandleRestoreDatabase(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		// Handle file upload restore
		file, err := c.FormFile("backup_file")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/admin/database?error=Invalid+backup+file")
			return
		}

		// Save the uploaded file
		tempPath := filepath.Join(h.BackupDir, "temp-"+file.Filename)
		if err := c.SaveUploadedFile(file, tempPath); err != nil {
			c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+save+uploaded+file&details="+err.Error())
			return
		}

		// Use the uploaded file for restoration
		filename = "temp-" + file.Filename
	}

	// Validate that the backup file exists
	backupPath := filepath.Join(h.BackupDir, filename)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Backup+file+not+found")
		return
	}

	// Create a backup of the current database before restoring
	currentBackupName := fmt.Sprintf("pre-restore-%s.db", time.Now().Format("2006-01-02-150405"))
	currentBackupPath := filepath.Join(h.BackupDir, currentBackupName)

	if err := backupDatabaseFile(h.DBPath, currentBackupPath); err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+backup+current+database&details="+err.Error())
		return
	}

	// Store auth info from context
	userID, userExists := c.Get("userID")
	email, emailExists := c.Get("email")
	username, usernameExists := c.Get("username")
	isAdmin, adminExists := c.Get("isAdmin")

	// Close current connections before restore
	sqlDB, err := h.DB.DB.DB()
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+access+database&details="+err.Error())
		return
	}

	// Close the current connection pool
	if err := sqlDB.Close(); err != nil {
		fmt.Printf("Warning: Error closing database connection: %v\n", err)
	}

	// Wait for connections to fully close
	time.Sleep(1 * time.Second)

	// Restore the database by copying the backup file
	if err := copyFile(backupPath, h.DBPath); err != nil {
		// Need to reopen the database
		newDB, reopenErr := db.ReopenWithoutMigrations(h.DBPath)
		if reopenErr != nil {
			c.Redirect(http.StatusSeeOther, "/admin/database?error=Critical+error:+Database+restore+failed+and+reconnection+failed&details="+reopenErr.Error())
			return
		}
		h.DB = newDB

		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+restore+database&details="+err.Error())
		return
	}

	// Reopen the database connection with the restored database
	newDB, err := db.ReopenWithoutMigrations(h.DBPath)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+reconnect+to+database+after+restore&details="+err.Error())
		return
	}

	// Update the handler's database connection
	h.DB = newDB

	// Restore auth context
	if userExists {
		c.Set("userID", userID)
	}
	if emailExists {
		c.Set("email", email)
	}
	if usernameExists {
		c.Set("username", username)
	}
	if adminExists {
		c.Set("isAdmin", isAdmin)
	}

	// Clean up temp file if it was an upload
	if strings.HasPrefix(filename, "temp-") {
		os.Remove(backupPath)
	}

	c.Redirect(http.StatusSeeOther, "/admin/database?status=Database+restored+successfully")
}

// HandleDownloadBackup allows downloading a backup file
func (h *Handlers) HandleDownloadBackup(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		h.HandleBadRequest(c, "Missing filename", "No backup file specified for download")
		return
	}

	// Validate the filename to prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		h.HandleBadRequest(c, "Invalid filename", "The filename contains invalid characters")
		return
	}

	// Set file path
	filePath := filepath.Join(h.BackupDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		h.HandleNotFound(c, "Backup file not found", "The requested backup file does not exist")
		return
	}

	// Serve the file for download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
}

// HandleDeleteBackup deletes a backup file
func (h *Handlers) HandleDeleteBackup(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Invalid+backup+file")
		return
	}

	// Validate the filename to prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Invalid+backup+filename")
		return
	}

	// Set file path
	filePath := filepath.Join(h.BackupDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Backup+file+not+found")
		return
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+delete+backup&details="+err.Error())
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/database?status=Backup+deleted+successfully")
}

// HandleRefreshBackups refreshes the backup list
func (h *Handlers) HandleRefreshBackups(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "/admin/database")
}

// HandleVacuumDatabase optimizes the database
func (h *Handlers) HandleVacuumDatabase(c *gin.Context) {
	// Check if DB is nil
	if h.DB == nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Database+connection+is+not+available")
		return
	}

	// Check if user is authenticated before proceeding
	_, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Create a backup before vacuum
	timestamp := time.Now().Format("2006-01-02-150405")
	backupName := fmt.Sprintf("pre-vacuum-%s.db", timestamp)
	backupPath := filepath.Join(h.BackupDir, backupName)

	// Make a backup without closing the DB
	if err := backupDatabaseFile(h.DBPath, backupPath); err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+backup+before+vacuum&details="+err.Error())
		return
	}

	// Run vacuum command
	result := h.DB.Exec("VACUUM")
	if result.Error != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Database+optimization+failed&details="+result.Error.Error())
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/database?status=Database+optimized+successfully")
}

// HandleClearJobHistory clears job history records
func (h *Handlers) HandleClearJobHistory(c *gin.Context) {
	// Check if DB is nil
	if h.DB == nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Database+connection+is+not+available")
		return
	}

	// Check if user is authenticated before proceeding
	_, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Backup the database before making significant changes
	timestamp := time.Now().Format("2006-01-02-150405")
	backupName := fmt.Sprintf("pre-clear-history-%s.db", timestamp)
	backupPath := filepath.Join(h.BackupDir, backupName)

	// Make a backup without closing the DB
	if err := backupDatabaseFile(h.DBPath, backupPath); err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+backup+before+clearing+history&details="+err.Error())
		return
	}

	// Make sure we have a valid connection before executing the DELETE
	var testCount int64
	if err := h.DB.Raw("SELECT COUNT(*) FROM job_histories").Scan(&testCount).Error; err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+verify+database+connection&details="+err.Error())
		return
	}

	// Build a more robust query that handles the case where the column might not exist
	// First check if created_at column exists
	var createdAtExists int
	columnCheckQuery := `SELECT COUNT(*) FROM pragma_table_info('job_histories') WHERE name = 'created_at'`
	h.DB.Raw(columnCheckQuery).Scan(&createdAtExists)

	var result *gorm.DB

	if createdAtExists > 0 {
		// If created_at exists, use both columns
		result = h.DB.Exec("DELETE FROM job_histories WHERE start_time < datetime('now', '-30 day') OR created_at < datetime('now', '-30 day')")
	} else {
		// Otherwise just use start_time
		result = h.DB.Exec("DELETE FROM job_histories WHERE start_time < datetime('now', '-30 day')")
	}

	if result.Error != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+clear+job+history&details="+result.Error.Error())
		return
	}

	// Log how many records were deleted
	recordsDeleted := result.RowsAffected
	logMessage := fmt.Sprintf("Deleted %d job history records", recordsDeleted)
	fmt.Println(logMessage)

	// Redirect to the database page with success message
	c.Redirect(http.StatusSeeOther, "/admin/database?status=Job+history+cleared+successfully+"+logMessage)
}

// backupDatabaseFile creates a copy of the database file without closing the connection
func backupDatabaseFile(srcPath, destPath string) error {
	// Copy the database using the WAL mode safe approach
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %v", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy database: %v", err)
	}

	// Also copy the WAL files if they exist
	walPath := srcPath + "-wal"
	if _, err := os.Stat(walPath); err == nil {
		if err := copyFile(walPath, destPath+"-wal"); err != nil {
			fmt.Printf("Warning: failed to copy WAL file: %v\n", err)
		}
	}

	shmPath := srcPath + "-shm"
	if _, err := os.Stat(shmPath); err == nil {
		if err := copyFile(shmPath, destPath+"-shm"); err != nil {
			fmt.Printf("Warning: failed to copy SHM file: %v\n", err)
		}
	}

	return destFile.Sync()
}

// reopenDatabaseWithAuth safely closes and reopens the database while preserving user authentication
// This is now only used as a fallback when necessary, not as the primary approach
func (h *Handlers) reopenDatabaseWithAuth(c *gin.Context, reason string) (success bool) {
	// Store auth info from context
	userID, userExists := c.Get("userID")
	email, emailExists := c.Get("email")
	username, usernameExists := c.Get("username")
	isAdmin, adminExists := c.Get("isAdmin")

	// First close the existing connection if it exists
	if h.DB != nil {
		// Close the database connection properly
		if err := h.DB.Close(); err != nil {
			fmt.Printf("Warning: Error closing database connection: %v\n", err)
		}

		// Set to nil to avoid using a closed connection
		h.DB = nil
	}

	// Make sure we wait longer to ensure the file is completely released
	// Different OSes might need different times for file handles to be released
	time.Sleep(1000 * time.Millisecond)

	// Define max retry attempts and backoff times
	maxRetries := 5
	var err error

	// Attempt to reopen the database with retries
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Reopen database WITHOUT running migrations
		h.DB, err = db.ReopenWithoutMigrations(h.DBPath)
		if err == nil {
			// Verify the connection works by executing a simple query
			var count int64
			if verifyErr := h.DB.Raw("SELECT 1").Scan(&count).Error; verifyErr == nil {
				break // Successfully reopened and verified
			} else {
				// Close this failed connection
				h.DB.Close()
				h.DB = nil
				err = verifyErr
				fmt.Printf("Database verification failed on attempt %d: %v\n", attempt+1, verifyErr)
			}
		} else {
			fmt.Printf("Database reopen failed on attempt %d: %v\n", attempt+1, err)
		}

		// Wait before retrying, with increasing backoff
		waitTime := time.Duration(1000*(attempt+1)) * time.Millisecond
		fmt.Printf("Waiting %v before retry #%d\n", waitTime, attempt+1)
		time.Sleep(waitTime)
	}

	// If all retries failed, redirect with error
	if err != nil {
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/database?error=Failed+to+reconnect+to+database+%s&details=%s", reason, err.Error()))
		return false
	}

	// Restore auth context
	if userExists {
		c.Set("userID", userID)
	}
	if emailExists {
		c.Set("email", email)
	}
	if usernameExists {
		c.Set("username", username)
	}
	if adminExists {
		c.Set("isAdmin", isAdmin)
	}

	// If we had a user, attempt to reload their info
	if userExists && h.DB != nil {
		var user db.User
		userIDValue, ok := userID.(uint)
		if !ok {
			// Try to convert from float64 (the JWT parser returns numbers as float64)
			if userIDFloat, ok := userID.(float64); ok {
				userIDValue = uint(userIDFloat)
			} else {
				// Try string conversion as a last resort
				if userIDStr, ok := userID.(string); ok {
					if parsed, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
						userIDValue = uint(parsed)
					}
				}
			}
		}

		if userIDValue > 0 {
			// Try to load the user with their roles
			result := h.DB.Preload("Roles").First(&user, userIDValue)
			if result.Error == nil {
				c.Set("user", &user)
			} else {
				// Log the error but continue - we still have basic user info from JWT
				fmt.Printf("Error loading user roles after DB reconnect: %v\n", result.Error)

				// Try a simpler query as fallback
				simpleResult := h.DB.First(&user, userIDValue)
				if simpleResult.Error == nil {
					c.Set("user", &user)
					fmt.Println("Loaded user without roles after DB reconnect")
				}
			}
		}
	}

	fmt.Printf("Database successfully reopened after %s\n", reason)
	return true
}

// Helper function to copy a file
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

// Helper function to format file size in human-readable format
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// HandleExportConfigs exports system configurations as JSON
func (h *Handlers) HandleExportConfigs(c *gin.Context) {
	// Check if DB is nil
	if h.DB == nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Database+connection+is+not+available")
		return
	}

	// Check if user is authenticated and is admin
	_, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	isAdmin, adminExists := c.Get("isAdmin")
	if !adminExists || isAdmin != true {
		h.HandleError(c, http.StatusForbidden, "Permission Denied", "You do not have permission to export configurations", nil)
		return
	}

	// Query all system configurations
	var configs []db.TransferConfig
	if err := h.DB.Find(&configs).Error; err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+export+configurations&details="+err.Error())
		return
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("gomft-configs-%s.json", timestamp)

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")

	// Write the JSON to response
	c.JSON(http.StatusOK, configs)
}

// HandleExportJobs exports jobs as JSON
func (h *Handlers) HandleExportJobs(c *gin.Context) {
	// Check if DB is nil
	if h.DB == nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Database+connection+is+not+available")
		return
	}

	// Check if user is authenticated and is admin
	_, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	isAdmin, adminExists := c.Get("isAdmin")
	if !adminExists || isAdmin != true {
		h.HandleError(c, http.StatusForbidden, "Permission Denied", "You do not have permission to export jobs", nil)
		return
	}

	// Query all jobs with all possible relationships
	var jobs []db.Job
	query := h.DB.Model(&db.Job{})

	// Check if each relation exists before trying to preload
	// This makes the export more robust against schema changes

	// Try to preload steps if they exist
	if h.DB.Migrator().HasTable("job_steps") {
		query = query.Preload("Steps")
	}

	// Try to preload schedules if they exist
	if h.DB.Migrator().HasTable("job_schedules") {
		query = query.Preload("Schedule")
	}

	// Get the jobs with appropriate preloads
	if err := query.Find(&jobs).Error; err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+export+jobs&details="+err.Error())
		return
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("gomft-jobs-%s.json", timestamp)

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")

	// Write the JSON to response
	c.JSON(http.StatusOK, jobs)
}
