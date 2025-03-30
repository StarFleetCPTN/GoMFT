package utils

import (
	"strings"
)

// GetNotificationFormTitle returns the title for the notification form page.
func GetNotificationFormTitle(isNew bool) string {
	if isNew {
		return "Add Notification Service"
	}
	return "Edit Notification Service"
}

// Contains checks if a string slice contains a specific string.
func Contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

// BoolToString converts a boolean to its string representation "true" or "false".
// Useful for setting HTML attributes that expect string values.
func BoolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// IsEventTriggerSelected checks if a specific event trigger should be pre-selected.
// It now accepts the anonymous struct type defined in types.NotificationFormData.
// Defaults to checking 'job_complete' and 'job_error' when creating a new service.
func IsEventTriggerSelected(service *struct {
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
}, event string, isNew bool) bool {
	if !isNew && service != nil {
		// Use the Contains helper function
		return Contains(service.EventTriggers, event)
	}
	// Default for new services: check complete and error
	return isNew && (event == "job_complete" || event == "job_error")
}

// FormatEventTriggerName converts event trigger keys to human-readable names.
func FormatEventTriggerName(event string) string {
	// Replace underscores with spaces and capitalize words
	name := strings.ReplaceAll(event, "_", " ")
	name = strings.Title(name) // Use strings.Title for capitalization
	return name
}
