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

// AddTimestampsToJobHistories creates a migration for adding timestamp columns to job_histories table
func AddTimestampsToJobHistories() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "006_add_timestamps_to_job_histories",
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

			// Add created_at column
			if err := tx.Exec("ALTER TABLE job_histories ADD COLUMN created_at DATETIME").Error; err != nil {
				return fmt.Errorf("failed to add created_at column: %v", err)
			}

			// Add updated_at column
			if err := tx.Exec("ALTER TABLE job_histories ADD COLUMN updated_at DATETIME").Error; err != nil {
				return fmt.Errorf("failed to add updated_at column: %v", err)
			}

			// Set default values for existing records
			now := time.Now()
			if err := tx.Exec("UPDATE job_histories SET created_at = ?, updated_at = ? WHERE created_at IS NULL", now, now).Error; err != nil {
				return fmt.Errorf("failed to update existing records: %v", err)
			}

			// Create audit log entry for the migration
			details, err := json.Marshal(map[string]interface{}{
				"columns_added": []string{"created_at", "updated_at"},
				"table":         "job_histories",
			})
			if err != nil {
				return err
			}

			return tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('schema_update', 'job_histories', 0, 1, ?, ?, ?, ?)
			`, string(details), now, now, now).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// In SQLite, we can't drop columns directly, so we'd need to:
			// 1. Create a new table without the columns
			// 2. Copy data from the old table
			// 3. Drop the old table
			// 4. Rename the new table
			//
			// However, keeping the timestamp columns is generally harmless,
			// so we'll just log a message instead of performing a risky rollback

			now := time.Now()
			details, err := json.Marshal(map[string]interface{}{
				"message": "Skipped removal of timestamp columns from job_histories due to SQLite limitations",
			})
			if err != nil {
				return err
			}

			return tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('migration_rollback', 'job_histories', 0, 1, ?, ?, ?, ?)
			`, string(details), now, now, now).Error
		},
	}
}
