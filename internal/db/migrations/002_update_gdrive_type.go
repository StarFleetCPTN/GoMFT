package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// UpdateGDriveType updates the source_type and destination_type from 'google_drive' to 'gdrive'
func UpdateGDriveType() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "002_update_gdrive_type",
		Migrate: func(tx *gorm.DB) error {
			// Update source_type
			if err := tx.Exec(`UPDATE transfer_configs SET source_type = 'gdrive' WHERE source_type = 'google_drive'`).Error; err != nil {
				return err
			}

			// Update destination_type
			return tx.Exec(`UPDATE transfer_configs SET destination_type = 'gdrive' WHERE destination_type = 'google_drive'`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Revert source_type
			if err := tx.Exec(`UPDATE transfer_configs SET source_type = 'google_drive' WHERE source_type = 'gdrive'`).Error; err != nil {
				return err
			}

			// Revert destination_type
			return tx.Exec(`UPDATE transfer_configs SET destination_type = 'google_drive' WHERE destination_type = 'gdrive'`).Error
		},
	}
}
