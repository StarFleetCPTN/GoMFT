package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	// Close the existing database connection to ensure a clean backup
	h.DB.Close()

	// Generate a backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02-150405")
	backupName := fmt.Sprintf("backup-%s.db", timestamp)
	backupPath := filepath.Join(h.BackupDir, backupName)

	// Copy the database file
	err := copyFile(h.DBPath, backupPath)
	if err != nil {
		// Reopen DB and preserve auth
		if !h.reopenDatabaseWithAuth(c, "after+backup+attempt") {
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+create+backup&details="+err.Error())
		return
	}

	// Reopen the database connection and preserve auth
	if !h.reopenDatabaseWithAuth(c, "after+backup") {
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

	// Close the database connection
	h.DB.Close()

	// Create a backup of the current database before restoring
	currentBackupName := fmt.Sprintf("pre-restore-%s.db", time.Now().Format("2006-01-02-150405"))
	currentBackupPath := filepath.Join(h.BackupDir, currentBackupName)
	if err := copyFile(h.DBPath, currentBackupPath); err != nil {
		// Reopen DB and preserve auth
		if !h.reopenDatabaseWithAuth(c, "after+pre-restore+backup+attempt") {
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+backup+current+database&details="+err.Error())
		return
	}

	// Restore the database by copying the backup file
	if err := copyFile(backupPath, h.DBPath); err != nil {
		// Reopen DB and preserve auth
		if !h.reopenDatabaseWithAuth(c, "after+restore+attempt") {
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+restore+database&details="+err.Error())
		return
	}

	// Reopen the database connection with the restored database and preserve auth
	if !h.reopenDatabaseWithAuth(c, "after+restore") {
		return
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

	// Close DB connection to make a clean backup
	h.DB.Close()

	if err := copyFile(h.DBPath, backupPath); err != nil {
		// Reopen DB and preserve auth
		if !h.reopenDatabaseWithAuth(c, "after+backup+attempt") {
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+backup+before+vacuum&details="+err.Error())
		return
	}

	// Reopen DB and preserve auth
	if !h.reopenDatabaseWithAuth(c, "after+backup") {
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

// reopenDatabaseWithAuth safely closes and reopens the database while preserving user authentication
func (h *Handlers) reopenDatabaseWithAuth(c *gin.Context, reason string) (success bool) {
	// Store auth info from context
	userID, userExists := c.Get("userID")
	email, emailExists := c.Get("email")
	username, usernameExists := c.Get("username")
	isAdmin, adminExists := c.Get("isAdmin")

	// First close the existing connection if it exists
	if h.DB != nil {
		h.DB.Close()
		// Set to nil to avoid using a closed connection
		h.DB = nil
	}

	// Make sure we wait a moment to ensure the file is released
	time.Sleep(100 * time.Millisecond)

	// Reopen database WITHOUT running migrations
	newDB, err := db.ReopenWithoutMigrations(h.DBPath)
	if err != nil {
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/database?error=Failed+to+reconnect+to+database+%s&details=%s", reason, err.Error()))
		return false
	}

	// Verify the connection works by executing a simple query
	var count int64
	if err := newDB.Raw("SELECT 1").Scan(&count).Error; err != nil {
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/database?error=Database+connection+test+failed+%s&details=%s", reason, err.Error()))
		return false
	}

	// Only after validation, assign the new database connection
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

	// If we had a user, attempt to reload their info
	if userExists && h.DB != nil {
		var user db.User
		userIDValue, ok := userID.(uint)
		if !ok {
			// Try to convert from float64 (the JWT parser returns numbers as float64)
			if userIDFloat, ok := userID.(float64); ok {
				userIDValue = uint(userIDFloat)
			}
		}

		if userIDValue > 0 {
			result := h.DB.Preload("Roles").First(&user, userIDValue)
			if result.Error == nil {
				c.Set("user", &user)
			} else {
				// Log the error but continue - we still have basic user info from JWT
				fmt.Printf("Error loading user roles after DB reconnect: %v\n", result.Error)
			}
		}
	}

	return true
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

	// Close DB connection to make a clean backup
	h.DB.Close()

	if err := copyFile(h.DBPath, backupPath); err != nil {
		// Reopen DB and preserve auth
		if !h.reopenDatabaseWithAuth(c, "after+backup+attempt") {
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/database?error=Failed+to+backup+before+clearing+history&details="+err.Error())
		return
	}

	// Reopen DB and preserve auth
	if !h.reopenDatabaseWithAuth(c, "after+backup") {
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
