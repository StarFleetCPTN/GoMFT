package db

import (
	"fmt"
	"time"
)

// NotificationType defines the type of notification
type NotificationType string

const (
	NotificationJobStart     NotificationType = "job_start"
	NotificationJobComplete  NotificationType = "job_complete"
	NotificationJobFail      NotificationType = "job_fail"
	NotificationConfigUpdate NotificationType = "config_update"
	NotificationSystemAlert  NotificationType = "system_alert"
)

// UserNotification represents a notification shown to users in the UI
type UserNotification struct {
	ID        uint             `json:"id" gorm:"primaryKey"`
	UserID    uint             `json:"user_id" gorm:"index"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Message   string           `json:"message"`
	Link      string           `json:"link"`
	JobID     uint             `json:"job_id,omitempty"`
	JobRunID  uint             `json:"job_run_id,omitempty"`
	ConfigID  uint             `json:"config_id,omitempty"`
	IsRead    bool             `json:"is_read" gorm:"default:false"`
	CreatedAt time.Time        `json:"created_at"`
}

// GetUserNotifications returns the latest notifications for a user
func (db *DB) GetUserNotifications(userID uint, limit int) ([]UserNotification, error) {
	var notifications []UserNotification
	result := db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&notifications)
	return notifications, result.Error
}

// GetUserNotificationCount returns the total count of notifications for a user
func (db *DB) GetUserNotificationCount(userID uint) (int64, error) {
	var count int64
	result := db.Model(&UserNotification{}).Where("user_id = ?", userID).Count(&count)
	return count, result.Error
}

// GetPaginatedUserNotifications returns paginated notifications for a user
func (db *DB) GetPaginatedUserNotifications(userID uint, offset, limit int) ([]UserNotification, error) {
	var notifications []UserNotification
	result := db.Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&notifications)
	return notifications, result.Error
}

// GetUnreadNotificationCount returns the count of unread notifications for a user
func (db *DB) GetUnreadNotificationCount(userID uint) (int64, error) {
	var count int64
	result := db.Model(&UserNotification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&count)
	return count, result.Error
}

// MarkNotificationAsRead marks a notification as read
func (db *DB) MarkNotificationAsRead(id uint) error {
	return db.Model(&UserNotification{}).Where("id = ?", id).Update("is_read", true).Error
}

// MarkAllNotificationsAsRead marks all notifications for a user as read
func (db *DB) MarkAllNotificationsAsRead(userID uint) error {
	return db.Model(&UserNotification{}).Where("user_id = ?", userID).Update("is_read", true).Error
}

// CreateJobNotification creates a notification for a job event
func (db *DB) CreateJobNotification(
	userID uint,
	jobID uint,
	jobRunID uint,
	notificationType NotificationType,
	title string,
	message string,
) error {
	notification := UserNotification{
		UserID:    userID,
		Type:      notificationType,
		Title:     title,
		Message:   message,
		JobID:     jobID,
		JobRunID:  jobRunID,
		Link:      generateJobRunLink(jobRunID),
		CreatedAt: time.Now(),
	}
	return db.Create(&notification).Error
}

// CreateConfigNotification creates a notification for a config update
func (db *DB) CreateConfigNotification(
	userID uint,
	configID uint,
	title string,
	message string,
) error {
	notification := UserNotification{
		UserID:    userID,
		Type:      NotificationConfigUpdate,
		Title:     title,
		Message:   message,
		ConfigID:  configID,
		Link:      generateConfigLink(configID),
		CreatedAt: time.Now(),
	}
	return db.Create(&notification).Error
}

// CreateSystemNotification creates a system-wide notification
func (db *DB) CreateSystemNotification(
	title string,
	message string,
) error {
	// Get all active users
	var users []User
	if err := db.Where("active = ?", true).Find(&users).Error; err != nil {
		return err
	}

	// Create a notification for each user
	for _, user := range users {
		notification := UserNotification{
			UserID:    user.ID,
			Type:      NotificationSystemAlert,
			Title:     title,
			Message:   message,
			CreatedAt: time.Now(),
		}
		if err := db.Create(&notification).Error; err != nil {
			return err
		}
	}
	return nil
}

// Helper functions to generate links
func generateJobRunLink(jobRunID uint) string {
	return "/job-runs/" + fmt.Sprintf("%d", jobRunID)
}

func generateConfigLink(configID uint) string {
	return "/configs/" + fmt.Sprintf("%d", configID)
}
