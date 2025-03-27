package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddAuthProviders adds tables for external authentication providers
func AddAuthProviders() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "011_add_auth_providers",
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
				if err := os.WriteFile(backupFile, data, 0644); err != nil {
					return fmt.Errorf("failed to write database backup: %v", err)
				}

				fmt.Printf("Created database backup at %s\n", backupFile)
			}

			// Create auth_providers table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS auth_providers (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name VARCHAR(255) NOT NULL,
				type VARCHAR(50) NOT NULL,
				enabled BOOLEAN DEFAULT TRUE,
				description TEXT,
				provider_url TEXT,
				icon_url TEXT,
				client_id VARCHAR(255),
				client_secret VARCHAR(255),
				redirect_url TEXT,
				scopes TEXT,
				attribute_mapping TEXT,
				config TEXT,
				successful_logins INTEGER DEFAULT 0,
				last_used DATETIME,
				created_at DATETIME,
				updated_at DATETIME
			)`).Error; err != nil {
				return err
			}

			// Create external_user_identities table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS external_user_identities (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				provider_id INTEGER NOT NULL,
				provider_type VARCHAR(50) NOT NULL,
				external_id VARCHAR(255) NOT NULL,
				email VARCHAR(255) NOT NULL,
				username VARCHAR(255),
				display_name VARCHAR(255),
				groups TEXT,
				last_login DATETIME,
				provider_data TEXT,
				created_at DATETIME,
				updated_at DATETIME,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
				FOREIGN KEY (provider_id) REFERENCES auth_providers(id) ON DELETE CASCADE
			)`).Error; err != nil {
				return err
			}

			// Create unique index on provider_id and external_id
			if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_external_user_identities_provider_external 
				ON external_user_identities(provider_id, external_id)`).Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP TABLE IF EXISTS external_user_identities").Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP TABLE IF EXISTS auth_providers").Error; err != nil {
				return err
			}
			return nil
		},
	}
}
