package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddRcloneCommandToConfig adds rclone command fields to the transfer_configs table
func AddRcloneCommandToConfig() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "010_add_rclone_command_to_config",
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

			// Add the command_id and command_flags columns to the transfer_configs table
			if err := tx.Exec(`ALTER TABLE transfer_configs ADD COLUMN command_id INTEGER DEFAULT NULL REFERENCES rclone_commands(id)`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`ALTER TABLE transfer_configs ADD COLUMN command_flags TEXT DEFAULT NULL`).Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Remove the columns in reverse order
			if err := tx.Exec(`ALTER TABLE transfer_configs DROP COLUMN command_flags`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`ALTER TABLE transfer_configs DROP COLUMN command_id`).Error; err != nil {
				return err
			}

			return nil
		},
	}
}
