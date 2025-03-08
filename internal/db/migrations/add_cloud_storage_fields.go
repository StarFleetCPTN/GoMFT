package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddCloudStorageFields adds fields for WebDAV, NextCloud, OneDrive, and Google Drive
func AddCloudStorageFields() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "add_cloud_storage_fields",
		Migrate: func(tx *gorm.DB) error {
			// Add source fields
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN source_client_id VARCHAR(255)").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN source_drive_id VARCHAR(255)").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN source_team_drive VARCHAR(255)").Error; err != nil {
				return err
			}
			
			// Add destination fields
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN dest_client_id VARCHAR(255)").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN dest_drive_id VARCHAR(255)").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN dest_team_drive VARCHAR(255)").Error; err != nil {
				return err
			}
			
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop source fields
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN source_client_id").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN source_drive_id").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN source_team_drive").Error; err != nil {
				return err
			}
			
			// Drop destination fields
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN dest_client_id").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN dest_drive_id").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN dest_team_drive").Error; err != nil {
				return err
			}
			
			return nil
		},
	}
} 