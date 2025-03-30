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

// AlterBooleanDefaults changes boolean columns with default:true to pointers
// using explicit table recreation with raw SQL for SQLite compatibility.
func AlterBooleanDefaults() *gormigrate.Migration {

	// --- Raw SQL CREATE TABLE statements for the target schema ---

	const createNotificationServicesSQL = `
	CREATE TABLE notification_services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		is_enabled INTEGER DEFAULT 1, -- Target: *bool, SQLite uses 0/1, default true
		config TEXT,
		description TEXT,
		event_triggers TEXT DEFAULT '[]',
		payload_template TEXT,
		secret_key TEXT,
		retry_policy TEXT DEFAULT 'simple',
		last_used timestamp,
		success_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		created_by INTEGER,
		created_at timestamp,
		updated_at timestamp
	);`

	const createAuthProvidersSQL = `
	CREATE TABLE auth_providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		enabled INTEGER DEFAULT 1, -- Target: *bool, SQLite uses 0/1, default true
		description TEXT,
		provider_url TEXT,
		client_id TEXT,
		client_secret TEXT,
		redirect_url TEXT,
		scopes TEXT,
		attribute_mapping TEXT,
		config TEXT,
		icon_url TEXT,
		successful_logins INTEGER DEFAULT 0,
		last_used timestamp,
		created_at timestamp,
		updated_at timestamp
	);`

	const createTransferConfigsSQL = `
	CREATE TABLE transfer_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		source_type TEXT NOT NULL,
		source_path TEXT NOT NULL,
		source_host TEXT,
		source_port INTEGER DEFAULT 22,
		source_user TEXT,
		source_key_file TEXT,
		source_bucket TEXT,
		source_region TEXT,
		source_access_key TEXT,
		source_endpoint TEXT,
		source_share TEXT,
		source_domain TEXT,
		source_passive_mode INTEGER DEFAULT 1, -- Already *bool, keep default
		source_client_id TEXT,
		source_drive_id TEXT,
		source_team_drive TEXT,
		source_read_only INTEGER,
		source_start_year INTEGER,
		source_include_archived INTEGER,
		file_pattern TEXT DEFAULT '*',
		output_pattern TEXT,
		destination_type TEXT NOT NULL,
		destination_path TEXT NOT NULL,
		dest_host TEXT,
		dest_port INTEGER DEFAULT 22,
		dest_user TEXT,
		dest_key_file TEXT,
		dest_bucket TEXT,
		dest_region TEXT,
		dest_access_key TEXT,
		dest_endpoint TEXT,
		dest_share TEXT,
		dest_domain TEXT,
		dest_passive_mode INTEGER DEFAULT 1, -- Already *bool, keep default
		dest_client_id TEXT,
		dest_drive_id TEXT,
		dest_team_drive TEXT,
		dest_read_only INTEGER,
		dest_start_year INTEGER,
		dest_include_archived INTEGER,
		use_builtin_auth_source INTEGER,
		use_builtin_auth_dest INTEGER,
		google_drive_authenticated INTEGER,
		archive_path TEXT,
		archive_enabled INTEGER DEFAULT 0,
		rclone_flags TEXT,
		command_id INTEGER DEFAULT 1,
		command_flags TEXT,
		command_flag_values TEXT,
		delete_after_transfer INTEGER DEFAULT 0,
		skip_processed_files INTEGER DEFAULT 1, -- Target: *bool, SQLite uses 0/1, default true
		max_concurrent_transfers INTEGER DEFAULT 4,
		created_by INTEGER,
		created_at timestamp,
		updated_at timestamp
	);`

	// --- End Raw SQL ---

	return &gormigrate.Migration{
		ID: "012_alter_boolean_defaults",
		Migrate: func(tx *gorm.DB) error {
			// --- Backup Logic (copied) ---
			var count int64
			if err := tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
				return fmt.Errorf("failed to check for existing tables: %v", err)
			}
			if count > 0 {
				sqlDB, err := tx.DB()
				if err != nil {
					return fmt.Errorf("failed to get underlying database: %v", err)
				}
				var seq int
				var name, dbPath string
				if err := sqlDB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); err != nil {
					return fmt.Errorf("failed to get database path: %v", err)
				}
				backupDir := os.Getenv("BACKUP_DIR")
				if backupDir == "" {
					backupDir = "/app/backups"
					if _, err := os.Stat(backupDir); os.IsNotExist(err) {
						backupDir = "backups"
					}
				}
				if err := os.MkdirAll(backupDir, 0755); err != nil {
					return fmt.Errorf("failed to create backup directory: %v", err)
				}
				dbFileName := filepath.Base(dbPath)
				backupFileName := fmt.Sprintf("%s.backup.%s", dbFileName, time.Now().Format("20060102_150405"))
				backupFile := filepath.Join(backupDir, backupFileName)
				data, err := os.ReadFile(dbPath)
				if err != nil {
					return fmt.Errorf("failed to read database for backup: %v", err)
				}
				if err := os.WriteFile(backupFile, data, 0600); err != nil {
					return fmt.Errorf("failed to create database backup: %v", err)
				}
				fmt.Printf("Created database backup at: %s\n", backupFile)
			}
			// --- End Backup Logic ---

			// --- Table Recreation Logic for SQLite ---
			if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				return fmt.Errorf("failed to disable foreign keys: %w", err)
			}
			defer func() {
				if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
					fmt.Printf("Warning: failed to re-enable foreign keys: %v\n", err)
				}
			}()

			// Helper function for table recreation
			recreateTable := func(tableName, createSQL string) error {
				fmt.Printf("Recreating table %s...\n", tableName)
				oldTableName := fmt.Sprintf("_%s_old", tableName)

				// Rename old table
				if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tableName, oldTableName)).Error; err == nil {
					fmt.Printf("Renamed %s to %s.\n", tableName, oldTableName)

					// Create new table using raw SQL
					fmt.Printf("Creating new %s table...\n", tableName)
					if err := tx.Exec(createSQL).Error; err != nil {
						return fmt.Errorf("failed to create new %s table: %w", tableName, err)
					}
					fmt.Printf("New %s table created.\n", tableName)

					// Copy data
					fmt.Printf("Copying data to new %s table...\n", tableName)
					// IMPORTANT: Ensure column order/names match if schema changed beyond types/defaults
					if err := tx.Exec(fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", tableName, oldTableName)).Error; err != nil {
						return fmt.Errorf("failed to copy data to new %s table: %w", tableName, err)
					}
					fmt.Printf("Data copied to %s.\n", tableName)

					// Drop old table
					if err := tx.Exec(fmt.Sprintf("DROP TABLE %s", oldTableName)).Error; err != nil {
						return fmt.Errorf("failed to drop old %s table: %w", tableName, err)
					}
					fmt.Printf("Successfully recreated %s.\n", tableName)
				} else {
					// Check if rename failed because table doesn't exist (fresh install)
					var tableExists int
					tx.Raw(fmt.Sprintf("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='%s'", tableName)).Scan(&tableExists)
					if tableExists == 0 {
						fmt.Printf("%s table does not exist, creating.\n", tableName)
						if err := tx.Exec(createSQL).Error; err != nil { // Create table directly
							return fmt.Errorf("failed to create new %s table: %w", tableName, err)
						}
					} else {
						return fmt.Errorf("failed to rename %s: %w", tableName, err) // Real rename error
					}
				}
				return nil
			}

			// Recreate tables
			if err := recreateTable("notification_services", createNotificationServicesSQL); err != nil {
				return err
			}
			if err := recreateTable("auth_providers", createAuthProvidersSQL); err != nil {
				return err
			}
			if err := recreateTable("transfer_configs", createTransferConfigsSQL); err != nil {
				return err
			}

			// --- End Table Recreation Logic ---

			// Create audit log entry
			now := time.Now()
			details, auditErr := json.Marshal(map[string]interface{}{
				"tables_affected": []string{"notification_services", "auth_providers", "transfer_configs"},
				"columns_altered": []string{"is_enabled", "enabled", "skip_processed_files"},
				"new_type":        "*bool (pointer to boolean)",
				"method":          "Table recreation (SQLite - Raw SQL)",
				"message":         "Changed boolean columns with default:true to pointers to handle false values correctly with GORM.",
			})
			if auditErr != nil {
				fmt.Printf("Warning: Failed to marshal audit log details: %v\n", auditErr)
			}

			if auditErr == nil {
				if auditExecErr := tx.Exec(`
					INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
					VALUES ('schema_update', 'multiple_tables', 0, 1, ?, ?, ?, ?)
				`, string(details), now, now, now).Error; auditExecErr != nil {
					fmt.Printf("Warning: Failed to insert audit log: %v\n", auditExecErr)
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Rollback is complex and risky with table recreation. Log skip.
			now := time.Now()
			details, err := json.Marshal(map[string]interface{}{
				"migration_id": "012_alter_boolean_defaults",
				"message":      "Skipping rollback of boolean column type changes (via table recreation) due to complexity/potential data loss.",
			})
			if err != nil {
				fmt.Printf("Warning: Failed to marshal rollback audit log details: %v\n", err)
			}

			if err == nil {
				if auditExecErr := tx.Exec(`
					INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
					VALUES ('migration_rollback', 'multiple_tables', 0, 1, ?, ?, ?, ?)
				`, string(details), now, now, now).Error; auditExecErr != nil {
					fmt.Printf("Warning: Failed to insert rollback audit log: %v\n", auditExecErr)
				}
			}
			fmt.Println("Rollback for migration 012_alter_boolean_defaults skipped for safety.")
			return nil
		},
	}
}
