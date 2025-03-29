package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"crypto/hmac"
	"crypto/sha256"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/components/notifications/form"
	"github.com/starfleetcptn/gomft/components/notifications/list"
	"github.com/starfleetcptn/gomft/components/notifications/types"
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
	eventTriggers := make([]string, 0)
	fmt.Println("serviceType", serviceType)
	switch serviceType {
	case "email":
		config["smtp_host"] = c.PostForm("smtp_host")
		config["smtp_port"] = c.PostForm("smtp_port")
		config["smtp_username"] = c.PostForm("smtp_username")
		config["smtp_password"] = c.PostForm("smtp_password")
		config["from_email"] = c.PostForm("from_email")

	case "pushbullet":
		config["api_key"] = c.PostForm("pushbullet_api_key")
		config["device_iden"] = c.PostForm("pushbullet_device_iden")
		config["title_template"] = c.PostForm("pushbullet_title_template")
		config["body_template"] = c.PostForm("pushbullet_body_template")

		// Build event triggers
		if c.PostForm("trigger_job_start") == "on" {
			eventTriggers = append(eventTriggers, "job_start")
		}
		if c.PostForm("trigger_job_complete") == "on" {
			eventTriggers = append(eventTriggers, "job_complete")
		}
		if c.PostForm("trigger_job_error") == "on" {
			eventTriggers = append(eventTriggers, "job_error")
		}

		// Validate required fields
		if config["api_key"] == "" {
			h.handleNotificationsWithError(c, "Pushbullet API Key is required.")
			return
		}

		// Create new notification service
		service := db.NotificationService{
			Name:          name,
			Type:          serviceType,
			IsEnabled:     isEnabled,
			Config:        config,
			Description:   description,
			EventTriggers: eventTriggers,
			CreatedBy:     c.GetUint("userID"),
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
		h.handleNotificationsWithSuccess(c, "Pushbullet notification service created successfully.")
		return

	case "ntfy":
		config["server"] = c.PostForm("ntfy_server")
		config["topic"] = c.PostForm("ntfy_topic")
		config["priority"] = c.PostForm("ntfy_priority")
		config["username"] = c.PostForm("ntfy_username")
		config["password"] = c.PostForm("ntfy_password")
		config["title_template"] = c.PostForm("ntfy_title_template")
		config["message_template"] = c.PostForm("ntfy_message_template")

		// Build event triggers
		if c.PostForm("trigger_job_start") == "on" {
			eventTriggers = append(eventTriggers, "job_start")
		}
		if c.PostForm("trigger_job_complete") == "on" {
			eventTriggers = append(eventTriggers, "job_complete")
		}
		if c.PostForm("trigger_job_error") == "on" {
			eventTriggers = append(eventTriggers, "job_error")
		}

		// Validate required fields
		if config["server"] == "" || config["topic"] == "" {
			h.handleNotificationsWithError(c, "Ntfy Server and Topic are required.")
			return
		}

		// Create new notification service
		service := db.NotificationService{
			Name:          name,
			Type:          serviceType,
			IsEnabled:     isEnabled,
			Config:        config,
			Description:   description,
			EventTriggers: eventTriggers,
			CreatedBy:     c.GetUint("userID"),
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
		h.handleNotificationsWithSuccess(c, "Ntfy notification service created successfully.")
		return

	case "gotify":
		config["url"] = c.PostForm("gotify_url")
		config["token"] = c.PostForm("gotify_token")
		config["priority"] = c.PostForm("gotify_priority")
		config["title_template"] = c.PostForm("gotify_title_template")
		config["message_template"] = c.PostForm("gotify_message_template")

		// Build event triggers
		if c.PostForm("trigger_job_start") == "on" {
			eventTriggers = append(eventTriggers, "job_start")
		}
		if c.PostForm("trigger_job_complete") == "on" {
			eventTriggers = append(eventTriggers, "job_complete")
		}
		if c.PostForm("trigger_job_error") == "on" {
			eventTriggers = append(eventTriggers, "job_error")
		}

		// Validate required fields
		if config["url"] == "" || config["token"] == "" {
			h.handleNotificationsWithError(c, "Gotify Server URL and Application Token are required.")
			return
		}

		// Create new notification service
		service := db.NotificationService{
			Name:          name,
			Type:          serviceType,
			IsEnabled:     isEnabled,
			Config:        config,
			Description:   description,
			EventTriggers: eventTriggers,
			CreatedBy:     c.GetUint("userID"),
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
		h.handleNotificationsWithSuccess(c, "Gotify notification service created successfully.")
		return

	case "pushover":
		config["app_token"] = c.PostForm("pushover_app_token")
		config["user_key"] = c.PostForm("pushover_user_key")
		config["device"] = c.PostForm("pushover_device")
		config["priority"] = c.PostForm("pushover_priority")
		config["sound"] = c.PostForm("pushover_sound")
		config["title_template"] = c.PostForm("pushover_title_template")
		config["message_template"] = c.PostForm("pushover_message_template")

		// Build event triggers
		if c.PostForm("trigger_job_start") == "on" {
			eventTriggers = append(eventTriggers, "job_start")
		}
		if c.PostForm("trigger_job_complete") == "on" {
			eventTriggers = append(eventTriggers, "job_complete")
		}
		if c.PostForm("trigger_job_error") == "on" {
			eventTriggers = append(eventTriggers, "job_error")
		}

		// Validate required fields
		if config["app_token"] == "" || config["user_key"] == "" {
			h.handleNotificationsWithError(c, "Pushover API Token and User Key are required.")
			return
		}

		// Create new notification service
		service := db.NotificationService{
			Name:          name,
			Type:          serviceType,
			IsEnabled:     isEnabled,
			Config:        config,
			Description:   description,
			EventTriggers: eventTriggers,
			CreatedBy:     c.GetUint("userID"),
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
		h.handleNotificationsWithSuccess(c, "Pushover notification service created successfully.")
		return

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
	// Check that the user has permissions to view settings
	if !h.checkPermission(c, "system.settings") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "You do not have permission to manage settings"})
		return
	}

	// Get notification service configuration
	serviceType := c.PostForm("type")
	config := make(map[string]string)

	// print the form data
	fmt.Println("serviceType", serviceType)

	// Process based on service type
	switch serviceType {
	case "email":
		config["smtp_host"] = c.PostForm("smtp_host")
		config["smtp_port"] = c.PostForm("smtp_port")
		config["smtp_username"] = c.PostForm("smtp_username")
		config["smtp_password"] = c.PostForm("smtp_password")
		config["from_email"] = c.PostForm("from_email")
		config["to_email"] = c.PostForm("to_email")
		config["use_tls"] = c.PostForm("use_tls")

		// Validate required fields
		if config["smtp_host"] == "" || config["smtp_port"] == "" || config["from_email"] == "" || config["to_email"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "SMTP Host, Port, From Email, and To Email are required"})
			return
		}

		// For email, we'll simply simulate a successful test
		// In a real implementation, you would want to actually try to send an email
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test email notification would be sent successfully"})
		return

	case "webhook":
		config["webhook_url"] = c.PostForm("webhook_url")
		config["webhook_method"] = c.PostForm("webhook_method")
		config["webhook_headers"] = c.PostForm("webhook_headers")
		config["webhook_body_template"] = c.PostForm("webhook_body_template")

		// Validate required fields
		if config["webhook_url"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Webhook URL is required"})
			return
		}

		// Send a test webhook
		// This is a simplified example - in a real implementation, you would want to use the templates
		// and properly format the request based on the configured method, headers, etc.
		resp, err := http.Post(config["webhook_url"], "application/json", bytes.NewBuffer([]byte(`{"message":"This is a test notification from GoMFT"}`)))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to send test webhook: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test webhook sent successfully"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Test webhook returned non-success status code: " + resp.Status})
		}
		return

	case "pushbullet":
		config["api_key"] = c.PostForm("pushbullet_api_key")
		config["device_iden"] = c.PostForm("pushbullet_device_iden")

		// Validate required fields
		if config["api_key"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Pushbullet API Key is required"})
			return
		}

		// Create a test push request
		pushURL := "https://api.pushbullet.com/v2/pushes"
		pushData := map[string]interface{}{
			"type":  "note",
			"title": "GoMFT Test Notification",
			"body":  "This is a test notification from GoMFT",
		}

		// Add device identifier if provided
		if config["device_iden"] != "" {
			pushData["device_iden"] = config["device_iden"]
		}

		jsonData, err := json.Marshal(pushData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to prepare test push: " + err.Error()})
			return
		}

		// Create the request
		req, err := http.NewRequest("POST", pushURL, bytes.NewBuffer(jsonData))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create request: " + err.Error()})
			return
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Access-Token", config["api_key"])

		// Send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to send test push: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Read response body for error details if needed
		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test push sent successfully"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Test push returned error: " + resp.Status + " - " + string(respBody)})
		}
		return

	case "ntfy":
		config["topic"] = c.PostForm("ntfy_topic")
		config["server"] = c.PostForm("ntfy_server")
		config["priority"] = c.PostForm("ntfy_priority")
		config["title"] = c.PostForm("ntfy_title")

		// Validate required fields
		if config["topic"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Ntfy Topic is required"})
			return
		}

		// Set default server if not provided
		if config["server"] == "" {
			config["server"] = "https://ntfy.sh"
		}

		// Create the URL for the notification
		ntfyURL := fmt.Sprintf("%s/%s", strings.TrimRight(config["server"], "/"), config["topic"])

		// Create the notification data
		ntfyData := map[string]interface{}{
			"topic":   config["topic"],
			"title":   "GoMFT Test Notification",
			"message": "This is a test notification from GoMFT",
		}

		// Add priority if provided
		if config["priority"] != "" {
			ntfyData["priority"] = config["priority"]
		}

		// Add title if provided
		if config["title"] != "" {
			ntfyData["title"] = config["title"]
		}

		jsonData, err := json.Marshal(ntfyData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to prepare test notification: " + err.Error()})
			return
		}

		// Send the notification
		resp, err := http.Post(ntfyURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to send test notification: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test notification sent successfully"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Test notification returned non-success status code: " + resp.Status})
		}
		return

	case "gotify":
		config["url"] = c.PostForm("gotify_url")
		config["token"] = c.PostForm("gotify_token")
		config["priority"] = c.PostForm("gotify_priority")
		config["title"] = c.PostForm("gotify_title_template")

		fmt.Println("config", config)

		// Validate required fields
		if config["url"] == "" || config["token"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Gotify Server URL and Application Token are required"})
			return
		}

		// Create the URL for the notification
		gotifyURL := fmt.Sprintf("%s/message", strings.TrimRight(config["url"], "/"))

		// Set default values for any missing fields
		title := "GoMFT Test Notification"
		if config["title"] != "" {
			title = config["title"]
		}

		priority := 5 // Default priority
		if config["priority"] != "" {
			priorityInt, err := strconv.Atoi(config["priority"])
			if err == nil {
				priority = priorityInt
			}
		}

		// Create the notification data
		gotifyData := map[string]interface{}{
			"title":    title,
			"message":  "This is a test notification from GoMFT",
			"priority": priority,
		}

		jsonData, err := json.Marshal(gotifyData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to prepare test notification: " + err.Error()})
			return
		}

		// Create the request
		req, err := http.NewRequest("POST", gotifyURL, bytes.NewBuffer(jsonData))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create request: " + err.Error()})
			return
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gotify-Key", config["token"])

		// Send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to send test notification: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Read response body for error details if needed
		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test notification sent successfully"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Test notification returned error: " + resp.Status + " - " + string(respBody)})
		}
		return

	case "pushover":
		config["app_token"] = c.PostForm("pushover_app_token")
		config["user_key"] = c.PostForm("pushover_user_key")
		config["device"] = c.PostForm("pushover_device")
		config["priority"] = c.PostForm("pushover_priority")
		config["sound"] = c.PostForm("pushover_sound")

		// Validate required fields
		if config["app_token"] == "" || config["user_key"] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Pushover API Token and User Key are required"})
			return
		}

		// Create the URL for the notification
		pushoverURL := "https://api.pushover.net/1/messages.json"

		// Create the form data for Pushover
		formData := url.Values{}
		formData.Set("token", config["app_token"])
		formData.Set("user", config["user_key"])
		formData.Set("title", "GoMFT Test Notification")
		formData.Set("message", "This is a test notification from GoMFT")

		// Add optional fields if provided
		if config["device"] != "" {
			formData.Set("device", config["device"])
		}
		if config["priority"] != "" {
			formData.Set("priority", config["priority"])
		}
		if config["sound"] != "" {
			formData.Set("sound", config["sound"])
		}

		// Send the request
		resp, err := http.PostForm(pushoverURL, formData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to send test notification: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Read response body for error details if needed
		respBody, _ := io.ReadAll(resp.Body)
		var pushoverResp map[string]interface{}
		json.Unmarshal(respBody, &pushoverResp)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 && pushoverResp["status"].(float64) == 1 {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test notification sent successfully"})
		} else {
			errorMsg := "Test notification returned error"
			if errStr, ok := pushoverResp["errors"]; ok {
				errorMsg = errorMsg + ": " + fmt.Sprintf("%v", errStr)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": errorMsg})
		}
		return

	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Unknown service type: " + serviceType})
	}
}

// replacePlaceholders replaces placeholders in templates with sample values
func replacePlaceholders(template string) string {
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

	result := template
	for placeholder, value := range replacements {
		result = strings.Replace(result, placeholder, value, -1)
	}

	return result
}

// sendTestPushbulletNotification sends a test Pushbullet notification
func sendTestPushbulletNotification(apiKey, deviceIden, title, body string) error {
	const pushbulletAPI = "https://api.pushbullet.com/v2/pushes"

	// Create payload
	payload := map[string]interface{}{
		"type":  "note",
		"title": title,
		"body":  body,
	}

	// Add device_iden if provided
	if deviceIden != "" {
		payload["device_iden"] = deviceIden
	}

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creating JSON payload: %v", err)
	}

	// Create request
	req, err := http.NewRequest("POST", pushbulletAPI, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Token", apiKey)

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushbullet API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// sendTestNtfyNotification sends a test Ntfy notification
func sendTestNtfyNotification(server, topic, title, message string, priority int, username, password string) error {
	// Ensure server doesn't end with a slash
	server = strings.TrimSuffix(server, "/")

	// Build URL
	ntfyURL := fmt.Sprintf("%s/%s", server, topic)

	// Create request
	req, err := http.NewRequest("POST", ntfyURL, bytes.NewBufferString(message))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Title", title)
	req.Header.Set("Priority", strconv.Itoa(priority))

	// Set authentication if provided
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ntfy API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
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
	var componentServices []types.NotificationServiceData // Use types.NotificationServiceData
	for _, service := range notificationServices {
		componentServices = append(componentServices, types.NotificationServiceData{ // Use types.NotificationServiceData
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

	data := types.SettingsNotificationsData{ // Use types.SettingsNotificationsData
		NotificationServices: componentServices,
	}

	ctx := h.CreateTemplateContext(c)
	list.List(ctx, data).Render(ctx, c.Writer) // Use list.List
}

// HandleNewNotificationPage handles GET /admin/settings/notifications/new
func (h *Handlers) HandleNewNotificationPage(c *gin.Context) {
	// Check if the user has permission to view settings
	if !h.checkPermission(c, "system.settings") {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	data := types.NotificationFormData{ // Use types.NotificationFormData
		IsNew: true,
		NotificationService: &struct {
			ID                      uint
			Name                    string
			Description             string
			Type                    string
			IsEnabled               bool
			EventTriggers           []string
			RetryPolicy             string
			WebhookURL              string
			Method                  string
			Headers                 string
			PayloadTemplate         string
			SecretKey               string
			PushbulletAPIKey        string
			PushbulletDeviceID      string
			PushbulletTitleTemplate string
			PushbulletBodyTemplate  string
			NtfyServer              string
			NtfyTopic               string
			NtfyPriority            string
			NtfyUsername            string
			NtfyPassword            string
			NtfyTitleTemplate       string
			NtfyMessageTemplate     string
			GotifyURL               string
			GotifyToken             string
			GotifyPriority          string
			GotifyTitleTemplate     string
			GotifyMessageTemplate   string
			PushoverAPIToken        string
			PushoverUserKey         string
			PushoverDevice          string
			PushoverPriority        string
			PushoverSound           string
			PushoverTitleTemplate   string
			PushoverMessageTemplate string
		}{},
	}

	ctx := h.CreateTemplateContext(c)
	form.NotificationForm(ctx, data).Render(ctx, c.Writer) // Use form.NotificationForm
}

// HandleEditNotificationPage handles GET /admin/settings/notifications/:id/edit
func (h *Handlers) HandleEditNotificationPage(c *gin.Context) {
	// Check if the user has permission to view settings
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

	// Find the notification service
	var service db.NotificationService
	if err := h.DB.First(&service, serviceID).Error; err != nil {
		h.handleNotificationsWithError(c, "Notification service not found.")
		return
	}

	// Convert to NotificationFormData
	data := types.NotificationFormData{ // Use types.NotificationFormData
		IsNew: false,
		NotificationService: &struct {
			ID                      uint
			Name                    string
			Description             string
			Type                    string
			IsEnabled               bool
			EventTriggers           []string
			RetryPolicy             string
			WebhookURL              string
			Method                  string
			Headers                 string
			PayloadTemplate         string
			SecretKey               string
			PushbulletAPIKey        string
			PushbulletDeviceID      string
			PushbulletTitleTemplate string
			PushbulletBodyTemplate  string
			NtfyServer              string
			NtfyTopic               string
			NtfyPriority            string
			NtfyUsername            string
			NtfyPassword            string
			NtfyTitleTemplate       string
			NtfyMessageTemplate     string
			GotifyURL               string
			GotifyToken             string
			GotifyPriority          string
			GotifyTitleTemplate     string
			GotifyMessageTemplate   string
			PushoverAPIToken        string
			PushoverUserKey         string
			PushoverDevice          string
			PushoverPriority        string
			PushoverSound           string
			PushoverTitleTemplate   string
			PushoverMessageTemplate string
		}{
			ID:                      service.ID,
			Name:                    service.Name,
			Description:             service.Description,
			Type:                    service.Type,
			IsEnabled:               service.IsEnabled,
			EventTriggers:           service.EventTriggers,
			RetryPolicy:             service.RetryPolicy,
			PayloadTemplate:         service.PayloadTemplate,
			SecretKey:               service.SecretKey,
			WebhookURL:              service.Config["webhook_url"],
			Method:                  service.Config["method"],
			Headers:                 service.Config["headers"],
			PushbulletAPIKey:        service.Config["api_key"],
			PushbulletDeviceID:      service.Config["device_iden"],
			PushbulletTitleTemplate: service.Config["title_template"],
			PushbulletBodyTemplate:  service.Config["body_template"],
			NtfyServer:              service.Config["server"],
			NtfyTopic:               service.Config["topic"],
			NtfyPriority:            service.Config["priority"],
			NtfyUsername:            service.Config["username"],
			NtfyPassword:            service.Config["password"],
			NtfyTitleTemplate:       service.Config["title_template"],
			NtfyMessageTemplate:     service.Config["message_template"],
			GotifyURL:               service.Config["url"],
			GotifyToken:             service.Config["token"],
			GotifyPriority:          service.Config["priority"],
			GotifyTitleTemplate:     service.Config["title_template"],
			GotifyMessageTemplate:   service.Config["message_template"],
			PushoverAPIToken:        service.Config["app_token"],
			PushoverUserKey:         service.Config["user_key"],
			PushoverDevice:          service.Config["device"],
			PushoverPriority:        service.Config["priority"],
			PushoverSound:           service.Config["sound"],
			PushoverTitleTemplate:   service.Config["title_template"],
			PushoverMessageTemplate: service.Config["message_template"],
		},
	}

	// Add type-specific fields
	if service.Type == "webhook" {
		data.NotificationService.WebhookURL = service.Config["webhook_url"]
		data.NotificationService.Method = service.Config["method"]
		data.NotificationService.Headers = service.Config["headers"]
	}

	ctx := h.CreateTemplateContext(c)
	form.NotificationForm(ctx, data).Render(ctx, c.Writer) // Use form.NotificationForm
}

// HandleUpdateNotificationService handles PUT /admin/settings/notifications/:id
func (h *Handlers) HandleUpdateNotificationService(c *gin.Context) {
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

	// Find the notification service
	var service db.NotificationService
	if err := h.DB.First(&service, serviceID).Error; err != nil {
		h.handleNotificationsWithError(c, "Notification service not found.")
		return
	}

	// Parse form data
	name := c.PostForm("name")
	serviceType := c.PostForm("type")
	description := c.PostForm("description")
	isEnabled := c.PostForm("is_enabled") == "on"

	// Validate required fields
	if name == "" || serviceType == "" {
		h.handleNotificationsWithError(c, "Name and type are required fields.")
		return
	}

	// Update basic fields
	service.Name = name
	service.Type = serviceType
	service.Description = description
	service.IsEnabled = isEnabled

	// Update type-specific fields based on service type
	switch serviceType {
	case "webhook":
		// Update webhook-specific fields
		service.Config["webhook_url"] = c.PostForm("webhook_url")
		service.Config["method"] = c.PostForm("method")
		service.Config["headers"] = c.PostForm("headers")
		service.PayloadTemplate = c.PostForm("payload_template")
		service.SecretKey = c.PostForm("secret_key")
		service.RetryPolicy = c.PostForm("retry_policy")

		// Update event triggers
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
		service.EventTriggers = eventTriggers

	case "pushbullet":
		// Update Pushbullet-specific fields
		service.Config["api_key"] = c.PostForm("pushbullet_api_key")
		service.Config["device_iden"] = c.PostForm("pushbullet_device_iden")
		service.Config["title_template"] = c.PostForm("pushbullet_title_template")
		service.Config["body_template"] = c.PostForm("pushbullet_body_template")

		// Update event triggers
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
		service.EventTriggers = eventTriggers

	case "ntfy":
		// Update Ntfy-specific fields
		service.Config["server"] = c.PostForm("ntfy_server")
		service.Config["topic"] = c.PostForm("ntfy_topic")
		service.Config["priority"] = c.PostForm("ntfy_priority")
		service.Config["username"] = c.PostForm("ntfy_username")
		service.Config["password"] = c.PostForm("ntfy_password")
		service.Config["title_template"] = c.PostForm("ntfy_title_template")
		service.Config["message_template"] = c.PostForm("ntfy_message_template")

		// Update event triggers
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
		service.EventTriggers = eventTriggers

	case "gotify":
		// Update Gotify-specific fields
		service.Config["url"] = c.PostForm("gotify_url")
		service.Config["token"] = c.PostForm("gotify_token")
		service.Config["priority"] = c.PostForm("gotify_priority")
		service.Config["title_template"] = c.PostForm("gotify_title_template")
		service.Config["message_template"] = c.PostForm("gotify_message_template")

		// Update event triggers
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
		service.EventTriggers = eventTriggers

	case "pushover":
		// Update Pushover-specific fields
		service.Config["app_token"] = c.PostForm("pushover_app_token")
		service.Config["user_key"] = c.PostForm("pushover_user_key")
		service.Config["device"] = c.PostForm("pushover_device")
		service.Config["priority"] = c.PostForm("pushover_priority")
		service.Config["sound"] = c.PostForm("pushover_sound")
		service.Config["title_template"] = c.PostForm("pushover_title_template")
		service.Config["message_template"] = c.PostForm("pushover_message_template")

		// Update event triggers
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
		service.EventTriggers = eventTriggers
	}

	// Save to database
	if err := h.DB.Save(&service).Error; err != nil {
		log.Printf("Error updating notification service: %v", err)
		h.handleNotificationsWithError(c, "Failed to update notification service: "+err.Error())
		return
	}

	// Create audit log
	auditDetails := map[string]interface{}{
		"name":           service.Name,
		"type":           service.Type,
		"is_enabled":     service.IsEnabled,
		"description":    service.Description,
		"event_triggers": service.EventTriggers,
	}

	auditLog := db.AuditLog{
		Action:     "update",
		EntityType: "notification_service",
		EntityID:   service.ID,
		UserID:     c.GetUint("userID"),
		Details:    auditDetails,
	}

	if err := h.DB.Create(&auditLog).Error; err != nil {
		log.Printf("Error creating audit log: %v", err)
	}

	// Redirect back to notifications page with success message
	h.handleNotificationsWithSuccess(c, "Notification service updated successfully.")
}

// handleNotificationsWithError renders the notifications page with an error message
func (h *Handlers) handleNotificationsWithError(c *gin.Context, errorMessage string) {
	var notificationServices []db.NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []types.NotificationServiceData // Use types.NotificationServiceData
	for _, service := range notificationServices {
		componentServices = append(componentServices, types.NotificationServiceData{ // Use types.NotificationServiceData
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

	data := types.SettingsNotificationsData{ // Use types.SettingsNotificationsData
		NotificationServices: componentServices,
		ErrorMessage:         errorMessage,
	}

	ctx := h.CreateTemplateContext(c)
	list.List(ctx, data).Render(ctx, c.Writer) // Use list.List
}

// handleNotificationsWithSuccess renders the notifications page with a success message
func (h *Handlers) handleNotificationsWithSuccess(c *gin.Context, successMessage string) {
	var notificationServices []db.NotificationService
	if err := h.DB.Find(&notificationServices).Error; err != nil {
		log.Printf("Error fetching notification services: %v", err)
	}

	// Convert to components.NotificationService
	var componentServices []types.NotificationServiceData // Use types.NotificationServiceData
	for _, service := range notificationServices {
		componentServices = append(componentServices, types.NotificationServiceData{ // Use types.NotificationServiceData
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

	data := types.SettingsNotificationsData{ // Use types.SettingsNotificationsData
		NotificationServices: componentServices,
		SuccessMessage:       successMessage,
	}

	ctx := h.CreateTemplateContext(c)
	list.List(ctx, data).Render(ctx, c.Writer) // Use list.List
}
