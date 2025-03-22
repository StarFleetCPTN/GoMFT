package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func InitialSchema() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "001_initial_schema",
		Migrate: func(tx *gorm.DB) error {
			// Disable foreign key constraints while creating tables
			if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				return fmt.Errorf("failed to disable foreign key constraints: %v", err)
			}
			defer func() {
				if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
					fmt.Printf("Warning: failed to re-enable foreign key constraints: %v\n", err)
				}
			}()

			// Create Users table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				email VARCHAR(255) NOT NULL UNIQUE,
				password_hash VARCHAR(255) NOT NULL,
				is_admin BOOLEAN DEFAULT FALSE,
				last_password_change DATETIME,
				failed_login_attempts INTEGER DEFAULT 0,
				account_locked BOOLEAN DEFAULT FALSE,
				lockout_until DATETIME,
				theme VARCHAR(255) DEFAULT 'light',
				created_at DATETIME,
				updated_at DATETIME
			)`).Error; err != nil {
				return err
			}

			// Create PasswordHistory table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS password_histories (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				password_hash VARCHAR(255) NOT NULL,
				created_at DATETIME,
				FOREIGN KEY (user_id) REFERENCES users(id)
			)`).Error; err != nil {
				return err
			}

			// Create PasswordResetToken table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS password_reset_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				token VARCHAR(255) NOT NULL,
				expires_at DATETIME NOT NULL,
				used BOOLEAN DEFAULT FALSE,
				created_at DATETIME,
				updated_at DATETIME,
				FOREIGN KEY (user_id) REFERENCES users(id)
			)`).Error; err != nil {
				return err
			}

			// Create TransferConfigs table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS transfer_configs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name VARCHAR(255) NOT NULL,
				source_type VARCHAR(255) NOT NULL,
				source_path TEXT NOT NULL,
				source_host VARCHAR(255),
				source_port INTEGER DEFAULT 22,
				source_user VARCHAR(255),
				source_key_file TEXT,
				source_bucket VARCHAR(255),
				source_region VARCHAR(255),
				source_access_key VARCHAR(255),
				source_endpoint VARCHAR(255),
				source_share VARCHAR(255),
				source_domain VARCHAR(255),
				source_passive_mode BOOLEAN DEFAULT TRUE,
				source_client_id VARCHAR(255),
				source_drive_id VARCHAR(255),
				source_team_drive VARCHAR(255),
				source_read_only BOOLEAN,
				source_start_year INTEGER,
				source_include_archived BOOLEAN,
				file_pattern VARCHAR(255) DEFAULT '*',
				output_pattern TEXT,
				destination_type VARCHAR(255) NOT NULL,
				destination_path TEXT NOT NULL,
				dest_host VARCHAR(255),
				dest_port INTEGER DEFAULT 22,
				dest_user VARCHAR(255),
				dest_key_file TEXT,
				dest_bucket VARCHAR(255),
				dest_region VARCHAR(255),
				dest_access_key VARCHAR(255),
				dest_endpoint VARCHAR(255),
				dest_share VARCHAR(255),
				dest_domain VARCHAR(255),
				dest_passive_mode BOOLEAN DEFAULT TRUE,
				dest_client_id VARCHAR(255),
				dest_drive_id VARCHAR(255),
				dest_team_drive VARCHAR(255),
				dest_read_only BOOLEAN,
				dest_start_year INTEGER,
				dest_include_archived BOOLEAN,
				use_builtin_auth_source BOOLEAN DEFAULT TRUE,
				use_builtin_auth_dest BOOLEAN DEFAULT TRUE,
				google_drive_authenticated BOOLEAN,
				archive_path TEXT,
				archive_enabled BOOLEAN DEFAULT FALSE,
				rclone_flags TEXT,
				delete_after_transfer BOOLEAN DEFAULT FALSE,
				skip_processed_files BOOLEAN DEFAULT TRUE,
				max_concurrent_transfers INTEGER DEFAULT 4,
				created_by INTEGER,
				created_at DATETIME,
				updated_at DATETIME,
				FOREIGN KEY (created_by) REFERENCES users(id)
			)`).Error; err != nil {
				return err
			}

			// Create Jobs table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS jobs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name VARCHAR(255),
				config_id INTEGER NOT NULL,
				config_ids TEXT,
				schedule VARCHAR(255) NOT NULL,
				enabled BOOLEAN DEFAULT TRUE,
				last_run DATETIME,
				next_run DATETIME,
				webhook_enabled BOOLEAN DEFAULT FALSE,
				webhook_url TEXT,
				webhook_secret TEXT,
				webhook_headers TEXT,
				notify_on_success BOOLEAN DEFAULT TRUE,
				notify_on_failure BOOLEAN DEFAULT TRUE,
				created_by INTEGER,
				created_at DATETIME,
				updated_at DATETIME,
				FOREIGN KEY (config_id) REFERENCES transfer_configs(id),
				FOREIGN KEY (created_by) REFERENCES users(id)
			)`).Error; err != nil {
				return err
			}

			// Create JobHistory table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS job_histories (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				job_id INTEGER NOT NULL,
				config_id INTEGER DEFAULT 0,
				start_time DATETIME NOT NULL,
				end_time DATETIME,
				status VARCHAR(255) NOT NULL,
				bytes_transferred INTEGER,
				files_transferred INTEGER,
				error_message TEXT,
				FOREIGN KEY (job_id) REFERENCES jobs(id)
			)`).Error; err != nil {
				return err
			}

			// Create FileMetadata table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS file_metadata (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				job_id INTEGER NOT NULL,
				config_id INTEGER DEFAULT 0,
				file_name VARCHAR(255) NOT NULL,
				original_path TEXT NOT NULL,
				file_size INTEGER NOT NULL,
				file_hash VARCHAR(255),
				creation_time DATETIME,
				mod_time DATETIME,
				processed_time DATETIME NOT NULL,
				destination_path TEXT NOT NULL,
				status VARCHAR(255) NOT NULL,
				error_message TEXT,
				created_at DATETIME,
				updated_at DATETIME,
				FOREIGN KEY (job_id) REFERENCES jobs(id)
			)`).Error; err != nil {
				return err
			}

			// Re-enable foreign key constraints and verify integrity
			if err := tx.Exec("PRAGMA foreign_key_check").Error; err != nil {
				return fmt.Errorf("foreign key integrity check failed: %v", err)
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop tables in reverse order to handle foreign key constraints
			tables := []string{
				"file_metadata",
				"job_histories",
				"jobs",
				"transfer_configs",
				"password_reset_tokens",
				"password_histories",
				"users",
			}
			for _, table := range tables {
				if err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)).Error; err != nil {
					return err
				}
			}
			return nil
		},
	}
}
