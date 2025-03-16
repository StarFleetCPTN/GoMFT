package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddGooglePhotosSupport adds Google Photos related fields to the transfer_configs table
func AddGooglePhotosSupport() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20240518_add_google_photos_support",
		Migrate: func(tx *gorm.DB) error {
			// Add Google Photos source fields
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN source_read_only BOOLEAN DEFAULT false").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN source_start_year INTEGER DEFAULT 0").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN source_include_archived BOOLEAN DEFAULT false").Error; err != nil {
				return err
			}

			// Add Google Photos destination fields
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN dest_read_only BOOLEAN DEFAULT false").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN dest_start_year INTEGER DEFAULT 0").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs ADD COLUMN dest_include_archived BOOLEAN DEFAULT false").Error; err != nil {
				return err
			}

			// Add OAuth field
			return tx.Exec("ALTER TABLE transfer_configs ADD COLUMN use_builtin_auth BOOLEAN DEFAULT true").Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Remove all added columns in reverse order
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN use_builtin_auth").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN dest_include_archived").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN dest_start_year").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN dest_read_only").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN source_include_archived").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE transfer_configs DROP COLUMN source_start_year").Error; err != nil {
				return err
			}
			return tx.Exec("ALTER TABLE transfer_configs DROP COLUMN source_read_only").Error
		},
	}
}
