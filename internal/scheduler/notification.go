package scheduler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"gorm.io/gorm" // Needed for the Create method signature in the interface
)

// NotificationDB defines the database methods needed by Notifier.
type NotificationDB interface {
	GetNotificationServices(enabledOnly bool) ([]db.NotificationService, error)
	UpdateNotificationService(service *db.NotificationService) error
	GetJob(jobID uint) (*db.Job, error)
	CreateJobNotification(userID uint, jobID uint, historyID uint, notificationType db.NotificationType, title string, message string) error
	Create(value interface{}) *gorm.DB // Used by createJobHistoryAndNotify
}

// Notifier handles sending notifications via various services.
type Notifier struct {
	db            NotificationDB // Use the interface type
	logger        *Logger
	skipSSLVerify bool
}

// NewNotifier creates a new Notifier.
func NewNotifier(database NotificationDB, logger *Logger, skipSSLVerify bool) *Notifier { // Accept the interface type and skip flag
	return &Notifier{
		db:            database,
		logger:        logger,
		skipSSLVerify: skipSSLVerify, // Store the flag
	}
}

// SendNotifications is the main entry point for sending notifications for a job execution step.
// It handles job-specific webhooks and triggers global notifications.
func (n *Notifier) SendNotifications(job *db.Job, history *db.JobHistory, config *db.TransferConfig) {
	// First, handle job-specific webhook if configured
	if job.GetWebhookEnabled() && job.WebhookURL != "" {
		// Skip notifications based on settings
		if history.Status == "completed" && !job.GetNotifyOnSuccess() {
			n.logger.LogDebug("Skipping success notification for job %d (notifyOnSuccess=false)", job.ID)
		} else if history.Status == "failed" && !job.GetNotifyOnFailure() {
			n.logger.LogDebug("Skipping failure notification for job %d (notifyOnFailure=false)", job.ID)
		} else {
			n.logger.LogInfo("Sending job-specific webhook notification for job %d", job.ID)
			n.sendJobWebhookNotification(job, history, config) // Call the job-specific sender
		}
	}

	// Next, process global notification services
	n.sendGlobalNotifications(job, history, config)
}

// sendJobWebhookNotification sends a notification to the job's configured webhook URL.
func (n *Notifier) sendJobWebhookNotification(job *db.Job, history *db.JobHistory, config *db.TransferConfig) {
	// Create the payload with useful information
	payload := map[string]interface{}{
		"event_type":        "job_execution",
		"job_id":            job.ID,
		"job_name":          job.Name,
		"config_id":         config.ID,
		"config_name":       config.Name,
		"status":            history.Status,
		"start_time":        history.StartTime.Format(time.RFC3339),
		"history_id":        history.ID,
		"bytes_transferred": history.BytesTransferred,
		"files_transferred": history.FilesTransferred,
	}

	if history.EndTime != nil {
		payload["end_time"] = history.EndTime.Format(time.RFC3339)
		duration := history.EndTime.Sub(history.StartTime)
		payload["duration_seconds"] = duration.Seconds()
	}

	if history.ErrorMessage != "" {
		payload["error_message"] = history.ErrorMessage
	}

	// Add source and destination information
	payload["source"] = map[string]string{
		"type": config.SourceType,
		"path": config.SourcePath,
	}
	payload["destination"] = map[string]string{
		"type": config.DestinationType,
		"path": config.DestinationPath,
	}

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		n.logger.LogError("Error marshaling webhook payload for job %d: %v", job.ID, err)
		return
	}

	n.logger.LogDebug("Webhook payload: %s", string(jsonPayload))

	// Create HTTP request
	req, err := http.NewRequest("POST", job.WebhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		n.logger.LogError("Error creating webhook request for job %d: %v", job.ID, err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoMFT-Webhook/1.0")

	// Add X-Hub-Signature if secret is configured
	if job.WebhookSecret != "" {
		h := hmac.New(sha256.New, []byte(job.WebhookSecret))
		h.Write(jsonPayload)
		signature := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", signature)
	}

	// Add custom headers if specified
	if job.WebhookHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(job.WebhookHeaders), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	n.logger.LogDebug("Webhook headers: %+v", req.Header)

	// Send the request with a timeout and configured TLS settings
	client := createHTTPClient(10*time.Second, n.skipSSLVerify)
	resp, err := client.Do(req)
	if err != nil {
		n.logger.LogError("Error sending webhook for job %d: %v", job.ID, err)
		return
	}
	defer resp.Body.Close()

	// Log the response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		n.logger.LogInfo("Webhook notification for job %d sent successfully (status: %d)", job.ID, resp.StatusCode)
	} else {
		n.logger.LogError("Webhook notification for job %d failed with status: %d", job.ID, resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		if len(respBody) > 0 {
			n.logger.LogDebug("Webhook response: %s", respBody)
		}
	}
}

// sendGlobalNotifications sends notifications through all configured notification services.
func (n *Notifier) sendGlobalNotifications(job *db.Job, history *db.JobHistory, config *db.TransferConfig) {
	// Fetch all enabled notification services
	services, err := n.db.GetNotificationServices(true) // Calls interface method
	if err != nil {
		n.logger.LogError("Error fetching notification services: %v", err)
		return
	}

	if len(services) == 0 {
		n.logger.LogDebug("No enabled notification services found")
		return
	}

	n.logger.LogInfo("Found %d enabled notification services", len(services))

	// Determine event type based on job status
	var eventType string
	switch history.Status {
	case "running":
		eventType = "job_start"
	case "completed", "completed_with_errors":
		eventType = "job_complete"
	case "failed":
		eventType = "job_error"
	default:
		eventType = "job_status"
	}

	// Process each notification service
	for i := range services {
		service := &services[i] // Use pointer to update stats
		n.logger.LogInfo("Processing notification service %s (%s)", service.Name, service.Type)
		// Check if this service should handle this event type
		shouldSend := false
		for _, trigger := range service.EventTriggers {
			if trigger == eventType {
				shouldSend = true
				break
			}
		}

		// Skip if this service doesn't handle this event type
		if !shouldSend {
			n.logger.LogDebug("Skipping notification service %s (%s) for event %s (not in triggers)",
				service.Name, service.Type, eventType)
			continue
		}

		// --- Moved Update Logic Inside this block ---
		n.logger.LogInfo("Sending notification via service %s (%s) for job %d",
			service.Name, service.Type, job.ID)

		// Send notification based on service type
		var notifyErr error
		switch service.Type {
		case "email":
			notifyErr = n.sendEmailNotification(service, job, history, config, eventType)
		case "webhook":
			notifyErr = n.sendServiceWebhookNotification(service, job, history, config, eventType)
		case "pushbullet":
			notifyErr = n.sendPushbulletNotification(service, job, history, config, eventType)
		case "ntfy":
			notifyErr = n.sendNtfyNotification(service, job, history, config, eventType)
		case "gotify":
			notifyErr = n.sendGotifyNotification(service, job, history, config, eventType)
		case "pushover":
			notifyErr = n.sendPushoverNotification(service, job, history, config, eventType)
		default:
			n.logger.LogError("Unsupported notification service type: %s", service.Type)
			// Skip update logic below if type is unsupported
			continue
		}

		// Update service success/failure count
		if notifyErr != nil {
			service.FailureCount++
			n.logger.LogError("Notification service %s failed: %v", service.Name, notifyErr)
		} else {
			service.SuccessCount++
			service.LastUsed = time.Now()
			n.logger.LogInfo("Notification service %s sent successfully", service.Name)
		}

		// Update notification service stats in the database
		if err := n.db.UpdateNotificationService(service); err != nil { // Calls interface method
			n.logger.LogError("Error updating notification service stats for service %s: %v", service.Name, err)
		}
		// --- End of Moved Update Logic ---

	} // End of loop through services
}

// sendEmailNotification sends an email notification using the configured email service.
func (n *Notifier) sendEmailNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	n.logger.LogDebug("Preparing email notification via service %s for job %d", service.Name, job.ID)

	// Extract SMTP settings from service config
	smtpHost := service.Config["smtp_host"]
	smtpPortStr := service.Config["smtp_port"]
	fromEmail := service.Config["from_email"]
	toEmail := service.Config["to_email"]

	// Validate required settings
	if smtpHost == "" || smtpPortStr == "" || fromEmail == "" || toEmail == "" {
		return fmt.Errorf("missing required SMTP settings")
	}

	// Parse SMTP port
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %v", err)
	}

	// Prepare email content
	subject := fmt.Sprintf("[GoMFT] Job %s: %s", job.Name, history.Status)
	body := generateEmailBody(job, history, config, eventType) // Use package-level helper

	// TODO: Implement actual email sending logic
	// This would typically involve using a package like "net/smtp" or a third-party
	// email library to send the actual email.
	// For actual implementation, you would use:
	// - smtpUsername := service.Config["smtp_username"]
	// - smtpPassword := service.Config["smtp_password"]

	n.logger.LogInfo("Email would be sent to %s with subject: %s", toEmail, subject)
	n.logger.LogDebug("Email body: %s", body)

	// Placeholder for actual email sending
	// For now, we'll just log that the email would be sent
	n.logger.LogInfo("Email notification prepared (SMTP: %s:%d, From: %s, To: %s)",
		smtpHost, smtpPort, fromEmail, toEmail)

	return nil
}

// generateEmailBody creates the email body for job notifications.
// (Remains package-level helper)
func generateEmailBody(job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Job: %s (ID: %d)\n", job.Name, job.ID))
	b.WriteString(fmt.Sprintf("Status: %s\n", history.Status))
	b.WriteString(fmt.Sprintf("Start Time: %s\n", history.StartTime.Format(time.RFC3339)))

	if history.EndTime != nil {
		b.WriteString(fmt.Sprintf("End Time: %s\n", history.EndTime.Format(time.RFC3339)))
		duration := history.EndTime.Sub(history.StartTime)
		b.WriteString(fmt.Sprintf("Duration: %.2f seconds\n", duration.Seconds()))
	}

	b.WriteString(fmt.Sprintf("Files Transferred: %d\n", history.FilesTransferred))
	b.WriteString(fmt.Sprintf("Bytes Transferred: %d\n", history.BytesTransferred))

	b.WriteString("\nTransfer Configuration:\n")
	b.WriteString(fmt.Sprintf("Name: %s (ID: %d)\n", config.Name, config.ID))
	b.WriteString(fmt.Sprintf("Source: %s:%s\n", config.SourceType, config.SourcePath))
	b.WriteString(fmt.Sprintf("Destination: %s:%s\n", config.DestinationType, config.DestinationPath))

	if history.ErrorMessage != "" {
		b.WriteString("\nError Details:\n")
		b.WriteString(history.ErrorMessage)
	}

	return b.String()
}

// createHTTPClient creates an HTTP client with appropriate TLS settings and timeout.
func createHTTPClient(timeout time.Duration, skipSSLVerify bool) *http.Client {
	// Clone the default transport to avoid modifying global state
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if skipSSLVerify {
		// Ensure TLSClientConfig exists before modifying it
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
		// TODO: Consider adding a log warning here when skipping verification
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// sendServiceWebhookNotification sends a webhook notification using a configured notification service.
func (n *Notifier) sendServiceWebhookNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	n.logger.LogDebug("Preparing webhook notification via service %s for job %d", service.Name, job.ID)

	// Extract webhook settings
	webhookURL := service.Config["webhook_url"]
	method := service.Config["method"]
	if method == "" {
		method = "POST" // Default to POST if not specified
	}

	// Validate required settings
	if webhookURL == "" {
		return fmt.Errorf("missing webhook URL")
	}

	// Prepare payload
	var payload map[string]interface{}

	// Use custom payload template if provided
	if service.PayloadTemplate != "" {
		// Parse the template and fill in variables
		payload = generateCustomPayload(service.PayloadTemplate, job, history, config, eventType) // Use package-level helper
	} else {
		// Use default payload format
		payload = generateDefaultPayload(job, history, config, eventType) // Use package-level helper
	}

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling webhook payload: %v", err)
	}

	n.logger.LogDebug("Webhook payload: %s", string(jsonPayload))

	// Create HTTP request
	req, err := http.NewRequest(method, webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error creating webhook request: %v", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoMFT-Notification/1.0")

	// Add signature if secret key is provided
	if service.SecretKey != "" {
		h := hmac.New(sha256.New, []byte(service.SecretKey))
		h.Write(jsonPayload)
		signature := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-GoMFT-Signature", signature)
	}

	// Add custom headers if specified
	if headersStr := service.Config["headers"]; headersStr != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersStr), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	n.logger.LogDebug("Webhook headers: %+v", req.Header)

	// Determine timeout based on retry policy
	timeout := 10 * time.Second
	maxRetries := 0

	switch service.RetryPolicy {
	case "none":
		maxRetries = 0
	case "simple":
		maxRetries = 3
		timeout = 15 * time.Second
	case "exponential":
		maxRetries = 5
		timeout = 30 * time.Second
	default:
		// Default to simple
		maxRetries = 3
		timeout = 15 * time.Second
	}

	// Prepare client with timeout and configured TLS settings
	client := createHTTPClient(timeout, n.skipSSLVerify)

	// Attempt to send with retries
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with increasing backoff
			backoffDuration := time.Duration(1<<uint(attempt-1)) * time.Second
			n.logger.LogInfo("Retrying webhook notification (attempt %d/%d) after %v",
				attempt, maxRetries, backoffDuration)
			time.Sleep(backoffDuration)
		}

		resp, err = client.Do(req)
		if err == nil {
			// Check for success status code
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				defer resp.Body.Close()
				n.logger.LogInfo("Webhook notification sent successfully (status: %d)", resp.StatusCode)
				return nil
			}

			// Error status code
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, respBody)
			n.logger.LogError("Webhook error (attempt %d/%d): %v", attempt+1, maxRetries+1, lastErr)
		} else {
			// Network or request error
			lastErr = fmt.Errorf("webhook request failed: %v", err)
			n.logger.LogError("Webhook request error (attempt %d/%d): %v", attempt+1, maxRetries+1, lastErr)
		}
	}

	return lastErr
}

// generateDefaultPayload creates a standard webhook payload.
// (Remains package-level helper)
func generateDefaultPayload(job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) map[string]interface{} {
	// get event type
	switch eventType {
	case "job_start":
		eventType = "Job Started"
	case "job_complete":
		eventType = "Job Completed"
	case "job_fail":
		eventType = "Job Failed"
	}

	payload := map[string]interface{}{
		"event": eventType,
		"job": map[string]interface{}{
			"id":             job.ID,
			"name":           job.Name,
			"status":         history.Status,
			"event":          eventType,
			"message":        history.ErrorMessage,
			"started_at":     history.StartTime.Format(time.RFC3339),
			"config_id":      config.ID,
			"config_name":    config.Name,
			"transfer_bytes": history.BytesTransferred,
			"file_count":     history.FilesTransferred,
		},
		"instance": map[string]interface{}{
			"id":          "gomft",
			"name":        "GoMFT",
			"version":     "1.0",        // TODO: Get actual version
			"environment": "production", // TODO: Get from env
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if history.EndTime != nil {
		payload["job"].(map[string]interface{})["completed_at"] = history.EndTime.Format(time.RFC3339)
		duration := history.EndTime.Sub(history.StartTime)
		payload["job"].(map[string]interface{})["duration_seconds"] = duration.Seconds()
	}

	return payload
}

// generateCustomPayload creates a webhook payload from a template.
// (Remains package-level helper)
func generateCustomPayload(template string, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) map[string]interface{} {
	// Start with the default payload as a base
	defaultPayload := generateDefaultPayload(job, history, config, eventType) // Use package-level helper

	// Parse the template string to JSON
	var customPayload map[string]interface{}
	if err := json.Unmarshal([]byte(template), &customPayload); err != nil {
		// If template can't be parsed, fall back to default payload
		return defaultPayload
	}

	// Replace variables in the template
	// This is a simplified version - a real implementation would do deep traversal
	// and replace all variables in the structure
	processedPayload := processPayloadVariables(customPayload, defaultPayload) // Use package-level helper

	return processedPayload
}

// processPayloadVariables recursively processes a payload structure and replaces variables.
// (Remains package-level helper)
func processPayloadVariables(customPayload map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Process each key-value pair in the custom payload
	for key, value := range customPayload {
		switch v := value.(type) {
		case string:
			// Replace string variables
			result[key] = replaceVariables(v, variables) // Use package-level helper
		case map[string]interface{}:
			// Recursively process nested maps
			result[key] = processPayloadVariables(v, variables) // Recursive call
		case []interface{}:
			// Process arrays
			result[key] = processArrayVariables(v, variables) // Use package-level helper
		default:
			// Keep other types as is
			result[key] = value
		}
	}

	return result
}

// processArrayVariables processes array elements for variable replacement.
// (Remains package-level helper)
func processArrayVariables(array []interface{}, variables map[string]interface{}) []interface{} {
	result := make([]interface{}, len(array))

	for i, value := range array {
		switch v := value.(type) {
		case string:
			result[i] = replaceVariables(v, variables) // Use package-level helper
		case map[string]interface{}:
			result[i] = processPayloadVariables(v, variables) // Recursive call
		case []interface{}:
			result[i] = processArrayVariables(v, variables) // Recursive call
		default:
			result[i] = value
		}
	}

	return result
}

// replaceVariables replaces variable placeholders in a string with their values.
// (Remains package-level helper)
func replaceVariables(template string, variables map[string]interface{}) string {
	// Check for variable pattern like {{job.name}}
	re := regexp.MustCompile(`{{([^{}]+)}}`)
	result := re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable path (e.g., "job.name")
		varPath := re.FindStringSubmatch(match)[1]
		parts := strings.Split(varPath, ".")

		// Navigate the variables structure to find the value
		var current interface{} = variables
		for _, part := range parts {
			if m, ok := current.(map[string]interface{}); ok {
				if val, exists := m[part]; exists {
					current = val
				} else {
					return match // Keep original if not found
				}
			} else {
				return match // Keep original if structure doesn't match
			}
		}

		// Convert the found value to string
		switch v := current.(type) {
		case string:
			return v
		case int, int64, uint, uint64, float32, float64:
			return fmt.Sprintf("%v", v)
		case bool:
			return fmt.Sprintf("%v", v)
		case time.Time:
			return v.Format(time.RFC3339)
		default:
			// For complex types, convert to JSON
			if bytes, err := json.Marshal(v); err == nil {
				return string(bytes)
			}
			return match
		}
	})

	return result
}

// updateJobStatus creates notifications based on job status changes.
// Note: The original function updated history, this one focuses on notification creation.
// TODO: This function seems redundant with createJobNotification, review and potentially remove.
func (n *Notifier) updateJobStatus(jobID uint, status string, startTime, endTime time.Time, message string) (*db.JobHistory, error) {
	// Create a temporary history object just for notification context
	history := &db.JobHistory{
		JobID:        jobID,
		Status:       status,
		StartTime:    startTime,
		EndTime:      &endTime,
		ErrorMessage: message,
	}

	// Get job details to find the creator for notification targeting
	job, err := n.db.GetJob(jobID) // Calls interface method
	if err != nil {
		// Log error but don't necessarily fail the whole operation if job fetch fails
		n.logger.LogError("Failed to get job details for notification: jobID=%d, error=%v", jobID, err)
		// Return the temporary history object and nil error, as notification is best-effort
		return history, nil
	}

	// Get the user who created the job
	userID := job.CreatedBy

	// Create job title from job name or ID
	jobTitle := job.Name
	if jobTitle == "" {
		jobTitle = fmt.Sprintf("Job #%d", job.ID)
	}

	// Check which notification to send based on status
	var notificationType db.NotificationType
	var title string
	var notificationMessage string // Renamed from message to avoid conflict

	switch status {
	case "running":
		notificationType = db.NotificationJobStart
		title = "Job Started"
		notificationMessage = jobTitle
	case "completed":
		notificationType = db.NotificationJobComplete
		title = "Job Complete"
		notificationMessage = jobTitle
	case "failed":
		notificationType = db.NotificationJobFail
		title = "Job Failed"
		notificationMessage = jobTitle
		if history.ErrorMessage != "" {
			notificationMessage = jobTitle + ": " + history.ErrorMessage
		}
	default:
		// Don't create notifications for other statuses
		return history, nil
	}

	// Create the notification in the database
	// Assuming history.ID is set elsewhere if needed, or pass 0 if not applicable here
	err = n.db.CreateJobNotification( // Calls interface method
		userID,
		jobID,
		0, // History ID might not be available/relevant here, pass 0 or adjust DB function
		notificationType,
		title,
		notificationMessage,
	)

	if err != nil {
		n.logger.LogError("Failed to create job notification: jobID=%d, error=%v", job.ID, err)
		// Continue anyway, not critical
	}

	// Return the temporary history object and nil error
	return history, nil
}

// Create a job history record and send notification
// TODO: This function mixes history creation and notification.
// History creation should likely happen elsewhere (e.g., JobExecutor).
// This method should perhaps just focus on sending notifications based on a history record.
// Renaming to sendNotificationsForHistory might be better.
func (n *Notifier) createJobHistoryAndNotify(job *db.Job, status string, startTime time.Time, endTime time.Time, message string) error {
	// Create the job history entry
	history := db.JobHistory{
		JobID:        job.ID,
		Status:       status,
		StartTime:    startTime,
		EndTime:      &endTime,
		ErrorMessage: message,
	}

	// Save to database - TODO: Move this responsibility?
	if err := n.db.Create(&history).Error; err != nil { // Calls interface method
		n.logger.LogError("Failed to create job history: jobID=%d, error=%v", job.ID, err)
		return err // Return error if history creation fails
	}

	// Create notification based on the *saved* history record (which now has an ID)
	err := n.createJobNotification(job, &history) // Call the dedicated notification creation method
	if err != nil {
		n.logger.LogError("Failed to create job notification: jobID=%d, error=%v", job.ID, err)
		// Continue anyway - notification is not critical
	}

	return nil
}

// Create a notification database record for a job event based on its history.
func (n *Notifier) createJobNotification(job *db.Job, history *db.JobHistory) error {
	// Get the user who created the job
	userID := job.CreatedBy

	// Create job title from job name or ID
	jobTitle := job.Name
	if jobTitle == "" {
		jobTitle = fmt.Sprintf("Job #%d", job.ID)
	}

	// Determine notification type and content
	var notificationType db.NotificationType
	var title string
	var message string

	switch history.Status {
	case "running":
		notificationType = db.NotificationJobStart
		title = "Job Started"
		message = jobTitle
	case "completed":
		notificationType = db.NotificationJobComplete
		title = "Job Complete"
		message = jobTitle
	case "failed":
		notificationType = db.NotificationJobFail
		title = "Job Failed"
		message = jobTitle
		if history.ErrorMessage != "" {
			message = jobTitle + ": " + history.ErrorMessage
		}
	default:
		// Don't create notifications for other statuses like 'completed_with_errors' here?
		// Or maybe map 'completed_with_errors' to NotificationJobComplete?
		n.logger.LogDebug("Skipping DB notification creation for status: %s", history.Status)
		return nil
	}

	// Create the notification record in the database
	return n.db.CreateJobNotification( // Calls interface method
		userID,
		job.ID,
		history.ID, // Use the actual history ID
		notificationType,
		title,
		message,
	)
}

// sendPushbulletNotification sends a notification via Pushbullet.
func (n *Notifier) sendPushbulletNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	n.logger.LogDebug("Sending Pushbullet notification for job %d", job.ID)

	// Get API key from service config
	apiKey, ok := service.Config["api_key"]
	if !ok || apiKey == "" {
		return fmt.Errorf("missing API key for Pushbullet notification")
	}

	// Get device identifier (optional)
	deviceIden := service.Config["device_iden"]

	// Prepare notification title
	titleTemplate := service.Config["title_template"]
	if titleTemplate == "" {
		titleTemplate = "GoMFT: {{job.event}} - {{job.name}}"
	}

	// Prepare notification body
	bodyTemplate := service.Config["body_template"]
	if bodyTemplate == "" {
		bodyTemplate = "Job '{{job.name}}' {{job.status}} at {{job.completed_at}}. {{job.file_count}} files transferred ({{job.transfer_bytes}} bytes)."
	}

	// Create variables for template replacement
	variables := generateDefaultPayload(job, history, config, eventType) // Use package-level helper

	// Replace variables in templates
	title := replaceVariables(titleTemplate, variables) // Use package-level helper
	body := replaceVariables(bodyTemplate, variables)   // Use package-level helper

	// Prepare request data
	url := "https://api.pushbullet.com/v2/pushes"
	data := map[string]interface{}{
		"type":  "note",
		"title": title,
		"body":  body,
	}

	// Add device identifier if provided
	if deviceIden != "" {
		data["device_iden"] = deviceIden
	}

	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal Pushbullet notification data: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Pushbullet request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Token", apiKey)

	// Send the request
	client := createHTTPClient(10*time.Second, n.skipSSLVerify)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Pushbullet notification: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Pushbullet API returned error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	n.logger.LogInfo("Successfully sent Pushbullet notification for job %d", job.ID)
	return nil
}

// sendNtfyNotification sends a notification via ntfy.sh or a self-hosted ntfy server.
func (n *Notifier) sendNtfyNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	n.logger.LogDebug("Sending ntfy notification for job %d", job.ID)

	// Get ntfy server and topic from service config
	topic, ok := service.Config["topic"]
	if !ok || topic == "" {
		return fmt.Errorf("missing topic for ntfy notification")
	}

	// Get server (use default if not provided)
	server := service.Config["server"]
	if server == "" {
		server = "https://ntfy.sh"
	}

	// Prepare notification title
	titleTemplate := service.Config["title_template"]
	if titleTemplate == "" {
		titleTemplate = "GoMFT: {{job.event}} - {{job.name}}"
	}

	// Prepare notification body
	messageTemplate := service.Config["message_template"]
	if messageTemplate == "" {
		messageTemplate = "Job '{{job.name}}' {{job.status}} at {{job.completed_at}}. {{job.file_count}} files transferred ({{job.transfer_bytes}} bytes)."
	}

	// Get priority if specified, default to 3
	priority := 3
	if priorityStr, ok := service.Config["priority"]; ok && priorityStr != "" {
		if p, err := strconv.Atoi(priorityStr); err == nil && p >= 1 && p <= 5 {
			priority = p
		}
	}

	// Create variables for template replacement
	variables := generateDefaultPayload(job, history, config, eventType) // Use package-level helper

	// Replace variables in templates
	title := replaceVariables(titleTemplate, variables)     // Use package-level helper
	message := replaceVariables(messageTemplate, variables) // Use package-level helper

	// Create the URL for the notification
	ntfyURL := fmt.Sprintf("%s/%s", strings.TrimRight(server, "/"), topic)

	// Create the notification data
	ntfyData := map[string]interface{}{
		"topic":    topic,
		"title":    title,
		"message":  message,
		"priority": priority,
	}

	// Add username and password if provided
	username := service.Config["username"]
	password := service.Config["password"]

	// Convert data to JSON
	jsonData, err := json.Marshal(ntfyData)
	if err != nil {
		return fmt.Errorf("failed to marshal ntfy notification data: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", ntfyURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create ntfy request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add basic auth if credentials provided
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	// Send the request
	client := createHTTPClient(10*time.Second, n.skipSSLVerify)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send ntfy notification: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ntfy API returned error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	n.logger.LogInfo("Successfully sent ntfy notification for job %d", job.ID)
	return nil
}

// sendGotifyNotification sends a notification via Gotify.
func (n *Notifier) sendGotifyNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	n.logger.LogDebug("Sending Gotify notification for job %d", job.ID)

	// Get Gotify server URL and token from service config
	serverURL, ok := service.Config["url"]
	if !ok || serverURL == "" {
		return fmt.Errorf("missing server URL for Gotify notification")
	}

	token, ok := service.Config["token"]
	if !ok || token == "" {
		return fmt.Errorf("missing application token for Gotify notification")
	}

	// Prepare notification title
	titleTemplate := service.Config["title_template"]
	if titleTemplate == "" {
		titleTemplate = "GoMFT: {{job.event}} - {{job.name}}"
	}

	// Prepare notification message
	messageTemplate := service.Config["message_template"]
	if messageTemplate == "" {
		messageTemplate = "Job '{{job.name}}' {{job.status}} at {{job.completed_at}}. {{job.file_count}} files transferred ({{job.transfer_bytes}} bytes)."
	}

	// Get priority if specified, default to 5
	priority := 5
	if priorityStr, ok := service.Config["priority"]; ok && priorityStr != "" {
		if p, err := strconv.Atoi(priorityStr); err == nil {
			priority = p
		}
	}

	// Create variables for template replacement
	variables := generateDefaultPayload(job, history, config, eventType) // Use package-level helper

	// Replace variables in templates
	title := replaceVariables(titleTemplate, variables)     // Use package-level helper
	message := replaceVariables(messageTemplate, variables) // Use package-level helper

	// Create the URL for the notification
	gotifyURL := fmt.Sprintf("%s/message", strings.TrimRight(serverURL, "/"))

	// Create the notification data
	gotifyData := map[string]interface{}{
		"title":    title,
		"message":  message,
		"priority": priority,
	}

	// Convert data to JSON
	jsonData, err := json.Marshal(gotifyData)
	if err != nil {
		return fmt.Errorf("failed to marshal Gotify notification data: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", gotifyURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Gotify request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gotify-Key", token)

	// Send the request
	client := createHTTPClient(10*time.Second, n.skipSSLVerify)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Gotify notification: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Gotify API returned error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	n.logger.LogInfo("Successfully sent Gotify notification for job %d", job.ID)
	return nil
}

// sendPushoverNotification sends a notification via Pushover.
func (n *Notifier) sendPushoverNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	n.logger.LogDebug("Sending Pushover notification for job %d", job.ID)

	// Get Pushover tokens from service config
	appToken, ok := service.Config["app_token"]
	if !ok || appToken == "" {
		return fmt.Errorf("missing application token/key for Pushover notification")
	}

	userKey, ok := service.Config["user_key"]
	if !ok || userKey == "" {
		return fmt.Errorf("missing user key for Pushover notification")
	}

	// Get optional device
	device := service.Config["device"]

	// Prepare notification title
	titleTemplate := service.Config["title_template"]
	if titleTemplate == "" {
		titleTemplate = "GoMFT: {{job.event}} - {{job.name}}"
	}

	// Prepare notification message
	messageTemplate := service.Config["message_template"]
	if messageTemplate == "" {
		messageTemplate = "Job '{{job.name}}' {{job.status}} at {{job.completed_at}}. {{job.file_count}} files transferred ({{job.transfer_bytes}} bytes)."
	}

	// Get priority if specified, default to 0 (normal)
	priority := 0
	if priorityStr, ok := service.Config["priority"]; ok && priorityStr != "" {
		if p, err := strconv.Atoi(priorityStr); err == nil && p >= -2 && p <= 2 {
			priority = p
		}
	}

	// Get sound if specified
	sound := service.Config["sound"]
	if sound == "" {
		sound = "pushover" // Default sound
	}

	// Create variables for template replacement
	variables := generateDefaultPayload(job, history, config, eventType) // Use package-level helper

	// Replace variables in templates
	title := replaceVariables(titleTemplate, variables)     // Use package-level helper
	message := replaceVariables(messageTemplate, variables) // Use package-level helper

	// Create the URL for the notification
	pushoverURL := "https://api.pushover.net/1/messages.json"

	// Create the form data
	formData := url.Values{}
	formData.Set("token", appToken)
	formData.Set("user", userKey)
	formData.Set("title", title)
	formData.Set("message", message)
	formData.Set("priority", strconv.Itoa(priority))
	formData.Set("sound", sound)

	// Add device if specified
	if device != "" {
		formData.Set("device", device)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", pushoverURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create Pushover request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	client := createHTTPClient(10*time.Second, n.skipSSLVerify)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Pushover notification: %v", err)
	}
	defer resp.Body.Close()

	// Check response and parse the JSON
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Pushover API returned error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Check response for success status
	var pushoverResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pushoverResp); err != nil {
		return fmt.Errorf("failed to decode Pushover response: %v", err)
	}

	// Verify the status is 1 (success)
	if status, ok := pushoverResp["status"].(float64); !ok || status != 1 {
		errMsg := "unknown error"
		if errors, ok := pushoverResp["errors"].([]interface{}); ok && len(errors) > 0 {
			errMsg = fmt.Sprintf("%v", errors[0])
		}
		return fmt.Errorf("Pushover API returned error: %s", errMsg)
	}

	n.logger.LogInfo("Successfully sent Pushover notification for job %d", job.ID)
	return nil
}
