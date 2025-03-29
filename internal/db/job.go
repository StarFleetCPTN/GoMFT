package db

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// Job represents a scheduled transfer task
type Job struct {
	ID        uint           `gorm:"primarykey"`
	Name      string         `form:"name"`
	ConfigID  uint           `gorm:"not null" form:"config_id"`
	Config    TransferConfig `gorm:"foreignkey:ConfigID"`
	ConfigIDs string         `gorm:"column:config_ids"` // Comma-separated list of config IDs
	Schedule  string         `gorm:"not null" form:"schedule"`
	Enabled   *bool          `gorm:"default:true" form:"enabled"`
	LastRun   *time.Time
	NextRun   *time.Time
	// Webhook notification fields
	WebhookEnabled  *bool  `gorm:"default:false" form:"webhook_enabled"`
	WebhookURL      string `form:"webhook_url"`
	WebhookSecret   string `form:"webhook_secret"`
	WebhookHeaders  string `form:"webhook_headers"` // JSON-encoded headers
	NotifyOnSuccess *bool  `gorm:"default:true" form:"notify_on_success"`
	NotifyOnFailure *bool  `gorm:"default:true" form:"notify_on_failure"`
	CreatedBy       uint
	User            User `gorm:"foreignkey:CreatedBy"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// JobHistory records the execution history of a job
type JobHistory struct {
	ID               uint      `gorm:"primarykey"`
	JobID            uint      `gorm:"not null"`
	Job              Job       `gorm:"foreignkey:JobID"`
	ConfigID         uint      `gorm:"default:0"` // The specific config ID this history entry is for
	StartTime        time.Time `gorm:"not null"`
	EndTime          *time.Time
	Status           string `gorm:"not null"`
	BytesTransferred int64
	FilesTransferred int
	ErrorMessage     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// --- Job Helper Methods ---

// GetConfigIDsList returns the list of config IDs as integers
func (j *Job) GetConfigIDsList() []uint {
	if j.ConfigIDs == "" {
		// If ConfigIDs is empty but ConfigID is set, return that as the only ID
		if j.ConfigID > 0 {
			return []uint{j.ConfigID}
		}
		return []uint{}
	}

	// Split the comma-separated string
	strIDs := strings.Split(j.ConfigIDs, ",")
	ids := make([]uint, 0, len(strIDs))

	// Convert each string to uint
	for _, strID := range strIDs {
		if id, err := strconv.ParseUint(strings.TrimSpace(strID), 10, 32); err == nil {
			ids = append(ids, uint(id))
		}
	}

	return ids
}

// SetConfigIDsList sets the config IDs from a slice of uint
func (j *Job) SetConfigIDsList(ids []uint) {
	// Convert to strings
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = strconv.FormatUint(uint64(id), 10)
	}

	// Join with commas
	j.ConfigIDs = strings.Join(strIDs, ",")

	// Debug log the final ConfigIDs string
	log.Printf("SetConfigIDsList: Setting ConfigIDs to: %s (from %v)", j.ConfigIDs, ids)

	// If there's at least one ID, set ConfigID to the first one for backward compatibility
	if len(ids) > 0 {
		j.ConfigID = ids[0]
	} else {
		j.ConfigID = 0 // Ensure ConfigID is cleared if the list is empty
	}
}

// GetConfigIDsAsStrings returns the list of config IDs as strings for template rendering
func (j *Job) GetConfigIDsAsStrings() []string {
	ids := j.GetConfigIDsList()
	strIDs := make([]string, len(ids))

	for i, id := range ids {
		strIDs[i] = fmt.Sprintf("'%d'", id)
	}

	return strIDs
}

// GetEnabled returns the value of Enabled with a default if nil
func (j *Job) GetEnabled() bool {
	if j.Enabled == nil {
		return true // Default to true if not set
	}
	return *j.Enabled
}

// SetEnabled sets the Enabled field
func (j *Job) SetEnabled(value bool) {
	j.Enabled = &value
}

// GetWebhookEnabled returns the value of WebhookEnabled with a default if nil
func (j *Job) GetWebhookEnabled() bool {
	if j.WebhookEnabled == nil {
		return false // Default to false if not set
	}
	return *j.WebhookEnabled
}

// SetWebhookEnabled sets the WebhookEnabled field
func (j *Job) SetWebhookEnabled(value bool) {
	j.WebhookEnabled = &value
}

// GetNotifyOnSuccess returns the value of NotifyOnSuccess with a default if nil
func (j *Job) GetNotifyOnSuccess() bool {
	if j.NotifyOnSuccess == nil {
		return true // Default to true if not set
	}
	return *j.NotifyOnSuccess
}

// SetNotifyOnSuccess sets the NotifyOnSuccess field
func (j *Job) SetNotifyOnSuccess(value bool) {
	j.NotifyOnSuccess = &value
}

// GetNotifyOnFailure returns the value of NotifyOnFailure with a default if nil
func (j *Job) GetNotifyOnFailure() bool {
	if j.NotifyOnFailure == nil {
		return true // Default to true if not set
	}
	return *j.NotifyOnFailure
}

// SetNotifyOnFailure sets the NotifyOnFailure field
func (j *Job) SetNotifyOnFailure(value bool) {
	j.NotifyOnFailure = &value
}
