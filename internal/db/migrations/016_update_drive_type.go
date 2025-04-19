package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// UpdateDriveType updates the source_type and dest_type from 'gdrive' to 'drive'
func UpdateDriveType() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "016_update_drive_type",
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

			// Update source_type
			if err := tx.Exec(`UPDATE transfer_configs SET source_type = 'drive' WHERE source_type = 'gdrive'`).Error; err != nil {
				return err
			}

			// Update dest_type
			return tx.Exec(`UPDATE transfer_configs SET dest_type = 'drive' WHERE dest_type = 'gdrive'`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Revert source_type
			if err := tx.Exec(`UPDATE transfer_configs SET source_type = 'gdrive' WHERE source_type = 'drive'`).Error; err != nil {
				return err
			}

			// Revert dest_type
			return tx.Exec(`UPDATE transfer_configs SET dest_type = 'gdrive' WHERE dest_type = 'drive'`).Error
		},
	}
}
