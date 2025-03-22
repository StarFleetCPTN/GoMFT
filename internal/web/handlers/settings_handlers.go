package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// NotificationService represents a notification service configuration
type NotificationService struct {
	ID              uint              `json:"id" gorm:"primaryKey"`
	Name            string            `json:"name" gorm:"not null"`
	Type            string            `json:"type" gorm:"not null"` // email, slack, webhook
	IsEnabled       bool              `json:"is_enabled" gorm:"default:true"`
	Config          map[string]string `json:"config" gorm:"-"`
	ConfigJSON      string            `json:"-" gorm:"column:config"`
	Description     string            `json:"description"`
	EventTriggers   string            `json:"event_triggers" gorm:"column:event_triggers;default:'[]'"`
	PayloadTemplate string            `json:"payload_template" gorm:"column:payload_template"`
	SecretKey       string            `json:"secret_key" gorm:"column:secret_key"`
	RetryPolicy     string            `json:"retry_policy" gorm:"column:retry_policy;default:'simple'"`
	LastUsed        time.Time         `json:"last_used" gorm:"column:last_used"`
	SuccessCount    int               `json:"success_count" gorm:"column:success_count;default:0"`
	FailureCount    int               `json:"failure_count" gorm:"column:failure_count;default:0"`
	CreatedBy       uint              `json:"created_by"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// BeforeSave converts Config map to JSON string for storage
func (n *NotificationService) BeforeSave() error {
	configJSON, err := json.Marshal(n.Config)
	if err != nil {
		return err
	}
	n.ConfigJSON = string(configJSON)
	return nil
}

// AfterFind converts JSON string back to Config map
func (n *NotificationService) AfterFind() error {
	if n.ConfigJSON != "" {
		return json.Unmarshal([]byte(n.ConfigJSON), &n.Config)
	}
	return nil
}

// HandleSettings handles GET /settings
func (h *Handlers) HandleSettings(c *gin.Context) {
	// Check if the user has permission to view settings
	if !h.checkPermission(c, "system.settings") {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	var notificationServices []NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []components.NotificationService
	for _, service := range notificationServices {
		// Parse event triggers from JSON string to string slice
		var eventTriggers []string
		if service.EventTriggers != "" {
			if err := json.Unmarshal([]byte(service.EventTriggers), &eventTriggers); err != nil {
				log.Printf("Error parsing event triggers: %v", err)
			}
		}

		componentServices = append(componentServices, components.NotificationService{
			ID:              service.ID,
			Name:            service.Name,
			Type:            service.Type,
			IsEnabled:       service.IsEnabled,
			Config:          service.Config,
			Description:     service.Description,
			EventTriggers:   eventTriggers,
			PayloadTemplate: service.PayloadTemplate,
			SecretKey:       service.SecretKey,
			RetryPolicy:     service.RetryPolicy,
			SuccessCount:    service.SuccessCount,
			FailureCount:    service.FailureCount,
		})
	}

	data := components.SettingsData{
		NotificationServices: componentServices,
	}

	ctx := h.CreateTemplateContext(c)
	components.Settings(ctx, data).Render(ctx, c.Writer)
}

// HandleCreateNotificationService handles POST /settings/notifications
func (h *Handlers) HandleCreateNotificationService(c *gin.Context) {
	// Check if the user has permission to manage settings
	if !h.checkPermission(c, "system.settings") {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// Parse form data
	name := c.PostForm("name")
	serviceType := c.PostForm("type")
	description := c.PostForm("description")
	isEnabled := c.PostForm("is_enabled") == "on"

	// Validate required fields
	if name == "" || serviceType == "" {
		// Return to settings page with error message
		h.handleSettingsWithError(c, "Name and type are required fields.")
		return
	}

	// Create config map based on service type
	config := make(map[string]string)

	switch serviceType {
	case "email":
		config["smtp_host"] = c.PostForm("smtp_host")
		config["smtp_port"] = c.PostForm("smtp_port")
		config["smtp_username"] = c.PostForm("smtp_username")
		config["smtp_password"] = c.PostForm("smtp_password")
		config["from_email"] = c.PostForm("from_email")
	case "slack":
		config["webhook_url"] = c.PostForm("webhook_url")
		config["channel"] = c.PostForm("channel")
	case "webhook":
		config["webhook_url"] = c.PostForm("webhook_url")
		config["method"] = c.PostForm("method")
		config["headers"] = c.PostForm("headers")

		// Add the new webhook fields
		// Create event triggers JSON array
		eventTriggers := make([]string, 0)
		if c.PostForm("trigger_job_start") == "on" {
			eventTriggers = append(eventTriggers, "job_start")
		}
		if c.PostForm("trigger_job_complete") == "on" {
			eventTriggers = append(eventTriggers, "job_complete")
		}
		if c.PostForm("trigger_job_error") == "on" {
			eventTriggers = append(eventTriggers, "job_error")
		}

		// Marshal the event triggers to JSON
		eventTriggersJSON, err := json.Marshal(eventTriggers)
		if err != nil {
			h.handleSettingsWithError(c, "Failed to process event triggers: "+err.Error())
			return
		}

		// Create new notification service with additional fields
		service := NotificationService{
			Name:            name,
			Type:            serviceType,
			IsEnabled:       isEnabled,
			Config:          config,
			Description:     description,
			EventTriggers:   string(eventTriggersJSON),
			PayloadTemplate: c.PostForm("payload_template"),
			SecretKey:       c.PostForm("secret_key"),
			RetryPolicy:     c.PostForm("retry_policy"),
			CreatedBy:       c.GetUint("userID"),
		}

		// Save to database
		if err := h.DB.Create(&service).Error; err != nil {
			log.Printf("Error creating notification service: %v", err)
			h.handleSettingsWithError(c, "Failed to create notification service: "+err.Error())
			return
		}

		// Create audit log
		auditDetails := map[string]interface{}{
			"name":           service.Name,
			"type":           service.Type,
			"is_enabled":     service.IsEnabled,
			"description":    service.Description,
			"event_triggers": eventTriggers,
			"retry_policy":   service.RetryPolicy,
			"has_secret_key": service.SecretKey != "",
		}

		auditLog := db.AuditLog{
			Action:     "create",
			EntityType: "notification_service",
			EntityID:   service.ID,
			UserID:     c.GetUint("userID"),
			Details:    auditDetails,
		}

		if err := h.DB.Create(&auditLog).Error; err != nil {
			log.Printf("Error creating audit log: %v", err)
		}

		// Redirect back to settings page with success message
		h.handleSettingsWithSuccess(c, "Notification service created successfully.")
		return
	default:
		h.handleSettingsWithError(c, "Invalid notification service type.")
		return
	}

	// Create new notification service
	service := NotificationService{
		Name:        name,
		Type:        serviceType,
		IsEnabled:   isEnabled,
		Config:      config,
		Description: description,
		CreatedBy:   c.GetUint("userID"),
	}

	// Save to database
	if err := h.DB.Create(&service).Error; err != nil {
		log.Printf("Error creating notification service: %v", err)
		h.handleSettingsWithError(c, "Failed to create notification service: "+err.Error())
		return
	}

	// Create audit log
	auditDetails := map[string]interface{}{
		"name":        service.Name,
		"type":        service.Type,
		"is_enabled":  service.IsEnabled,
		"description": service.Description,
	}

	auditLog := db.AuditLog{
		Action:     "create",
		EntityType: "notification_service",
		EntityID:   service.ID,
		UserID:     c.GetUint("userID"),
		Details:    auditDetails,
	}

	if err := h.DB.Create(&auditLog).Error; err != nil {
		log.Printf("Error creating audit log: %v", err)
	}

	// Redirect back to settings page with success message
	h.handleSettingsWithSuccess(c, "Notification service created successfully.")
}

// HandleDeleteNotificationService handles DELETE /settings/notifications/:id
func (h *Handlers) HandleDeleteNotificationService(c *gin.Context) {
	// Check if the user has permission to manage settings
	if !h.checkPermission(c, "system.settings") {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// Get service ID from path
	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		h.handleSettingsWithError(c, "Invalid notification service ID.")
		return
	}

	// Find service to delete (for audit log)
	var service NotificationService
	if err := h.DB.First(&service, serviceID).Error; err != nil {
		h.handleSettingsWithError(c, "Notification service not found.")
		return
	}

	// Delete the service
	if err := h.DB.Delete(&NotificationService{}, serviceID).Error; err != nil {
		log.Printf("Error deleting notification service: %v", err)
		h.handleSettingsWithError(c, "Failed to delete notification service: "+err.Error())
		return
	}

	// Create audit log
	auditDetails := map[string]interface{}{
		"name":        service.Name,
		"type":        service.Type,
		"is_enabled":  service.IsEnabled,
		"description": service.Description,
	}

	auditLog := db.AuditLog{
		Action:     "delete",
		EntityType: "notification_service",
		EntityID:   service.ID,
		UserID:     c.GetUint("userID"),
		Details:    auditDetails,
	}

	if err := h.DB.Create(&auditLog).Error; err != nil {
		log.Printf("Error creating audit log: %v", err)
	}

	// Redirect back to settings page with success message
	h.handleSettingsWithSuccess(c, "Notification service deleted successfully.")
}

// Helper function to check if user has a specific permission
func (h *Handlers) checkPermission(c *gin.Context, permission string) bool {
	// If user is admin, they have all permissions
	isAdmin, exists := c.Get("isAdmin")
	if exists && isAdmin.(bool) {
		return true
	}

	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		return false
	}

	// Check if user has the required permission
	// h.DB.UserHasPermission undefined (type *db.DB has no field or method UserHasPermission)
	// Load user with roles and check permission
	var user db.User
	if err := h.DB.Preload("Roles").First(&user, userID).Error; err != nil {
		log.Printf("Error loading user: %v", err)
		return false
	}

	return user.HasPermission(permission)
}

// handleSettingsWithError renders the settings page with an error message
func (h *Handlers) handleSettingsWithError(c *gin.Context, errorMessage string) {
	var notificationServices []NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []components.NotificationService
	for _, service := range notificationServices {
		// Parse event triggers from JSON string to string slice
		var eventTriggers []string
		if service.EventTriggers != "" {
			if err := json.Unmarshal([]byte(service.EventTriggers), &eventTriggers); err != nil {
				log.Printf("Error parsing event triggers: %v", err)
			}
		}

		componentServices = append(componentServices, components.NotificationService{
			ID:              service.ID,
			Name:            service.Name,
			Type:            service.Type,
			IsEnabled:       service.IsEnabled,
			Config:          service.Config,
			Description:     service.Description,
			EventTriggers:   eventTriggers,
			PayloadTemplate: service.PayloadTemplate,
			SecretKey:       service.SecretKey,
			RetryPolicy:     service.RetryPolicy,
			SuccessCount:    service.SuccessCount,
			FailureCount:    service.FailureCount,
		})
	}

	data := components.SettingsData{
		NotificationServices: componentServices,
		ErrorMessage:         errorMessage,
	}

	ctx := h.CreateTemplateContext(c)
	components.Settings(ctx, data).Render(ctx, c.Writer)
}

// handleSettingsWithSuccess renders the settings page with a success message
func (h *Handlers) handleSettingsWithSuccess(c *gin.Context, successMessage string) {
	var notificationServices []NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []components.NotificationService
	for _, service := range notificationServices {
		// Parse event triggers from JSON string to string slice
		var eventTriggers []string
		if service.EventTriggers != "" {
			if err := json.Unmarshal([]byte(service.EventTriggers), &eventTriggers); err != nil {
				log.Printf("Error parsing event triggers: %v", err)
			}
		}

		componentServices = append(componentServices, components.NotificationService{
			ID:              service.ID,
			Name:            service.Name,
			Type:            service.Type,
			IsEnabled:       service.IsEnabled,
			Config:          service.Config,
			Description:     service.Description,
			EventTriggers:   eventTriggers,
			PayloadTemplate: service.PayloadTemplate,
			SecretKey:       service.SecretKey,
			RetryPolicy:     service.RetryPolicy,
			SuccessCount:    service.SuccessCount,
			FailureCount:    service.FailureCount,
		})
	}

	data := components.SettingsData{
		NotificationServices: componentServices,
		SuccessMessage:       successMessage,
	}

	ctx := h.CreateTemplateContext(c)
	components.Settings(ctx, data).Render(ctx, c.Writer)
}
