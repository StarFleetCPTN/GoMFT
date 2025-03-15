package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// UpdateSkipProcessedFilesToNullable changes the skip_processed_files column to be nullable
func UpdateSkipProcessedFilesToNullable() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20250515_update_skip_processed_files_to_nullable",
		Migrate: func(tx *gorm.DB) error {
			// SQLite specific command - this would need to be adjusted for other databases
			return tx.Exec("ALTER TABLE transfer_configs RENAME TO transfer_configs_old; " +
				"CREATE TABLE transfer_configs (" +
				"id INTEGER PRIMARY KEY AUTOINCREMENT, " +
				"name VARCHAR(255) NOT NULL, " +
				"source_type VARCHAR(255) NOT NULL, " +
				"source_path VARCHAR(255) NOT NULL, " +
				"source_host VARCHAR(255), " +
				"source_port INTEGER DEFAULT 22, " +
				"source_user VARCHAR(255), " +
				"source_key_file VARCHAR(255), " +
				"source_bucket VARCHAR(255), " +
				"source_region VARCHAR(255), " +
				"source_access_key VARCHAR(255), " +
				"source_endpoint VARCHAR(255), " +
				"source_share VARCHAR(255), " +
				"source_domain VARCHAR(255), " +
				"source_passive_mode BOOLEAN DEFAULT true, " +
				"source_client_id VARCHAR(255), " +
				"source_drive_id VARCHAR(255), " +
				"source_team_drive VARCHAR(255), " +
				"file_pattern VARCHAR(255) DEFAULT '*', " +
				"output_pattern VARCHAR(255), " +
				"destination_type VARCHAR(255) NOT NULL, " +
				"destination_path VARCHAR(255) NOT NULL, " +
				"dest_host VARCHAR(255), " +
				"dest_port INTEGER DEFAULT 22, " +
				"dest_user VARCHAR(255), " +
				"dest_key_file VARCHAR(255), " +
				"dest_bucket VARCHAR(255), " +
				"dest_region VARCHAR(255), " +
				"dest_access_key VARCHAR(255), " +
				"dest_endpoint VARCHAR(255), " +
				"dest_share VARCHAR(255), " +
				"dest_domain VARCHAR(255), " +
				"dest_passive_mode BOOLEAN DEFAULT true, " +
				"dest_client_id VARCHAR(255), " +
				"dest_drive_id VARCHAR(255), " +
				"dest_team_drive VARCHAR(255), " +
				"archive_path VARCHAR(255), " +
				"archive_enabled BOOLEAN DEFAULT false, " +
				"rclone_flags VARCHAR(255), " +
				"delete_after_transfer BOOLEAN DEFAULT false, " +
				"skip_processed_files BOOLEAN DEFAULT true, " + // Keep as BOOLEAN, but now it's nullable
				"max_concurrent_transfers INTEGER DEFAULT 4, " +
				"created_by INTEGER, " +
				"created_at DATETIME, " +
				"updated_at DATETIME" +
				"); " +
				"INSERT INTO transfer_configs SELECT * FROM transfer_configs_old; " +
				"DROP TABLE transfer_configs_old;").Error
		},
		Rollback: func(tx *gorm.DB) error {
			// No need to rollback as the data structure remains compatible
			return nil
		},
	}
}
