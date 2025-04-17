package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddProviderRefsToTransferConfig adds the storage provider reference fields to the transfer_configs table
func AddProviderRefsToTransferConfig() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "015_add_provider_refs_to_transfer_config",
		Migrate: func(tx *gorm.DB) error {
			// Add source_provider_id and destination_provider_id columns to transfer_configs table
			if err := tx.Exec(`ALTER TABLE transfer_configs ADD COLUMN source_provider_id INTEGER REFERENCES storage_providers(id)`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`ALTER TABLE transfer_configs ADD COLUMN destination_provider_id INTEGER REFERENCES storage_providers(id)`).Error; err != nil {
				return err
			}

			// Create indexes for better performance when joining with the storage_providers table
			if err := tx.Exec(`CREATE INDEX idx_transfer_configs_source_provider_id ON transfer_configs(source_provider_id)`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`CREATE INDEX idx_transfer_configs_destination_provider_id ON transfer_configs(destination_provider_id)`).Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop indexes first
			if err := tx.Exec(`DROP INDEX IF EXISTS idx_transfer_configs_source_provider_id`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`DROP INDEX IF EXISTS idx_transfer_configs_destination_provider_id`).Error; err != nil {
				return err
			}

			// Remove columns
			if err := tx.Exec(`ALTER TABLE transfer_configs DROP COLUMN source_provider_id`).Error; err != nil {
				return err
			}

			if err := tx.Exec(`ALTER TABLE transfer_configs DROP COLUMN destination_provider_id`).Error; err != nil {
				return err
			}

			return nil
		},
	}
}
