package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddUserNotifications adds the user_notifications table
func AddUserNotifications() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "008_add_user_notifications",
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
			// Create the user_notifications table
			err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS user_notifications (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					type TEXT NOT NULL,
					title TEXT NOT NULL,
					message TEXT NOT NULL,
					link TEXT NOT NULL,
					job_id INTEGER,
					job_run_id INTEGER,
					config_id INTEGER,
					is_read BOOLEAN NOT NULL DEFAULT 0,
					created_at DATETIME NOT NULL,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
				)
			`).Error
			if err != nil {
				return err
			}

			// Create an index on user_id for faster lookups
			err = tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_user_notifications_user_id ON user_notifications(user_id)
			`).Error
			if err != nil {
				return err
			}

			// Create an index on created_at for faster sorting
			err = tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_user_notifications_created_at ON user_notifications(created_at)
			`).Error
			if err != nil {
				return err
			}

			// Create an index on is_read for faster filtering of unread notifications
			err = tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_user_notifications_is_read ON user_notifications(is_read)
			`).Error
			if err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec(`DROP TABLE IF EXISTS user_notifications`).Error
		},
	}
}
