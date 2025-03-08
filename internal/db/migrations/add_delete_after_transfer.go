package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddDeleteAfterTransferColumn adds the delete_after_transfer column to transfer_configs table
func AddDeleteAfterTransferColumn() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "add_delete_after_transfer_column",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec("ALTER TABLE transfer_configs ADD COLUMN delete_after_transfer BOOLEAN NOT NULL DEFAULT false").Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec("ALTER TABLE transfer_configs DROP COLUMN delete_after_transfer").Error
		},
	}
} 