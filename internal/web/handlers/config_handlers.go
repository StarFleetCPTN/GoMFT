package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/storage"
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
	userID := c.GetUint("userID")

	// Fetch source and destination providers for the user
	sourceProviders, err := h.DB.GetStorageProviders(userID)
	if err != nil {
		log.Printf("Warning: Failed to fetch source providers: %v", err)
		sourceProviders = []db.StorageProvider{} // Use empty slice if there's an error
	}

	destinationProviders, err := h.DB.GetStorageProviders(userID)
	if err != nil {
		log.Printf("Warning: Failed to fetch destination providers: %v", err)
		destinationProviders = []db.StorageProvider{} // Use empty slice if there's an error
	}

	data := components.ConfigFormData{
		Config:               &db.TransferConfig{},
		IsNew:                true,
		SourceProviders:      sourceProviders,
		DestinationProviders: destinationProviders,
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

	// Fetch the initial command details for pre-rendering flags
	initialCommand, err := h.DB.GetRcloneCommandWithFlags(config.CommandID)
	if err != nil {
		// Log the error but proceed, the form might still be usable without pre-rendered flags
		log.Printf("Warning: Failed to get initial command flags for config %d: %v", config.ID, err)
		initialCommand = nil // Ensure it's nil if fetching failed
	}

	// Parse the selected flags and values from the config
	selectedFlagsMap := make(map[uint]bool)
	if config.CommandFlags != "" {
		var selectedFlagIDs []uint
		// Use json.Unmarshal directly as CommandFlags should be a JSON array string
		if err := json.Unmarshal([]byte(config.CommandFlags), &selectedFlagIDs); err == nil {
			for _, id := range selectedFlagIDs {
				selectedFlagsMap[id] = true
			}
		} else {
			log.Printf("Warning: Failed to unmarshal CommandFlags for config %d: %v. JSON: %s", config.ID, err, config.CommandFlags)
		}
	}

	selectedFlagValues := make(map[uint]string)
	if config.CommandFlagValues != "" {
		// Use json.Unmarshal directly as CommandFlagValues should be a JSON object string
		if err := json.Unmarshal([]byte(config.CommandFlagValues), &selectedFlagValues); err != nil {
			log.Printf("Warning: Failed to unmarshal CommandFlagValues for config %d: %v. JSON: %s", config.ID, err, config.CommandFlagValues)
			selectedFlagValues = make(map[uint]string) // Reset on error
		}
	}

	// Fetch source and destination providers for the user
	sourceProviders, err := h.DB.GetStorageProviders(userID)
	if err != nil {
		log.Printf("Warning: Failed to fetch source providers: %v", err)
		sourceProviders = []db.StorageProvider{} // Use empty slice if there's an error
	}

	destinationProviders, err := h.DB.GetStorageProviders(userID)
	if err != nil {
		log.Printf("Warning: Failed to fetch destination providers: %v", err)
		destinationProviders = []db.StorageProvider{} // Use empty slice if there's an error
	}

	data := components.ConfigFormData{
		Config:               &config,
		IsNew:                false,
		InitialCommand:       initialCommand,
		SelectedFlagsMap:     selectedFlagsMap,
		SelectedFlagValues:   selectedFlagValues,
		SourceProviders:      sourceProviders,
		DestinationProviders: destinationProviders,
	}
	components.ConfigForm(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleCreateConfig handles the POST /configs route
func (h *Handlers) HandleCreateConfig(c *gin.Context) {
	var config db.TransferConfig

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

	// Debug information for provider types handling
	useSourceProvider := c.PostForm("use_source_provider") == "true"
	log.Printf("DEBUG: useSourceProvider: %v", useSourceProvider)
	sourceProviderIDStr := c.PostForm("source_provider_id")
	log.Printf("DEBUG: sourceProviderIDStr: '%s'", sourceProviderIDStr)

	useDestProvider := c.PostForm("use_destination_provider") == "true"
	log.Printf("DEBUG: useDestProvider: %v", useDestProvider)
	destProviderIDStr := c.PostForm("destination_provider_id")
	log.Printf("DEBUG: destProviderIDStr: '%s'", destProviderIDStr)

	// Handle provider references, ensuring we have valid provider types
	if useSourceProvider && sourceProviderIDStr != "" {
		sourceProviderID, err := strconv.ParseUint(sourceProviderIDStr, 10, 32)
		if err == nil {
			providerID := uint(sourceProviderID)

			// Just verify the provider exists without loading the full object
			exists, err := h.getProviderIDOnly(providerID)
			if err != nil {
				log.Printf("Error checking source provider %d: %v", providerID, err)
				c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to check source provider: %v", err))
				return
			}

			if !exists {
				log.Printf("Source provider %d not found", providerID)
				c.String(http.StatusBadRequest, "Source provider not found")
				return
			}

			// Set only the ID in the config
			config.SourceProviderID = &providerID

			// Use the type from the form for source type mapping
			sourceProviderType, err := h.DB.GetStorageProviderType(providerID)
			if err != nil {
				log.Printf("Error fetching source provider type: %v", err)
				c.String(http.StatusInternalServerError, "Failed to fetch source provider type")
				return
			}
			config.SourceType = string(sourceProviderType)
			log.Printf("DEBUG: Using source type '%s' from form", sourceProviderType)
		} else {
			log.Printf("Error parsing source provider ID '%s': %v", sourceProviderIDStr, err)
		}
	} else {
		// Clear provider reference if not using a provider
		config.SourceProviderID = nil
		if config.SourceType == "" {
			log.Printf("ERROR: No source type provided when not using a provider reference")
			c.String(http.StatusBadRequest, "Invalid configuration: Source type is required when not using a provider reference")
			return
		}
	}

	if useDestProvider && destProviderIDStr != "" {
		destProviderID, err := strconv.ParseUint(destProviderIDStr, 10, 32)
		if err == nil {
			providerID := uint(destProviderID)

			// Just verify the provider exists without loading the full object
			exists, err := h.getProviderIDOnly(providerID)
			if err != nil {
				log.Printf("Error checking destination provider %d: %v", providerID, err)
				c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to check destination provider: %v", err))
				return
			}

			if !exists {
				log.Printf("Destination provider %d not found", providerID)
				c.String(http.StatusBadRequest, "Destination provider not found")
				return
			}

			// Set only the ID in the config
			config.DestinationProviderID = &providerID

			// Use the type from the form for destination type mapping
			destinationProviderType, err := h.DB.GetStorageProviderType(providerID)
			if err != nil {
				log.Printf("Error fetching destination provider type: %v", err)
				c.String(http.StatusInternalServerError, "Failed to fetch destination provider type")
				return
			}
			config.DestinationType = string(destinationProviderType)
			log.Printf("DEBUG: Using destination type '%s' from form", destinationProviderType)
		} else {
			log.Printf("Error parsing destination provider ID '%s': %v", destProviderIDStr, err)
		}
	} else {
		// Clear provider reference if not using a provider
		config.DestinationProviderID = nil
		if config.DestinationType == "" {
			log.Printf("ERROR: No destination type provided when not using a provider reference")
			c.String(http.StatusBadRequest, "Invalid configuration: Destination type is required when not using a provider reference")
			return
		}
	}

	// Final check to ensure we have valid types
	if config.SourceType == "" {
		log.Printf("ERROR: Source type is empty after all processing")
		c.String(http.StatusBadRequest, "Invalid configuration: Source type cannot be empty")
		return
	}

	if config.DestinationType == "" {
		log.Printf("ERROR: Destination type is empty after all processing")
		c.String(http.StatusBadRequest, "Invalid configuration: Destination type cannot be empty")
		return
	}

	// Log final types before database operations
	log.Printf("DEBUG: Final config types - SourceType: '%s', DestinationType: '%s'", config.SourceType, config.DestinationType)

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

	// Process flag values for non-boolean flags
	flagValues := make(map[uint]string)
	for key, values := range c.Request.PostForm {
		// Check if key is a flag value field (format: flag_value_ID)
		if strings.HasPrefix(key, "flag_value_") {
			flagIDStr := strings.TrimPrefix(key, "flag_value_")
			flagID, err := strconv.ParseUint(flagIDStr, 10, 64)
			if err != nil {
				log.Printf("Error parsing flag value ID: %v", err)
				continue
			}

			// Only process if the corresponding enable checkbox is checked
			enableKey := fmt.Sprintf("flag_enable_%s", flagIDStr)
			enableValue := c.Request.PostForm.Get(enableKey)
			if enableValue == "on" && len(values) > 0 && values[0] != "" {
				flagValues[uint(flagID)] = values[0]
			}
		}
	}

	// Store flag values as JSON if any exist
	if len(flagValues) > 0 {
		flagValuesJSON, err := json.Marshal(flagValues)
		if err != nil {
			log.Printf("Error marshaling flag values: %v", err)
		} else {
			config.CommandFlagValues = string(flagValuesJSON)
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

	if err := tx.Save(&config).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating config: %v", err)
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to update config: %v", err))
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
	// Debug log the entire form for inspection
	log.Printf("DEBUG: Form data received in HandleUpdateConfig: %+v", c.Request.PostForm)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid ID: %v", err))
		return
	}

	// Load the existing config with its current providers
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

	// Create a new config instance for the updated values
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

	// Process provider references
	useSourceProvider := c.PostForm("use_source_provider") == "true"
	sourceProviderIDStr := c.PostForm("source_provider_id")
	if useSourceProvider && sourceProviderIDStr != "" {
		sourceProviderID, err := strconv.ParseUint(sourceProviderIDStr, 10, 32)
		if err == nil {
			providerID := uint(sourceProviderID)
			provider, err := h.DB.GetStorageProvider(providerID)
			if err != nil {
				log.Printf("Error loading source provider %d: %v", providerID, err)
				c.String(http.StatusBadRequest, "Source provider not found or invalid")
				return
			}
			config.SetSourceProvider(provider)
		} else {
			log.Printf("Error parsing source provider ID '%s': %v", sourceProviderIDStr, err)
			c.String(http.StatusBadRequest, "Invalid source provider ID format")
			return
		}
	} else {
		config.SourceProviderID = nil
		config.SourceProvider = nil
		if config.SourceType == "" {
			c.String(http.StatusBadRequest, "Source type is required when not using a provider")
			return
		}
	}

	useDestProvider := c.PostForm("use_destination_provider") == "true"
	destProviderIDStr := c.PostForm("destination_provider_id")
	if useDestProvider && destProviderIDStr != "" {
		destProviderID, err := strconv.ParseUint(destProviderIDStr, 10, 32)
		if err == nil {
			providerID := uint(destProviderID)
			provider, err := h.DB.GetStorageProvider(providerID)
			if err != nil {
				log.Printf("Error loading destination provider %d: %v", providerID, err)
				c.String(http.StatusBadRequest, "Destination provider not found or invalid")
				return
			}
			config.SetDestinationProvider(provider)
		} else {
			log.Printf("Error parsing destination provider ID '%s': %v", destProviderIDStr, err)
			c.String(http.StatusBadRequest, "Invalid destination provider ID format")
			return
		}
	} else {
		config.DestinationProviderID = nil
		config.DestinationProvider = nil
		if config.DestinationType == "" {
			c.String(http.StatusBadRequest, "Destination type is required when not using a provider")
			return
		}
	}

	// Validate the provider configuration
	if err := config.ValidateProviderConfiguration(); err != nil {
		log.Printf("Provider configuration validation failed: %v", err)
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid provider configuration: %v", err))
		return
	}

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		log.Printf("Error beginning transaction: %v", tx.Error)
		c.String(http.StatusInternalServerError, "Failed to begin transaction")
		return
	}

	// Save the config
	if err := tx.Save(&config).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating config: %v", err)
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to update config: %v", err))
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
		Action:     "update",
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
		// Continue anyway, as the config was updated in the database
	} else {
		log.Printf("Generated rclone config for config ID %d", config.ID)
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

// HandleTestProviderConnection tests a connection to a storage provider
func (h *Handlers) HandleTestProviderConnection(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get provider type from form values (source or destination)
	providerType := c.PostForm("providerType")
	if providerType != "source" && providerType != "destination" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid provider type. Must be 'source' or 'destination'",
		})
		return
	}

	// Check if using a provider reference
	var providerID uint
	var err error

	if providerType == "source" {
		if c.PostForm("use_source_provider") == "true" && c.PostForm("source_provider_id") != "" {
			providerIDStr := c.PostForm("source_provider_id")
			id, err := strconv.ParseUint(providerIDStr, 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": "Invalid source provider ID",
				})
				return
			}
			providerID = uint(id)

			// Verify the provider exists using our lightweight method
			exists, err := h.getProviderIDOnly(providerID)
			if err != nil || !exists {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": "Source provider not found",
				})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Source provider not selected",
			})
			return
		}
	} else { // destination
		if c.PostForm("use_destination_provider") == "true" && c.PostForm("destination_provider_id") != "" {
			providerIDStr := c.PostForm("destination_provider_id")
			id, err := strconv.ParseUint(providerIDStr, 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": "Invalid destination provider ID",
				})
				return
			}
			providerID = uint(id)

			// Verify the provider exists
			provider, err := h.DB.GetStorageProvider(providerID)
			if err != nil || provider == nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": "Destination provider not found",
				})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Destination provider not selected",
			})
			return
		}
	}

	// Create connector service
	connectorService, err := storage.NewConnectorService(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to initialize connection service",
			"error": map[string]string{
				"code": "service_error",
			},
		})
		return
	}

	// Test the connection
	result, err := connectorService.TestConnection(c.Request.Context(), providerID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Connection test failed: %v", err),
			"error": map[string]string{
				"code": "test_failed",
			},
		})
		return
	}

	// Return the result
	response := gin.H{
		"success": result.Success,
		"message": result.Message,
	}

	if !result.Success && result.Error != nil {
		response["error"] = map[string]string{
			"code": result.Error.Code,
		}
	}

	c.JSON(http.StatusOK, response)
}

// Search for source provider by ID, without triggering a full provider load/validation
func (h *Handlers) getProviderIDOnly(providerID uint) (bool, error) {
	var count int64
	err := h.DB.Model(&db.StorageProvider{}).Where("id = ?", providerID).Count(&count).Error
	return count > 0, err
}
