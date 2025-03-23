package migrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddNotificationServices creates a migration for adding the notification services table
func AddNotificationServices() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "007_add_notification_services",
		Migrate: func(tx *gorm.DB) error {
			// Check if any tables exist (indicating an existing database)
			var count int64
			if err := tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
				return fmt.Errorf("failed to check for existing tables: %v", err)
			}

			// If tables exist, create a backup
			if count > 0 {
				// Get the database path
				sqlDB, err := tx.DB()
				if err != nil {
					return fmt.Errorf("failed to get underlying database: %v", err)
				}

				var seq int
				var name, dbPath string
				if err := sqlDB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); err != nil {
					return fmt.Errorf("failed to get database path: %v", err)
				}

				// Get backup directory from environment variable or use default
				backupDir := os.Getenv("BACKUP_DIR")
				if backupDir == "" {
					backupDir = "/app/backups" // Default Docker path
					// Check if we're not in Docker
					if _, err := os.Stat(backupDir); os.IsNotExist(err) {
						backupDir = "backups" // Fallback to local directory
					}
				}

				// Create backup directory if it doesn't exist
				if err := os.MkdirAll(backupDir, 0755); err != nil {
					return fmt.Errorf("failed to create backup directory: %v", err)
				}

				// Create backup file with timestamp in the backup directory
				dbFileName := filepath.Base(dbPath)
				backupFileName := fmt.Sprintf("%s.backup.%s", dbFileName, time.Now().Format("20060102_150405"))
				backupFile := filepath.Join(backupDir, backupFileName)

				// Read original database
				data, err := os.ReadFile(dbPath)
				if err != nil {
					return fmt.Errorf("failed to read database for backup: %v", err)
				}

				// Write backup
				if err := os.WriteFile(backupFile, data, 0600); err != nil {
					return fmt.Errorf("failed to create database backup: %v", err)
				}

				fmt.Printf("Created database backup at: %s\n", backupFile)
			}

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
