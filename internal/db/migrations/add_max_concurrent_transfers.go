package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddMaxConcurrentTransfersColumn adds the max_concurrent_transfers column to transfer_configs table
func AddMaxConcurrentTransfersColumn() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20250311_add_max_concurrent_transfers",
		Migrate: func(tx *gorm.DB) error {
			// Add max_concurrent_transfers column with default value of 4
			return tx.Exec("ALTER TABLE transfer_configs ADD COLUMN max_concurrent_transfers INTEGER DEFAULT 4").Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop the column if needed
			return tx.Exec("ALTER TABLE transfer_configs DROP COLUMN max_concurrent_transfers").Error
		},
	}
}
