package handlers

import (
	"fmt"
	"log"
	"net/http"

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

	useBuiltinAuthSourceVal := c.Request.FormValue("use_builtin_auth_source")
	useBuiltinAuthSourceValue := useBuiltinAuthSourceVal == "on" || useBuiltinAuthSourceVal == "true"
	config.UseBuiltinAuthSource = &useBuiltinAuthSourceValue

	useBuiltinAuthDestVal := c.Request.FormValue("use_builtin_auth_dest")
	useBuiltinAuthDestValue := useBuiltinAuthDestVal == "on" || useBuiltinAuthDestVal == "true"
	config.UseBuiltinAuthDest = &useBuiltinAuthDestValue

	if err := h.DB.Create(&config).Error; err != nil {
		log.Printf("Error creating config: %v", err)
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create config: %v", err))
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

// HandleUpdateConfig handles the PUT /configs/:id route
func (h *Handlers) HandleUpdateConfig(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")

	var config db.TransferConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		log.Printf("Error finding config: %v", err)
		c.String(http.StatusNotFound, "Config not found")
		return
	}

	// Check if user owns this config
	if config.CreatedBy != userID {
		// Check if user is admin
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.String(http.StatusForbidden, "You do not have permission to update this config")
			return
		}
	}

	// Get the old config values for comparison
	oldConfig := config

	// Bind form data to config
	if err := c.ShouldBind(&config); err != nil {
		log.Printf("Error binding config form: %v", err)
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid form data: %v", err))
		return
	}

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

	useBuiltinAuthSourceVal := c.Request.FormValue("use_builtin_auth_source")
	useBuiltinAuthSourceValue := useBuiltinAuthSourceVal == "on" || useBuiltinAuthSourceVal == "true"
	config.UseBuiltinAuthSource = &useBuiltinAuthSourceValue

	useBuiltinAuthDestVal := c.Request.FormValue("use_builtin_auth_dest")
	useBuiltinAuthDestValue := useBuiltinAuthDestVal == "on" || useBuiltinAuthDestVal == "true"
	config.UseBuiltinAuthDest = &useBuiltinAuthDestValue

	// Preserve fields that shouldn't be updated
	config.CreatedBy = oldConfig.CreatedBy

	if err := h.DB.Save(&config).Error; err != nil {
		log.Printf("Error updating config: %v", err)
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to update config: %v", err))
		return
	}

	// Regenerate rclone config file
	if err := h.DB.GenerateRcloneConfig(&config); err != nil {
		log.Printf("Warning: Failed to regenerate rclone config: %v", err)
		// Continue anyway, as the config was updated in the database
	} else {
		log.Printf("Regenerated rclone config for config ID %d", config.ID)
	}

	c.Redirect(http.StatusFound, "/configs")
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

	// Delete config
	if err := h.DB.Delete(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete config: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Config deleted successfully"})
}
