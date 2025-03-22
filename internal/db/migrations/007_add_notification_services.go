package migrations

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddNotificationServices creates a migration for adding the notification services table
func AddNotificationServices() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "007_add_notification_services",
		Migrate: func(tx *gorm.DB) error {
			// Create notification_services table
			type NotificationService struct {
				ID              uint   `gorm:"primaryKey"`
				Name            string `gorm:"not null"`
				Type            string `gorm:"not null"` // email, slack, webhook
				IsEnabled       bool   `gorm:"default:true"`
				ConfigJSON      string `gorm:"column:config"`
				Description     string
				EventTriggers   string    `gorm:"column:event_triggers;default:'[]'"`
				PayloadTemplate string    `gorm:"column:payload_template"`
				SecretKey       string    `gorm:"column:secret_key"`
				RetryPolicy     string    `gorm:"column:retry_policy;default:'simple'"`
				LastUsed        time.Time `gorm:"column:last_used"`
				SuccessCount    int       `gorm:"column:success_count;default:0"`
				FailureCount    int       `gorm:"column:failure_count;default:0"`
				CreatedBy       uint
				CreatedAt       time.Time
				UpdatedAt       time.Time
			}

			if err := tx.AutoMigrate(&NotificationService{}); err != nil {
				return fmt.Errorf("failed to create notification_services table: %v", err)
			}

			// Create audit log entry
			now := time.Now()
			details, err := json.Marshal(map[string]interface{}{
				"table_name":     "notification_services",
				"system_created": true,
			})
			if err != nil {
				return err
			}

			return tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('create_table', 'schema', 0, 1, ?, ?, ?, ?)
			`, string(details), now, now, now).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Log rollback operation
			now := time.Time{}
			details, err := json.Marshal(map[string]interface{}{
				"table_name": "notification_services",
				"message":    "Dropping notification_services table",
			})
			if err != nil {
				return err
			}

			if err := tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('drop_table', 'schema', 0, 1, ?, ?, ?, ?)
			`, string(details), now, now, now).Error; err != nil {
				return err
			}

			// Drop the table
			return tx.Migrator().DropTable("notification_services")
		},
	}
}
