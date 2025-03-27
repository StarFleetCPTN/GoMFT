package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddAuditLogs creates a migration for adding audit logs and user roles tables
func AddAuditLogs() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "004_add_audit_logs",
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

			// Create roles table first
			if err := tx.Exec(`
				CREATE TABLE roles (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name VARCHAR(255) NOT NULL UNIQUE,
					description TEXT,
					permissions TEXT,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					deleted_at TIMESTAMP NULL
				)
			`).Error; err != nil {
				return err
			}

			// Create audit_logs table
			if err := tx.Exec(`
				CREATE TABLE audit_logs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					deleted_at TIMESTAMP NULL,
					action VARCHAR(50) NOT NULL,
					entity_type VARCHAR(50) NOT NULL,
					entity_id INTEGER NOT NULL,
					user_id INTEGER NOT NULL,
					details TEXT,
					timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					CONSTRAINT idx_audit_logs_action CHECK (action IS NOT NULL),
					CONSTRAINT idx_audit_logs_entity_type CHECK (entity_type IS NOT NULL),
					CONSTRAINT idx_audit_logs_entity_id CHECK (entity_id IS NOT NULL),
					CONSTRAINT idx_audit_logs_user_id CHECK (user_id IS NOT NULL),
					CONSTRAINT idx_audit_logs_timestamp CHECK (timestamp IS NOT NULL)
				)
			`).Error; err != nil {
				return err
			}

			// Create user_roles table for many-to-many relationship
			return tx.Exec(`
				CREATE TABLE user_roles (
					user_id INTEGER NOT NULL,
					role_id INTEGER NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (user_id, role_id),
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
					FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
				)
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP TABLE IF EXISTS user_roles").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP TABLE IF EXISTS audit_logs").Error; err != nil {
				return err
			}
			return tx.Exec("DROP TABLE IF EXISTS roles").Error
		},
	}
}
