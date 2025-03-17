package migrations

import (
	"fmt"
	"os"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Add2FA creates a migration for adding Two-Factor Authentication fields
func Add2FA() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "003_add_2fa",
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

				// Create backup file with timestamp
				backupFile := fmt.Sprintf("%s.backup.%s", dbPath, time.Now().Format("20060102_150405"))

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

			// Add new columns for 2FA - one at a time for SQLite compatibility
			if err := tx.Exec(`ALTER TABLE users ADD COLUMN two_factor_secret VARCHAR(32)`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`ALTER TABLE users ADD COLUMN two_factor_enabled BOOLEAN DEFAULT FALSE`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`ALTER TABLE users ADD COLUMN backup_codes TEXT`).Error; err != nil {
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Remove 2FA columns - one at a time for SQLite compatibility
			if err := tx.Exec(`ALTER TABLE users DROP COLUMN two_factor_secret`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`ALTER TABLE users DROP COLUMN two_factor_enabled`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`ALTER TABLE users DROP COLUMN backup_codes`).Error; err != nil {
				return err
			}
			return nil
		},
	}
}
