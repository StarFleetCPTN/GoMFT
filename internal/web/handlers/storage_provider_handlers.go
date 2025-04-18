package handlers

import (
	"encoding/json"
	"fmt"
	"io"
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

// HandleStorageProviderOptions returns HTML options for storage provider dropdowns
func (h *Handlers) HandleStorageProviderOptions(c *gin.Context) {
	// Get user ID from context
	userID := c.GetUint("userID")

	providers, err := h.DB.GetStorageProviders(userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "", "<option value=\"\">Error loading providers</option>")
		return
	}

	// Return HTML for option elements
	var html strings.Builder
	html.WriteString("<option value=\"\">Select a provider...</option>")

	for _, provider := range providers {
		html.WriteString(fmt.Sprintf("<option value=\"%d\">%s (%s)</option>",
			provider.ID,
			provider.Name,
			provider.Type))
	}

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html.String())
}

// Handler for listing all storage providers
func (h *Handlers) HandleListStorageProviders(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get all storage providers for this user
	providers, err := h.DB.GetStorageProviders(userID)
	if err != nil {
		ctx := components.CreateTemplateContext(c)
		_ = components.StorageProviders(ctx, components.StorageProvidersData{
			Error: "Failed to retrieve storage providers",
		}).Render(ctx, c.Writer)
		return
	}

	// Render template
	ctx := components.CreateTemplateContext(c)
	_ = components.StorageProviders(ctx, components.StorageProvidersData{
		Providers: providers,
		Status:    c.Query("status"),
		Error:     c.Query("error"),
	}).Render(ctx, c.Writer)
}

// Handler for showing the new storage provider form
func (h *Handlers) HandleNewStorageProvider(c *gin.Context) {
	// Create an empty provider
	provider := db.StorageProvider{}

	// Render template
	ctx := components.CreateTemplateContext(c)
	_ = components.StorageProviderForm(ctx, components.StorageProviderFormData{
		Provider: &provider,
		IsEdit:   false,
	}).Render(ctx, c.Writer)
}

// Handler for creating a new storage provider
func (h *Handlers) HandleCreateStorageProvider(c *gin.Context) {
	userID := c.GetUint("userID")

	// Parse form input
	provider, parseErr := h.parseProviderFromForm(c)
	if parseErr != nil {
		ctx := components.CreateTemplateContext(c)
		_ = components.StorageProviderForm(ctx, components.StorageProviderFormData{
			Provider: &provider,
			IsEdit:   false,
			Error:    parseErr.Error(),
		}).Render(ctx, c.Writer)
		return
	}

	// Set created by
	provider.CreatedBy = userID

	// Create provider in database
	err := h.DB.CreateStorageProvider(&provider)
	if err != nil {
		ctx := components.CreateTemplateContext(c)
		_ = components.StorageProviderForm(ctx, components.StorageProviderFormData{
			Provider: &provider,
			IsEdit:   false,
			Error:    fmt.Sprintf("Failed to create storage provider: %v", err),
		}).Render(ctx, c.Writer)
		return
	}

	// Test if requested
	if c.PostForm("test") == "true" {
		// Create connector service
		connectorService, err := storage.NewConnectorService(h.DB)
		if err != nil {
			c.Redirect(http.StatusFound, fmt.Sprintf("/storage-providers?status=created&error=Created but failed to test connection: %v", err))
			return
		}

		// Test connection
		result, err := connectorService.TestConnection(c.Request.Context(), provider.ID, userID)
		if err != nil {
			c.Redirect(http.StatusFound, fmt.Sprintf("/storage-providers?status=created&error=Created but failed to test connection: %v", err))
			return
		}

		if result.Success {
			c.Redirect(http.StatusFound, "/storage-providers?status=created&test_status=success")
		} else {
			errMsg := result.Message
			if result.Error != nil {
				errMsg = fmt.Sprintf("%s (%s)", result.Message, result.Error.Code)
			}
			c.Redirect(http.StatusFound, fmt.Sprintf("/storage-providers?status=created&test_status=failed&error=%s", errMsg))
		}
		return
	}

	// Redirect to list with success message
	c.Redirect(http.StatusFound, "/storage-providers?status=created")
}

// Handler for showing the edit storage provider form
func (h *Handlers) HandleEditStorageProvider(c *gin.Context) {
	userID := c.GetUint("userID")
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.Redirect(http.StatusFound, "/storage-providers?error=Invalid provider ID")
		return
	}

	// Get provider from database
	provider, err := h.DB.GetStorageProviderWithOwnerCheck(uint(id), userID)
	if err != nil {
		c.Redirect(http.StatusFound, "/storage-providers?error=Storage provider not found")
		return
	}

	// Render template
	ctx := components.CreateTemplateContext(c)
	_ = components.StorageProviderForm(ctx, components.StorageProviderFormData{
		Provider: provider,
		IsEdit:   true,
	}).Render(ctx, c.Writer)
}

// Handler for updating an existing storage provider
func (h *Handlers) HandleUpdateStorageProvider(c *gin.Context) {
	userID := c.GetUint("userID")
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.Redirect(http.StatusFound, "/storage-providers?error=Invalid provider ID")
		return
	}

	// Get existing provider from database
	existingProvider, err := h.DB.GetStorageProviderWithOwnerCheck(uint(id), userID)
	if err != nil {
		c.Redirect(http.StatusFound, "/storage-providers?error=Storage provider not found")
		return
	}

	// Parse form input
	provider, parseErr := h.parseProviderFromForm(c)
	if parseErr != nil {
		ctx := components.CreateTemplateContext(c)
		_ = components.StorageProviderForm(ctx, components.StorageProviderFormData{
			Provider: existingProvider,
			IsEdit:   true,
			Error:    parseErr.Error(),
		}).Render(ctx, c.Writer)
		return
	}

	// Set ID and created by
	provider.ID = existingProvider.ID
	provider.CreatedBy = existingProvider.CreatedBy

	// For sensitive fields, if they're empty, keep the existing values
	// These fields should not get exported to the form and back
	if provider.Password == "" {
		provider.EncryptedPassword = existingProvider.EncryptedPassword
	}
	if provider.SecretKey == "" {
		provider.EncryptedSecretKey = existingProvider.EncryptedSecretKey
	}
	if provider.ClientSecret == "" {
		provider.EncryptedClientSecret = existingProvider.EncryptedClientSecret
	}
	if provider.RefreshToken == "" {
		provider.EncryptedRefreshToken = existingProvider.EncryptedRefreshToken
	}

	// Update provider in database
	err = h.DB.UpdateStorageProvider(&provider)
	if err != nil {
		ctx := components.CreateTemplateContext(c)
		_ = components.StorageProviderForm(ctx, components.StorageProviderFormData{
			Provider: &provider,
			IsEdit:   true,
			Error:    fmt.Sprintf("Failed to update storage provider: %v", err),
		}).Render(ctx, c.Writer)
		return
	}

	// Test if requested
	if c.PostForm("test") == "true" {
		// Create connector service
		connectorService, err := storage.NewConnectorService(h.DB)
		if err != nil {
			c.Redirect(http.StatusFound, fmt.Sprintf("/storage-providers?status=updated&error=Updated but failed to test connection: %v", err))
			return
		}

		// Test connection
		result, err := connectorService.TestConnection(c.Request.Context(), provider.ID, userID)
		if err != nil {
			c.Redirect(http.StatusFound, fmt.Sprintf("/storage-providers?status=updated&error=Updated but failed to test connection: %v", err))
			return
		}

		if result.Success {
			c.Redirect(http.StatusFound, "/storage-providers?status=updated&test_status=success")
		} else {
			errMsg := result.Message
			if result.Error != nil {
				errMsg = fmt.Sprintf("%s (%s)", result.Message, result.Error.Code)
			}
			c.Redirect(http.StatusFound, fmt.Sprintf("/storage-providers?status=updated&test_status=failed&error=%s", errMsg))
		}
		return
	}

	// Redirect to list with success message
	c.Redirect(http.StatusFound, "/storage-providers?status=updated")
}

// Handler for deleting a storage provider
func (h *Handlers) HandleDeleteStorageProvider(c *gin.Context) {
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	// Check if provider is used in any transfer configs
	var count int64
	if err := h.DB.Model(&db.TransferConfig{}).
		Where("source_provider_id = ? OR destination_provider_id = ?", id, id).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to check dependencies: %v", err)})
		return
	}

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This provider is being used by one or more transfer configurations"})
		return
	}

	// Delete provider
	err = h.DB.DeleteStorageProvider(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete provider: %v", err)})
		return
	}

	c.Header("HX-Refresh", "true")
	c.JSON(http.StatusOK, gin.H{"message": "Provider deleted successfully"})
}

// Handler for testing a storage provider connection
func (h *Handlers) HandleTestStorageProvider(c *gin.Context) {
	userID := c.GetUint("userID")
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid provider ID"})
		return
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
	result, err := connectorService.TestConnection(c.Request.Context(), uint(id), userID)
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

// Handler for duplicating a storage provider
func (h *Handlers) HandleDuplicateStorageProvider(c *gin.Context) {
	userID := c.GetUint("userID")
	idParam := c.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	// Get the original provider
	originalProvider, err := h.DB.GetStorageProviderWithOwnerCheck(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage provider not found or you do not have permission"})
		return
	}

	// Duplicate the provider
	duplicateProvider := *originalProvider
	duplicateProvider.ID = 0 // New record
	duplicateProvider.Name = originalProvider.Name + " - Copy"
	duplicateProvider.CreatedBy = userID
	duplicateProvider.CreatedAt = time.Now()
	duplicateProvider.UpdatedAt = time.Now()
	// Deep copy pointer fields here if any are added in the future

	// Save the duplicate
	err = h.DB.CreateStorageProvider(&duplicateProvider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create duplicate provider: " + err.Error()})
		return
	}

	c.Header("HX-Refresh", "true")
	c.JSON(http.StatusOK, gin.H{"message": "Provider duplicated successfully"})
}

// Handler for rendering the import page
func (h *Handlers) HandleStorageProvidersImportPage(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	_ = components.StorageProvidersImportPage(ctx, components.RcloneImportPreview{}).Render(ctx, c.Writer)
}

// Handler for previewing rclone config remotes
func (h *Handlers) HandleStorageProvidersImportPreview(c *gin.Context) {
	userID := c.GetUint("userID")
	file, _, err := c.Request.FormFile("rclone_config")
	if err != nil {
		ctx := components.CreateTemplateContext(c)
		// Check if this is an HTMX request
		if c.GetHeader("HX-Request") == "true" {
			_ = components.RcloneImportPreviewContent(ctx, components.RcloneImportPreview{Error: "Failed to read uploaded file"}).Render(ctx, c.Writer)
		} else {
			_ = components.StorageProvidersImportPage(ctx, components.RcloneImportPreview{Error: "Failed to read uploaded file"}).Render(ctx, c.Writer)
		}
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		ctx := components.CreateTemplateContext(c)
		// Check if this is an HTMX request
		if c.GetHeader("HX-Request") == "true" {
			_ = components.RcloneImportPreviewContent(ctx, components.RcloneImportPreview{Error: "Failed to read file content"}).Render(ctx, c.Writer)
		} else {
			_ = components.StorageProvidersImportPage(ctx, components.RcloneImportPreview{Error: "Failed to read file content"}).Render(ctx, c.Writer)
		}
		return
	}
	parsed, err := parseRcloneConfig(content)
	if err != nil {
		ctx := components.CreateTemplateContext(c)
		// Check if this is an HTMX request
		if c.GetHeader("HX-Request") == "true" {
			_ = components.RcloneImportPreviewContent(ctx, components.RcloneImportPreview{Error: fmt.Sprintf("Failed to parse config: %v", err)}).Render(ctx, c.Writer)
		} else {
			_ = components.StorageProvidersImportPage(ctx, components.RcloneImportPreview{Error: fmt.Sprintf("Failed to parse config: %v", err)}).Render(ctx, c.Writer)
		}
		return
	}

	var remotes []components.RcloneRemotePreview
	for name, section := range parsed {
		provider, _ := storageProviderFromRcloneSection(name, section, userID)
		fields := make(map[string]string)
		for k, v := range section {
			fields[k] = v
		}
		remotes = append(remotes, components.RcloneRemotePreview{
			Name:   provider.Name,
			Type:   string(provider.Type),
			Fields: fields,
			Import: true,
		})
	}
	ctx := components.CreateTemplateContext(c)

	// Check if this is an HTMX request
	if c.GetHeader("HX-Request") == "true" {
		_ = components.RcloneImportPreviewContent(ctx, components.RcloneImportPreview{Remotes: remotes}).Render(ctx, c.Writer)
	} else {
		_ = components.StorageProvidersImportPage(ctx, components.RcloneImportPreview{Remotes: remotes}).Render(ctx, c.Writer)
	}
}

// Handler for confirming import of selected remotes
func (h *Handlers) HandleStorageProvidersImportConfirm(c *gin.Context) {
	userID := c.GetUint("userID")

	// Make sure form is parsed
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		log.Printf("Error parsing form: %v", err)
	}

	// For debugging
	log.Printf("Form data: %+v", c.Request.PostForm)
	log.Printf("Form method: %s", c.Request.Method)

	// Parse remotes from form
	remotes := []db.StorageProvider{}

	// Debug: check for import_ keys
	var importKeys []string
	for key := range c.Request.PostForm {
		if strings.HasPrefix(key, "import_") {
			importKeys = append(importKeys, key)
			log.Printf("Found import key: %s with value: %v", key, c.Request.PostForm[key])
		}
	}
	log.Printf("Import keys found: %v", importKeys)

	for key, vals := range c.Request.PostForm {
		log.Printf("Processing key: %s with values: %v", key, vals)
		if strings.HasPrefix(key, "import_") {
			// Check the value - it might not be exactly "on"
			log.Printf("Import checkbox value: %v", vals)

			// Accept any non-empty value as checked
			if len(vals) > 0 && vals[0] != "" {
				name := strings.TrimPrefix(key, "import_")
				log.Printf("Processing remote: %s", name)
				providerType := db.StorageProviderType(c.PostForm("type_" + name))

				provider := db.StorageProvider{
					Name:      c.PostForm("name_" + name),
					Type:      providerType,
					CreatedBy: userID,
				}

				// Collect all fields for this provider
				fields := map[string]string{}
				for k, v := range c.Request.PostForm {
					if strings.HasPrefix(k, "field_"+name+"_") && len(v) > 0 {
						fieldKey := strings.TrimPrefix(k, "field_"+name+"_")
						fields[fieldKey] = v[0]
					}
				}

				// Map fields to provider struct based on provider type
				switch providerType {
				case db.ProviderTypeGoogleDrive:
					// Map Google Drive specific fields
					for fieldKey, fieldValue := range fields {
						switch fieldKey {
						case "client_id":
							provider.ClientID = fieldValue
						case "client_secret":
							provider.ClientSecret = fieldValue
						case "refresh_token":
							provider.RefreshToken = fieldValue
						case "token":
							// Token is a JSON object containing access_token, refresh_token, etc.
							// Extract refresh_token if not already set
							if provider.RefreshToken == "" {
								// Try to parse the token JSON
								var tokenData map[string]interface{}
								if err := json.Unmarshal([]byte(fieldValue), &tokenData); err == nil {
									if rt, ok := tokenData["refresh_token"].(string); ok && rt != "" {
										provider.RefreshToken = rt
									}
								}
							}
						case "team_drive":
							provider.TeamDrive = fieldValue
						}
					}
					// Set authenticated to true for OAuth providers with refresh token
					if provider.RefreshToken != "" {
						authenticated := true
						provider.Authenticated = &authenticated
					}
				case db.ProviderTypeS3, db.ProviderTypeB2, db.ProviderTypeWasabi, db.ProviderTypeMinio:
					// Map S3-compatible provider fields
					for fieldKey, fieldValue := range fields {
						switch fieldKey {
						case "access_key_id", "access_key":
							provider.AccessKey = fieldValue
						case "secret_access_key", "secret_key":
							provider.SecretKey = fieldValue
						case "endpoint":
							provider.Endpoint = fieldValue
						case "region":
							provider.Region = fieldValue
						case "bucket":
							provider.Bucket = fieldValue
						}
					}
				case db.ProviderTypeSFTP, db.ProviderTypeFTP:
					// Map SFTP/FTP fields
					for fieldKey, fieldValue := range fields {
						switch fieldKey {
						case "host":
							provider.Host = fieldValue
						case "user", "username":
							provider.Username = fieldValue
						case "pass", "password":
							provider.Password = fieldValue
						case "port":
							if port, err := strconv.Atoi(fieldValue); err == nil {
								provider.Port = port
							}
						}
					}
				case db.ProviderTypeSMB:
					// Map SMB fields
					for fieldKey, fieldValue := range fields {
						switch fieldKey {
						case "host":
							provider.Host = fieldValue
						case "user", "username":
							provider.Username = fieldValue
						case "pass", "password":
							provider.Password = fieldValue
						case "domain":
							provider.Domain = fieldValue
						case "share":
							provider.Share = fieldValue
						}
					}
				case db.ProviderTypeOneDrive:
					// Map OneDrive fields
					for fieldKey, fieldValue := range fields {
						switch fieldKey {
						case "client_id":
							provider.ClientID = fieldValue
						case "client_secret":
							provider.ClientSecret = fieldValue
						case "refresh_token":
							provider.RefreshToken = fieldValue
						case "drive_id":
							provider.DriveID = fieldValue
						}
					}
					// Set authenticated to true for OAuth providers with refresh token
					if provider.RefreshToken != "" {
						authenticated := true
						provider.Authenticated = &authenticated
					}
				default:
					// For other provider types, log the fields for debugging
					log.Printf("Unhandled provider type: %s with fields: %v", providerType, fields)
				}

				remotes = append(remotes, provider)
			}
		}
	}

	// Import each selected provider
	var importErrs []string
	var successCount int

	for _, provider := range remotes {
		log.Printf("Importing provider: %+v", provider)
		err := createOrUpdateStorageProvider(h.DB, &provider)
		if err != nil {
			importErrs = append(importErrs, fmt.Sprintf("%s: %v", provider.Name, err))
		} else {
			successCount++
		}
	}

	// Create template context
	ctx := components.CreateTemplateContext(c)
	
	// Prepare result message
	if c.GetHeader("HX-Request") == "true" {
		var resultHTML string

		if len(importErrs) > 0 {
			// Error message
			errorMsg := "<div class=\"mb-4 p-4 text-sm text-red-800 rounded-lg bg-red-50 dark:bg-gray-800 dark:text-red-400\" role=\"alert\">\n"
			errorMsg += "<div class=\"flex items-center\">\n"
			errorMsg += "<i class=\"fas fa-exclamation-circle flex-shrink-0 mr-2\"></i>\n"
			errorMsg += "<span>Failed to import some providers:</span>\n"
			errorMsg += "</div>\n"
			errorMsg += "<ul class=\"mt-1.5 ml-4 list-disc list-inside\">\n"
			for _, err := range importErrs {
				errorMsg += "<li>" + err + "</li>\n"
			}
			errorMsg += "</ul>\n"
			errorMsg += "</div>\n"

			// If some providers were imported successfully
			if successCount > 0 {
				errorMsg += "<div class=\"mb-4 p-4 text-sm text-green-800 rounded-lg bg-green-50 dark:bg-gray-800 dark:text-green-400\" role=\"alert\">\n"
				errorMsg += "<div class=\"flex items-center\">\n"
				errorMsg += "<i class=\"fas fa-check-circle flex-shrink-0 mr-2\"></i>\n"
				errorMsg += fmt.Sprintf("<span>Successfully imported %d provider(s)</span>\n", successCount)
				errorMsg += "</div>\n"
				errorMsg += "</div>\n"
			}

			resultHTML = errorMsg
		} else if successCount > 0 {
			// Success message
			successMsg := "<div class=\"mb-4 p-4 text-sm text-green-800 rounded-lg bg-green-50 dark:bg-gray-800 dark:text-green-400\" role=\"alert\">\n"
			successMsg += "<div class=\"flex items-center\">\n"
			successMsg += "<i class=\"fas fa-check-circle flex-shrink-0 mr-2\"></i>\n"
			successMsg += fmt.Sprintf("<span>Successfully imported %d provider(s)</span>\n", successCount)
			successMsg += "</div>\n"
			successMsg += "</div>\n"

			resultHTML = successMsg
		} else {
			// No providers selected
			resultHTML = "<div class=\"mb-4 p-4 text-sm text-blue-800 rounded-lg bg-blue-50 dark:bg-gray-800 dark:text-blue-400\" role=\"alert\">\n"
			resultHTML += "<div class=\"flex items-center\">\n"
			resultHTML += "<i class=\"fas fa-info-circle flex-shrink-0 mr-2\"></i>\n"
			resultHTML += "<span>No providers were selected for import</span>\n"
			resultHTML += "</div>\n"
			resultHTML += "</div>\n"
		}

		// Add buttons
		resultHTML += "<div class=\"mt-6\">\n"
		resultHTML += "<a href=\"/storage-providers\" class=\"text-white bg-blue-700 hover:bg-blue-800 focus:ring-4 focus:ring-blue-300 font-medium rounded-lg text-sm px-5 py-2.5 dark:bg-blue-600 dark:hover:bg-blue-700 focus:outline-none dark:focus:ring-blue-800\">\n"
		resultHTML += "<i class=\"fas fa-list mr-2\"></i>View All Providers\n"
		resultHTML += "</a>\n"
		resultHTML += "<a href=\"/storage-providers/import\" class=\"ml-2 text-gray-900 bg-white border border-gray-300 focus:outline-none hover:bg-gray-100 focus:ring-4 focus:ring-gray-200 font-medium rounded-lg text-sm px-5 py-2.5 dark:bg-gray-800 dark:text-white dark:border-gray-600 dark:hover:bg-gray-700 dark:hover:border-gray-600 dark:focus:ring-gray-700\">\n"
		resultHTML += "<i class=\"fas fa-file-import mr-2\"></i>Import Another Config\n"
		resultHTML += "</a>\n"
		resultHTML += "</div>\n"

		// Send response
		c.Writer.Header().Set("Content-Type", "text/html")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Write([]byte(resultHTML))
	} else {
		// For regular requests, redirect to storage providers page with a flash message
		if len(importErrs) > 0 {
			// Show error page
			_ = components.StorageProvidersImportPage(ctx, components.RcloneImportPreview{Error: strings.Join(importErrs, "; ")}).Render(ctx, c.Writer)
		} else {
			// Redirect to storage providers page on success
			c.Redirect(http.StatusSeeOther, "/storage-providers")
		}
	}
}

// Handler for importing rclone config file
func (h *Handlers) HandleImportRcloneConfig(c *gin.Context) {
	userID := c.GetUint("userID")
	file, _, err := c.Request.FormFile("rclone_config")
	if err != nil {
		c.JSON(400, gin.H{"error": "Missing file: " + err.Error()})
		return
	}
	defer file.Close()

	// Read the file content
	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to read file: " + err.Error()})
		return
	}

	// Parse as INI (rclone config format)
	cfg, err := parseRcloneConfig(content)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid rclone config: " + err.Error()})
		return
	}

	imported := 0
	failed := 0
	var errors []string
	for name, section := range cfg {
		provider, err := storageProviderFromRcloneSection(name, section, userID)
		if err != nil {
			failed++
			errors = append(errors, name+": "+err.Error())
			continue
		}
		// Try to create or update
		err = createOrUpdateStorageProvider(h.DB, &provider)
		if err != nil {
			failed++
			errors = append(errors, name+": "+err.Error())
			continue
		}
		imported++
	}
	if failed == 0 {
		c.JSON(200, gin.H{"message": "Imported successfully", "imported": imported})
	} else {
		c.JSON(400, gin.H{"error": "Some remotes failed", "imported": imported, "failed": failed, "details": errors})
	}
}

// Helper: parse rclone config INI into map[string]map[string]string
func parseRcloneConfig(content []byte) (map[string]map[string]string, error) {
	cfg := make(map[string]map[string]string)
	var current string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			cfg[current] = make(map[string]string)
			continue
		}
		if current == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			cfg[current][strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return cfg, nil
}

// Helper: create or update provider (fallback if DB method not present)
func createOrUpdateStorageProvider(dbh *db.DB, provider *db.StorageProvider) error {
	existing, err := dbh.GetStorageProviderByNameAndUser(provider.Name, provider.CreatedBy)
	if err == nil && existing != nil {
		provider.ID = existing.ID
		return dbh.UpdateStorageProvider(provider)
	}
	return dbh.CreateStorageProvider(provider)
}

// Helper: convert rclone section to StorageProvider
func storageProviderFromRcloneSection(name string, section map[string]string, userID uint) (db.StorageProvider, error) {
	providerType, ok := section["type"]
	if !ok || providerType == "" {
		providerType = string(db.ProviderTypeGeneric)
	}
	// Optionally: you can check for known types and set generic if not recognized
	knownTypes := map[string]bool{
		"sftp": true, "s3": true, "onedrive": true, "drive": true, "gphotos": true, "ftp": true, "smb": true, "hetzner": true, "local": true, "webdav": true, "nextcloud": true, "b2": true, "wasabi": true, "minio": true,
	}
	if !knownTypes[providerType] {
		providerType = string(db.ProviderTypeGeneric)
	}
	provider := db.StorageProvider{
		Name:      name,
		Type:      db.StorageProviderType(providerType),
		CreatedBy: userID,
	}
	// Map common fields
	for k, v := range section {
		switch k {
		case "host":
			provider.Host = v
		case "user":
			provider.Username = v
		case "pass":
			provider.Password = v
		case "port":
			if port, err := strconv.Atoi(v); err == nil {
				provider.Port = port
			}
		case "bucket":
			provider.Bucket = v
		case "region":
			provider.Region = v
		case "access_key_id":
			provider.AccessKey = v
		case "secret_access_key":
			provider.SecretKey = v
		case "endpoint":
			provider.Endpoint = v
		case "domain":
			provider.Domain = v
			// Add more mappings as needed
		}
	}
	return provider, nil
}

// Helper function to parse provider from form
func (h *Handlers) parseProviderFromForm(c *gin.Context) (db.StorageProvider, error) {
	provider := db.StorageProvider{}

	// Basic info
	provider.Name = c.PostForm("name")
	provider.Type = db.StorageProviderType(c.PostForm("type"))

	// Validate required fields
	if provider.Name == "" {
		return provider, fmt.Errorf("provider name is required")
	}

	if provider.Type == "" {
		return provider, fmt.Errorf("provider type is required")
	}

	// Parse common fields
	provider.Host = c.PostForm("host")
	if portStr := c.PostForm("port"); portStr != "" {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return provider, fmt.Errorf("invalid port number")
		}
		provider.Port = int(port)
	}

	// Additional fields based on provider type
	switch provider.Type {
	case db.ProviderTypeSFTP, db.ProviderTypeFTP, db.ProviderTypeSMB, db.ProviderTypeHetzner:
		// Username and password
		provider.Username = c.PostForm("username")
		provider.Password = c.PostForm("password")

		// For SFTP also get key file
		if provider.Type == db.ProviderTypeSFTP || provider.Type == db.ProviderTypeHetzner {
			provider.KeyFile = c.PostForm("keyFile")
		}

		// For SMB also get domain
		if provider.Type == db.ProviderTypeSMB {
			provider.Domain = c.PostForm("domain")
		}

		// For FTP also get passive mode
		if provider.Type == db.ProviderTypeFTP {
			passiveMode := c.PostForm("passiveMode") == "true"
			provider.PassiveMode = &passiveMode
		}

		// Validate required fields
		if provider.Username == "" {
			return provider, fmt.Errorf("username is required")
		}

		if provider.Host == "" {
			return provider, fmt.Errorf("host is required")
		}

	case db.ProviderTypeWebDAV, db.ProviderTypeNextcloud:
		// WebDAV specific fields
		provider.Username = c.PostForm("username")
		provider.Password = c.PostForm("password")

		// Log for debugging
		log.Printf("WebDAV provider: username=%s, password present=%v",
			provider.Username, provider.Password != "")

		// WebDAV uses Host without port (full URL)
		if provider.Host == "" {
			return provider, fmt.Errorf("host/URL is required for WebDAV provider")
		}

		if provider.Username == "" {
			return provider, fmt.Errorf("username is required for WebDAV provider")
		}

	case db.ProviderTypeS3, db.ProviderTypeWasabi, db.ProviderTypeMinio, db.ProviderTypeB2:
		// S3 specific fields
		provider.AccessKey = c.PostForm("accessKey")
		provider.SecretKey = c.PostForm("secretKey")
		provider.Bucket = c.PostForm("bucket")
		provider.Region = c.PostForm("region")
		provider.Endpoint = c.PostForm("endpoint")

		// Validate required fields
		if provider.AccessKey == "" {
			return provider, fmt.Errorf("access key is required")
		}

		if provider.Bucket == "" {
			return provider, fmt.Errorf("bucket is required")
		}

		// Validate region based on provider type
		// B2 doesn't require region or endpoint
		// Wasabi doesn't require region
		if provider.Type == db.ProviderTypeS3 || provider.Type == db.ProviderTypeMinio {
			if provider.Region == "" {
				return provider, fmt.Errorf("region is required for %s", provider.Type)
			}
		}

	case db.ProviderTypeOneDrive, db.ProviderTypeGoogleDrive, db.ProviderTypeGooglePhoto:
		// Cloud storage fields
		provider.ClientID = c.PostForm("clientID")
		provider.ClientSecret = c.PostForm("clientSecret")

		// Google Drive specific fields
		if provider.Type == db.ProviderTypeGoogleDrive {
			provider.DriveID = c.PostForm("driveID")
			provider.TeamDrive = c.PostForm("teamDrive")
		}

		// Google Photos specific fields
		if provider.Type == db.ProviderTypeGooglePhoto {
			readOnly := c.PostForm("readOnly") == "true"
			provider.ReadOnly = &readOnly
		}

		// Validate required fields
		if provider.ClientID == "" {
			return provider, fmt.Errorf("client ID is required")
		}

	case db.ProviderTypeLocal:
		// Local provider uses Host as the path
		provider.Host = c.PostForm("localPath")

		// Validate required fields
		if provider.Host == "" {
			return provider, fmt.Errorf("base path is required")
		}
	}

	return provider, nil
}
