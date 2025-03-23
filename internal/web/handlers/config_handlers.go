package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleConfigs handles the GET /configs route
func (h *Handlers) HandleConfigs(c *gin.Context) {
	userID := c.GetUint("userID")

	var configs []db.TransferConfig
	h.DB.Where("created_by = ?", userID).Find(&configs)

	// Check for error or status parameters in the URL
	error := c.Query("error")
	errorDetails := c.Query("details")
	status := c.Query("status")

	data := components.ConfigsData{
		Configs:      configs,
		Error:        error,
		ErrorDetails: errorDetails,
		Status:       status,
	}
	components.Configs(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleNewConfig handles the GET /configs/new route
func (h *Handlers) HandleNewConfig(c *gin.Context) {
	data := components.ConfigFormData{
		Config: &db.TransferConfig{},
		IsNew:  true,
	}
	components.ConfigForm(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleEditConfig handles the GET /configs/:id/edit route
func (h *Handlers) HandleEditConfig(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")

	var config db.TransferConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/configs")
		return
	}

	// Check if user owns this config
	if config.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.Redirect(http.StatusFound, "/configs")
			return
		}
	}

	data := components.ConfigFormData{
		Config: &config,
		IsNew:  false,
	}
	components.ConfigForm(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleCreateConfig handles the POST /configs route
func (h *Handlers) HandleCreateConfig(c *gin.Context) {
	var config db.TransferConfig

	if err := c.ShouldBind(&config); err != nil {
		log.Printf("Error binding config form: %v", err)
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid form data: %v", err))
		return
	}

	userID := c.GetUint("userID")
	config.CreatedBy = userID

	// Process Boolean fields
	skipProcessedVal := c.Request.FormValue("skip_processed_files")
	skipProcessedValue := skipProcessedVal == "on" || skipProcessedVal == "true"
	config.SkipProcessedFiles = &skipProcessedValue

	archiveEnabledVal := c.Request.FormValue("archive_enabled")
	archiveEnabledValue := archiveEnabledVal == "on" || archiveEnabledVal == "true"
	config.ArchiveEnabled = &archiveEnabledValue

	deleteAfterTransferVal := c.Request.FormValue("delete_after_transfer")
	deleteAfterTransferValue := deleteAfterTransferVal == "on" || deleteAfterTransferVal == "true"
	config.DeleteAfterTransfer = &deleteAfterTransferValue

	sourcePassiveModeVal := c.Request.FormValue("source_passive_mode")
	sourcePassiveModeValue := sourcePassiveModeVal == "on" || sourcePassiveModeVal == "true"
	config.SourcePassiveMode = &sourcePassiveModeValue

	destPassiveModeVal := c.Request.FormValue("dest_passive_mode")
	destPassiveModeValue := destPassiveModeVal == "on" || destPassiveModeVal == "true"
	config.DestPassiveMode = &destPassiveModeValue

	// Google Photos specific fields
	destReadOnlyVal := c.Request.FormValue("dest_read_only")
	destReadOnlyValue := destReadOnlyVal == "on" || destReadOnlyVal == "true"
	config.DestReadOnly = &destReadOnlyValue

	sourceReadOnlyVal := c.Request.FormValue("source_read_only")
	sourceReadOnlyValue := sourceReadOnlyVal == "on" || sourceReadOnlyVal == "true"
	config.SourceReadOnly = &sourceReadOnlyValue

	destIncludeArchivedVal := c.Request.FormValue("dest_include_archived")
	destIncludeArchivedValue := destIncludeArchivedVal == "on" || destIncludeArchivedVal == "true"
	config.DestIncludeArchived = &destIncludeArchivedValue

	sourceIncludeArchivedVal := c.Request.FormValue("source_include_archived")
	sourceIncludeArchivedValue := sourceIncludeArchivedVal == "on" || sourceIncludeArchivedVal == "true"
	config.SourceIncludeArchived = &sourceIncludeArchivedValue

	// Get command_id and validate it
	commandIDStr := c.Request.FormValue("command_id")
	if commandIDStr != "" {
		commandID, err := strconv.ParseUint(commandIDStr, 10, 64)
		if err != nil {
			log.Printf("Error parsing command ID: %v", err)
		} else {
			config.CommandID = uint(commandID)
		}
	} else {
		// Default to copy command (ID 1)
		config.CommandID = 1
	}

	// Get command_flags and store as JSON
	commandFlags := c.PostFormArray("command_flags")
	if len(commandFlags) > 0 {
		flagIDs := make([]uint, 0, len(commandFlags))
		for _, flagStr := range commandFlags {
			flagID, err := strconv.ParseUint(flagStr, 10, 64)
			if err != nil {
				log.Printf("Error parsing flag ID: %v", err)
				continue
			}
			flagIDs = append(flagIDs, uint(flagID))
		}
		flagsJSON, err := json.Marshal(flagIDs)
		if err != nil {
			log.Printf("Error marshaling flag IDs: %v", err)
		} else {
			config.CommandFlags = string(flagsJSON)
		}
	}

	// Process builtin auth settings
	useBuiltinAuthSourceVal := c.Request.FormValue("use_builtin_auth_source")
	useBuiltinAuthSourceValue := useBuiltinAuthSourceVal == "on" || useBuiltinAuthSourceVal == "true"
	config.UseBuiltinAuthSource = &useBuiltinAuthSourceValue

	useBuiltinAuthDestVal := c.Request.FormValue("use_builtin_auth_dest")
	useBuiltinAuthDestValue := useBuiltinAuthDestVal == "on" || useBuiltinAuthDestVal == "true"
	config.UseBuiltinAuthDest = &useBuiltinAuthDestValue

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		log.Printf("Error beginning transaction: %v", tx.Error)
		c.String(http.StatusInternalServerError, "Failed to begin transaction")
		return
	}

	if err := tx.Create(&config).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating config: %v", err)
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create config: %v", err))
		return
	}

	// Create audit log entry
	auditDetails := map[string]interface{}{
		"name":                  config.Name,
		"source_type":           config.SourceType,
		"dest_type":             config.DestinationType,
		"source_path":           config.SourcePath,
		"dest_path":             config.DestinationPath,
		"skip_processed_files":  *config.SkipProcessedFiles,
		"archive_enabled":       *config.ArchiveEnabled,
		"delete_after_transfer": *config.DeleteAfterTransfer,
		"source_passive_mode":   *config.SourcePassiveMode,
		"dest_passive_mode":     *config.DestPassiveMode,
	}

	auditLog := db.AuditLog{
		Action:     "create",
		EntityType: "config",
		EntityID:   config.ID,
		UserID:     userID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating audit log: %v", err)
		c.String(http.StatusInternalServerError, "Failed to create audit log")
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.String(http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	// Generate rclone config file
	if err := h.DB.GenerateRcloneConfig(&config); err != nil {
		log.Printf("Warning: Failed to generate rclone config: %v", err)
		// Continue anyway, as the config was created in the database
	} else {
		log.Printf("Generated rclone config for config ID %d", config.ID)
	}

	c.Redirect(http.StatusFound, "/configs")
}

// HandleUpdateConfig handles the POST /configs/:id route
func (h *Handlers) HandleUpdateConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid ID: %v", err))
		return
	}

	existingConfig, err := h.DB.GetTransferConfig(uint(id))
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error getting config: %v", err))
		return
	}

	if existingConfig == nil {
		c.String(http.StatusNotFound, "Configuration not found")
		return
	}

	// Check if the user has permission to edit this config
	userID := c.GetUint("userID")
	if existingConfig.CreatedBy != userID {
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.String(http.StatusForbidden, "You don't have permission to edit this configuration")
			return
		}
	}

	var config db.TransferConfig

	if err := c.ShouldBind(&config); err != nil {
		log.Printf("Error binding config form: %v", err)
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid form data: %v", err))
		return
	}

	// Preserve the original creator ID and creation time
	config.ID = existingConfig.ID
	config.CreatedBy = existingConfig.CreatedBy
	config.CreatedAt = existingConfig.CreatedAt

	// Process Boolean fields
	skipProcessedVal := c.Request.FormValue("skip_processed_files")
	skipProcessedValue := skipProcessedVal == "on" || skipProcessedVal == "true"
	config.SkipProcessedFiles = &skipProcessedValue

	archiveEnabledVal := c.Request.FormValue("archive_enabled")
	archiveEnabledValue := archiveEnabledVal == "on" || archiveEnabledVal == "true"
	config.ArchiveEnabled = &archiveEnabledValue

	deleteAfterTransferVal := c.Request.FormValue("delete_after_transfer")
	deleteAfterTransferValue := deleteAfterTransferVal == "on" || deleteAfterTransferVal == "true"
	config.DeleteAfterTransfer = &deleteAfterTransferValue

	sourcePassiveModeVal := c.Request.FormValue("source_passive_mode")
	sourcePassiveModeValue := sourcePassiveModeVal == "on" || sourcePassiveModeVal == "true"
	config.SourcePassiveMode = &sourcePassiveModeValue

	destPassiveModeVal := c.Request.FormValue("dest_passive_mode")
	destPassiveModeValue := destPassiveModeVal == "on" || destPassiveModeVal == "true"
	config.DestPassiveMode = &destPassiveModeValue

	// Google Photos specific fields
	destReadOnlyVal := c.Request.FormValue("dest_read_only")
	destReadOnlyValue := destReadOnlyVal == "on" || destReadOnlyVal == "true"
	config.DestReadOnly = &destReadOnlyValue

	sourceReadOnlyVal := c.Request.FormValue("source_read_only")
	sourceReadOnlyValue := sourceReadOnlyVal == "on" || sourceReadOnlyVal == "true"
	config.SourceReadOnly = &sourceReadOnlyValue

	destIncludeArchivedVal := c.Request.FormValue("dest_include_archived")
	destIncludeArchivedValue := destIncludeArchivedVal == "on" || destIncludeArchivedVal == "true"
	config.DestIncludeArchived = &destIncludeArchivedValue

	sourceIncludeArchivedVal := c.Request.FormValue("source_include_archived")
	sourceIncludeArchivedValue := sourceIncludeArchivedVal == "on" || sourceIncludeArchivedVal == "true"
	config.SourceIncludeArchived = &sourceIncludeArchivedValue

	// Get command_id and validate it
	commandIDStr := c.Request.FormValue("command_id")
	if commandIDStr != "" {
		commandID, err := strconv.ParseUint(commandIDStr, 10, 64)
		if err != nil {
			log.Printf("Error parsing command ID: %v", err)
		} else {
			config.CommandID = uint(commandID)
		}
	} else {
		// Default to copy command (ID 1)
		config.CommandID = 1
	}

	// Get command_flags and store as JSON
	commandFlags := c.PostFormArray("command_flags")
	if len(commandFlags) > 0 {
		flagIDs := make([]uint, 0, len(commandFlags))
		for _, flagStr := range commandFlags {
			flagID, err := strconv.ParseUint(flagStr, 10, 64)
			if err != nil {
				log.Printf("Error parsing flag ID: %v", err)
				continue
			}
			flagIDs = append(flagIDs, uint(flagID))
		}
		flagsJSON, err := json.Marshal(flagIDs)
		if err != nil {
			log.Printf("Error marshaling flag IDs: %v", err)
		} else {
			config.CommandFlags = string(flagsJSON)
		}
	}

	// Process builtin auth settings
	useBuiltinAuthSourceVal := c.Request.FormValue("use_builtin_auth_source")
	useBuiltinAuthSourceValue := useBuiltinAuthSourceVal == "on" || useBuiltinAuthSourceVal == "true"
	config.UseBuiltinAuthSource = &useBuiltinAuthSourceValue

	useBuiltinAuthDestVal := c.Request.FormValue("use_builtin_auth_dest")
	useBuiltinAuthDestValue := useBuiltinAuthDestVal == "on" || useBuiltinAuthDestVal == "true"
	config.UseBuiltinAuthDest = &useBuiltinAuthDestValue

	// Preserve the Google Drive authentication status if it's already authenticated
	config.GoogleDriveAuthenticated = existingConfig.GoogleDriveAuthenticated

	// Update the LastUpdated timestamp
	config.UpdatedAt = time.Now()

	if err := h.DB.UpdateTransferConfig(&config); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating configuration: %v", err))
		return
	}

	// Redirect to the configs page
	c.Redirect(http.StatusSeeOther, "/configs")
}

// HandleDeleteConfig handles the DELETE /configs/:id route
func (h *Handlers) HandleDeleteConfig(c *gin.Context) {
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

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Create audit log before deletion
	auditDetails := map[string]interface{}{
		"name":                  config.Name,
		"source_type":           config.SourceType,
		"dest_type":             config.DestinationType,
		"source_path":           config.SourcePath,
		"dest_path":             config.DestinationPath,
		"skip_processed_files":  *config.SkipProcessedFiles,
		"archive_enabled":       *config.ArchiveEnabled,
		"delete_after_transfer": *config.DeleteAfterTransfer,
		"source_passive_mode":   *config.SourcePassiveMode,
		"dest_passive_mode":     *config.DestPassiveMode,
	}

	auditLog := db.AuditLog{
		Action:     "delete",
		EntityType: "config",
		EntityID:   config.ID,
		UserID:     userID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
		return
	}

	// Delete config
	if err := tx.Delete(&config).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete config: %v", err)})
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Config deleted successfully"})
}

// HandleDuplicateConfig handles the POST /configs/:id/duplicate route
func (h *Handlers) HandleDuplicateConfig(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")

	var originalConfig db.TransferConfig
	if err := h.DB.First(&originalConfig, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}

	// Check if user owns this config
	if originalConfig.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to duplicate this config"})
			return
		}
	}

	// Create a duplicate config
	duplicateConfig := originalConfig
	duplicateConfig.ID = 0 // Set ID to 0 to create a new record
	duplicateConfig.Name = originalConfig.Name + " - Copy"
	duplicateConfig.CreatedAt = time.Now()
	duplicateConfig.UpdatedAt = time.Now()
	duplicateConfig.CreatedBy = userID

	// Deep copy all boolean pointers
	skipProcessedVal := *originalConfig.SkipProcessedFiles
	duplicateConfig.SkipProcessedFiles = &skipProcessedVal

	archiveEnabledVal := *originalConfig.ArchiveEnabled
	duplicateConfig.ArchiveEnabled = &archiveEnabledVal

	deleteAfterTransferVal := *originalConfig.DeleteAfterTransfer
	duplicateConfig.DeleteAfterTransfer = &deleteAfterTransferVal

	sourcePassiveModeVal := *originalConfig.SourcePassiveMode
	duplicateConfig.SourcePassiveMode = &sourcePassiveModeVal

	destPassiveModeVal := *originalConfig.DestPassiveMode
	duplicateConfig.DestPassiveMode = &destPassiveModeVal

	// Google Photos specific fields
	if originalConfig.DestReadOnly != nil {
		destReadOnlyVal := *originalConfig.DestReadOnly
		duplicateConfig.DestReadOnly = &destReadOnlyVal
	}

	if originalConfig.SourceReadOnly != nil {
		sourceReadOnlyVal := *originalConfig.SourceReadOnly
		duplicateConfig.SourceReadOnly = &sourceReadOnlyVal
	}

	if originalConfig.DestIncludeArchived != nil {
		destIncludeArchivedVal := *originalConfig.DestIncludeArchived
		duplicateConfig.DestIncludeArchived = &destIncludeArchivedVal
	}

	if originalConfig.SourceIncludeArchived != nil {
		sourceIncludeArchivedVal := *originalConfig.SourceIncludeArchived
		duplicateConfig.SourceIncludeArchived = &sourceIncludeArchivedVal
	}

	if originalConfig.UseBuiltinAuthSource != nil {
		useBuiltinAuthSourceVal := *originalConfig.UseBuiltinAuthSource
		duplicateConfig.UseBuiltinAuthSource = &useBuiltinAuthSourceVal
	}

	if originalConfig.UseBuiltinAuthDest != nil {
		useBuiltinAuthDestVal := *originalConfig.UseBuiltinAuthDest
		duplicateConfig.UseBuiltinAuthDest = &useBuiltinAuthDestVal
	}

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		log.Printf("Error beginning transaction: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	if err := tx.Create(&duplicateConfig).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating duplicate config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create duplicate config: %v", err)})
		return
	}

	// Create audit log entry
	auditDetails := map[string]interface{}{
		"name":                  duplicateConfig.Name,
		"source_type":           duplicateConfig.SourceType,
		"dest_type":             duplicateConfig.DestinationType,
		"source_path":           duplicateConfig.SourcePath,
		"dest_path":             duplicateConfig.DestinationPath,
		"skip_processed_files":  *duplicateConfig.SkipProcessedFiles,
		"archive_enabled":       *duplicateConfig.ArchiveEnabled,
		"delete_after_transfer": *duplicateConfig.DeleteAfterTransfer,
		"source_passive_mode":   *duplicateConfig.SourcePassiveMode,
		"dest_passive_mode":     *duplicateConfig.DestPassiveMode,
		"duplicated_from":       originalConfig.ID,
	}

	auditLog := db.AuditLog{
		Action:     "duplicate",
		EntityType: "config",
		EntityID:   duplicateConfig.ID,
		UserID:     userID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating audit log: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Generate rclone config file for the duplicate
	if err := h.DB.GenerateRcloneConfig(&duplicateConfig); err != nil {
		log.Printf("Warning: Failed to generate rclone config for duplicate: %v", err)
		// Continue anyway, as the config was created in the database
	} else {
		log.Printf("Generated rclone config for duplicate config ID %d", duplicateConfig.ID)
	}

	// Return with full page reload to show the new config
	c.Header("HX-Refresh", "true")
	c.JSON(http.StatusOK, gin.H{"message": "Config duplicated successfully"})
}
