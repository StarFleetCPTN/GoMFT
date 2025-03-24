package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto/hmac"
	"crypto/sha256"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleSettings handles GET /settings
func (h *Handlers) HandleSettings(c *gin.Context) {
	// Check if the user has permission to view settings
	if !h.checkPermission(c, "system.settings") {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	var notificationServices []db.NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []components.NotificationService
	for _, service := range notificationServices {
		componentServices = append(componentServices, components.NotificationService{
			ID:              service.ID,
			Name:            service.Name,
			Type:            service.Type,
			IsEnabled:       service.IsEnabled,
			Config:          service.Config,
			Description:     service.Description,
			EventTriggers:   service.EventTriggers,
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

// HandleCreateNotificationService handles POST /admin/settings/notifications
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
		// Return to notifications page with error message
		h.handleNotificationsWithError(c, "Name and type are required fields.")
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
	case "webhook":
		config["webhook_url"] = c.PostForm("webhook_url")
		config["method"] = c.PostForm("method")
		config["headers"] = c.PostForm("headers")

		// Add the new webhook fields
		// Create event triggers array
		// print all event triggers
		log.Printf("Event triggers: %v", c.PostForm("trigger_job_start"))
		log.Printf("Event triggers: %v", c.PostForm("trigger_job_complete"))
		log.Printf("Event triggers: %v", c.PostForm("trigger_job_error"))
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

		// Create new notification service with additional fields
		service := db.NotificationService{
			Name:            name,
			Type:            serviceType,
			IsEnabled:       isEnabled,
			Config:          config,
			Description:     description,
			EventTriggers:   eventTriggers,
			PayloadTemplate: c.PostForm("payload_template"),
			SecretKey:       c.PostForm("secret_key"),
			RetryPolicy:     c.PostForm("retry_policy"),
			CreatedBy:       c.GetUint("userID"),
		}

		// Save to database
		if err := h.DB.Create(&service).Error; err != nil {
			log.Printf("Error creating notification service: %v", err)
			h.handleNotificationsWithError(c, "Failed to create notification service: "+err.Error())
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

		// Redirect back to notifications page with success message
		h.handleNotificationsWithSuccess(c, "Notification service created successfully.")
		return
	default:
		h.handleNotificationsWithError(c, "Invalid notification service type.")
		return
	}

	// Create new notification service
	service := db.NotificationService{
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
		h.handleNotificationsWithError(c, "Failed to create notification service: "+err.Error())
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

	// Redirect back to notifications page with success message
	h.handleNotificationsWithSuccess(c, "Notification service created successfully.")
}

// HandleDeleteNotificationService handles DELETE /admin/settings/notifications/:id
func (h *Handlers) HandleDeleteNotificationService(c *gin.Context) {
	// Check if the user has permission to manage settings
	if !h.checkPermission(c, "system.settings") {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// Get service ID from path
	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		h.handleNotificationsWithError(c, "Invalid notification service ID.")
		return
	}

	// Find service to delete (for audit log)
	var service db.NotificationService
	if err := h.DB.First(&service, serviceID).Error; err != nil {
		h.handleNotificationsWithError(c, "Notification service not found.")
		return
	}

	// Delete the service
	if err := h.DB.Delete(&db.NotificationService{}, serviceID).Error; err != nil {
		log.Printf("Error deleting notification service: %v", err)
		h.handleNotificationsWithError(c, "Failed to delete notification service: "+err.Error())
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

	// Redirect back to notifications page with success message
	h.handleNotificationsWithSuccess(c, "Notification service deleted successfully.")
}

// HandleTestNotification handles POST /settings/notifications/test
// This endpoint tests a notification configuration without saving it
func (h *Handlers) HandleTestNotification(c *gin.Context) {
	// Check if the user has permission to manage settings
	if !h.checkPermission(c, "system.settings") {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "You don't have permission to test notifications",
		})
		return
	}

	// Parse form data to create a test notification service
	name := c.PostForm("name")
	serviceType := c.PostForm("type")

	// Validate required fields
	if name == "" || serviceType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Name and type are required fields",
		})
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

		// Basic validation
		if config["smtp_host"] == "" || config["smtp_port"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "SMTP host and port are required for email notifications",
			})
			return
		}

	case "webhook":
		config["webhook_url"] = c.PostForm("webhook_url")
		config["method"] = c.PostForm("method")
		config["headers"] = c.PostForm("headers")

		// Basic validation
		if config["webhook_url"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Webhook URL is required for webhook notifications",
			})
			return
		}

		// Get additional webhook fields
		payloadTemplate := c.PostForm("payload_template")
		secretKey := c.PostForm("secret_key")

		// Create and format sample payload
		samplePayload := generateSamplePayload(payloadTemplate)

		// Send test webhook
		err := sendTestWebhook(config, samplePayload, secretKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to send test webhook: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Test webhook sent successfully",
		})
		return

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid notification service type",
		})
		return
	}

	// For email, simulate a successful test for now
	// In a real implementation, you would send an actual test notification
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Simulated %s notification test successful", serviceType),
	})
}

// generateSamplePayload creates a sample payload for testing
func generateSamplePayload(template string) string {
	// If no template provided, use a default sample
	if template == "" {
		return `{
			"event": "job_complete",
			"job": {
				"id": "sample-job-123",
				"name": "Test Job",
				"status": "completed",
				"message": "This is a test notification",
				"started_at": "` + time.Now().Add(-5*time.Minute).Format(time.RFC3339) + `",
				"completed_at": "` + time.Now().Format(time.RFC3339) + `",
				"duration_seconds": 300,
				"config_id": "config-456",
				"config_name": "Test Config",
				"transfer_bytes": 1024,
				"file_count": 5
			},
			"instance": {
				"id": "gomft-instance-1",
				"name": "GoMFT Test Instance",
				"version": "1.0.0",
				"environment": "testing"
			},
			"timestamp": "` + time.Now().Format(time.RFC3339) + `",
			"notification_id": "test-notification"
		}`
	}

	// Replace placeholders in the template with sample values
	samplePayload := template
	// Replace common placeholders
	replacements := map[string]string{
		"{{job.id}}":               "sample-job-123",
		"{{job.name}}":             "Test Job",
		"{{job.status}}":           "completed",
		"{{job.message}}":          "This is a test notification",
		"{{job.event}}":            "job_complete",
		"{{job.started_at}}":       time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		"{{job.completed_at}}":     time.Now().Format(time.RFC3339),
		"{{job.duration_seconds}}": "300",
		"{{job.config_id}}":        "config-456",
		"{{job.config_name}}":      "Test Config",
		"{{job.transfer_bytes}}":   "1024",
		"{{job.file_count}}":       "5",
		"{{instance.id}}":          "gomft-instance-1",
		"{{instance.name}}":        "GoMFT Test Instance",
		"{{instance.version}}":     "1.0.0",
		"{{instance.environment}}": "testing",
		"{{timestamp}}":            time.Now().Format(time.RFC3339),
		"{{notification.id}}":      "test-notification",
	}

	for placeholder, value := range replacements {
		samplePayload = strings.Replace(samplePayload, placeholder, value, -1)
	}

	return samplePayload
}

// sendTestWebhook sends a test webhook to the specified URL
func sendTestWebhook(config map[string]string, payload string, secretKey string) error {
	webhookURL := config["webhook_url"]
	method := config["method"]
	if method == "" {
		method = "POST"
	}

	// Create the request
	req, err := http.NewRequest(method, webhookURL, bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set default Content-Type if not specified
	req.Header.Set("Content-Type", "application/json")

	// Parse and set custom headers
	if config["headers"] != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(config["headers"]), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	// Add signature if secret key is provided
	if secretKey != "" {
		signature := calculateSignature(payload, secretKey)
		req.Header.Set("X-GoMFT-Signature", signature)
	}

	// Send the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending webhook: %v", err)
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// calculateSignature generates an HMAC signature for webhook payloads
func calculateSignature(payload string, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(payload))
	return fmt.Sprintf("sha256=%x", h.Sum(nil))
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

// HandleNotificationsPage handles GET /admin/settings/notifications
func (h *Handlers) HandleNotificationsPage(c *gin.Context) {
	// Check if the user has permission to view settings
	if !h.checkPermission(c, "system.settings") {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	var notificationServices []db.NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []components.NotificationService
	for _, service := range notificationServices {
		componentServices = append(componentServices, components.NotificationService{
			ID:              service.ID,
			Name:            service.Name,
			Type:            service.Type,
			IsEnabled:       service.IsEnabled,
			Config:          service.Config,
			Description:     service.Description,
			EventTriggers:   service.EventTriggers,
			PayloadTemplate: service.PayloadTemplate,
			SecretKey:       service.SecretKey,
			RetryPolicy:     service.RetryPolicy,
			SuccessCount:    service.SuccessCount,
			FailureCount:    service.FailureCount,
		})
	}

	data := components.SettingsNotificationsData{
		NotificationServices: componentServices,
	}

	ctx := h.CreateTemplateContext(c)
	components.Notifications(ctx, data).Render(ctx, c.Writer)
}

// handleNotificationsWithError renders the notifications page with an error message
func (h *Handlers) handleNotificationsWithError(c *gin.Context, errorMessage string) {
	var notificationServices []db.NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []components.NotificationService
	for _, service := range notificationServices {
		componentServices = append(componentServices, components.NotificationService{
			ID:              service.ID,
			Name:            service.Name,
			Type:            service.Type,
			IsEnabled:       service.IsEnabled,
			Config:          service.Config,
			Description:     service.Description,
			EventTriggers:   service.EventTriggers,
			PayloadTemplate: service.PayloadTemplate,
			SecretKey:       service.SecretKey,
			RetryPolicy:     service.RetryPolicy,
			SuccessCount:    service.SuccessCount,
			FailureCount:    service.FailureCount,
		})
	}

	data := components.SettingsNotificationsData{
		NotificationServices: componentServices,
		ErrorMessage:         errorMessage,
	}

	ctx := h.CreateTemplateContext(c)
	components.Notifications(ctx, data).Render(ctx, c.Writer)
}

// handleNotificationsWithSuccess renders the notifications page with a success message
func (h *Handlers) handleNotificationsWithSuccess(c *gin.Context, successMessage string) {
	var notificationServices []db.NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []components.NotificationService
	for _, service := range notificationServices {
		componentServices = append(componentServices, components.NotificationService{
			ID:              service.ID,
			Name:            service.Name,
			Type:            service.Type,
			IsEnabled:       service.IsEnabled,
			Config:          service.Config,
			Description:     service.Description,
			EventTriggers:   service.EventTriggers,
			PayloadTemplate: service.PayloadTemplate,
			SecretKey:       service.SecretKey,
			RetryPolicy:     service.RetryPolicy,
			SuccessCount:    service.SuccessCount,
			FailureCount:    service.FailureCount,
		})
	}

	data := components.SettingsNotificationsData{
		NotificationServices: componentServices,
		SuccessMessage:       successMessage,
	}

	ctx := h.CreateTemplateContext(c)
	components.Notifications(ctx, data).Render(ctx, c.Writer)
}
