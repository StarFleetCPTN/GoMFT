package db

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// NotificationService represents a notification service configuration
type NotificationService struct {
	ID                uint              `json:"id" gorm:"primaryKey"`
	Name              string            `json:"name" gorm:"not null"`
	Type              string            `json:"type" gorm:"not null"` // email, webhook
	IsEnabled         bool              `json:"is_enabled" gorm:"default:true"`
	Config            map[string]string `json:"config" gorm:"-"`
	ConfigJSON        string            `json:"-" gorm:"column:config"`
	Description       string            `json:"description"`
	EventTriggers     []string          `json:"event_triggers" gorm:"-"`
	EventTriggersJSON string            `json:"-" gorm:"column:event_triggers;default:'[]'"`
	PayloadTemplate   string            `json:"payload_template" gorm:"column:payload_template"`
	SecretKey         string            `json:"secret_key" gorm:"column:secret_key"`
	RetryPolicy       string            `json:"retry_policy" gorm:"column:retry_policy;default:'simple'"`
	LastUsed          time.Time         `json:"last_used" gorm:"column:last_used"`
	SuccessCount      int               `json:"success_count" gorm:"column:success_count;default:0"`
	FailureCount      int               `json:"failure_count" gorm:"column:failure_count;default:0"`
	CreatedBy         uint              `json:"created_by"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// BeforeSave converts Config map and EventTriggers to JSON strings for storage
func (n *NotificationService) BeforeSave(tx *gorm.DB) error {
	configJSON, err := json.Marshal(n.Config)
	if err != nil {
		return err
	}
	n.ConfigJSON = string(configJSON)

	eventsJSON, err := json.Marshal(n.EventTriggers)
	if err != nil {
		return err
	}
	n.EventTriggersJSON = string(eventsJSON)

	return nil
}

// AfterFind converts JSON strings back to Config map and EventTriggers
func (n *NotificationService) AfterFind(tx *gorm.DB) error {
	if n.ConfigJSON != "" {
		if err := json.Unmarshal([]byte(n.ConfigJSON), &n.Config); err != nil {
			return err
		}
	}

	if n.EventTriggersJSON != "" {
		if err := json.Unmarshal([]byte(n.EventTriggersJSON), &n.EventTriggers); err != nil {
			return err
		}
	}

	return nil
}

// GetNotificationServices returns notification services, filtered by enabled status if specified
func (db *DB) GetNotificationServices(onlyEnabled bool) ([]NotificationService, error) {
	var services []NotificationService
	query := db.DB

	if onlyEnabled {
		query = query.Where("is_enabled = ?", true)
	}

	if err := query.Find(&services).Error; err != nil {
		return nil, err
	}

	return services, nil
}

// GetNotificationService returns a notification service by ID
func (db *DB) GetNotificationService(id uint) (*NotificationService, error) {
	var service NotificationService
	if err := db.First(&service, id).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

// CreateNotificationService creates a new notification service
func (db *DB) CreateNotificationService(service *NotificationService) error {
	return db.Create(service).Error
}

// UpdateNotificationService updates an existing notification service
func (db *DB) UpdateNotificationService(service *NotificationService) error {
	return db.Save(service).Error
}

// DeleteNotificationService deletes a notification service by ID
func (db *DB) DeleteNotificationService(id uint) error {
	return db.Delete(&NotificationService{}, id).Error
}
