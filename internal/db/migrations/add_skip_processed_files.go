package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddSkipProcessedFilesColumn adds the skip_processed_files column to transfer_configs table
func AddSkipProcessedFilesColumn() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20250310_add_skip_processed_files",
		Migrate: func(tx *gorm.DB) error {
			// Add skip_processed_files column with default value of true
			return tx.Exec("ALTER TABLE transfer_configs ADD COLUMN skip_processed_files BOOLEAN DEFAULT true").Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop the column if needed
			return tx.Exec("ALTER TABLE transfer_configs DROP COLUMN skip_processed_files").Error
		},
	}
}
