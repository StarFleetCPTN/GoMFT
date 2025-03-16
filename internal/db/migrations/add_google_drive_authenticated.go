package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddGoogleDriveAuthenticated adds the GoogleDriveAuthenticated field to the transfer_configs table
func AddGoogleDriveAuthenticated() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20240315_add_google_drive_authenticated",
		Migrate: func(tx *gorm.DB) error {
			// Add the GoogleDriveAuthenticated column with a default value of false
			return tx.Exec("ALTER TABLE transfer_configs ADD COLUMN google_drive_authenticated BOOLEAN DEFAULT false").Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Remove the GoogleDriveAuthenticated column
			return tx.Exec("ALTER TABLE transfer_configs DROP COLUMN google_drive_authenticated").Error
		},
	}
}
