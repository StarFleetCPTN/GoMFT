package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// UpdateBuiltinAuthFields updates the use_builtin_auth field to separate source and destination fields
func UpdateBuiltinAuthFields() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "002_update_builtin_auth_fields",
		Migrate: func(tx *gorm.DB) error {
			// First, add the new columns
			if err := tx.Exec(`ALTER TABLE transfer_configs ADD COLUMN use_builtin_auth_source BOOLEAN`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`ALTER TABLE transfer_configs ADD COLUMN use_builtin_auth_dest BOOLEAN`).Error; err != nil {
				return err
			}

			// Copy the old value to both new columns
			if err := tx.Exec(`UPDATE transfer_configs SET 
				use_builtin_auth_source = use_builtin_auth,
				use_builtin_auth_dest = use_builtin_auth`).Error; err != nil {
				return err
			}

			// Drop the old column
			return tx.Exec(`ALTER TABLE transfer_configs DROP COLUMN use_builtin_auth`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Add back the original column
			if err := tx.Exec(`ALTER TABLE transfer_configs ADD COLUMN use_builtin_auth BOOLEAN`).Error; err != nil {
				return err
			}

			// Copy the source value back (could also use dest, they should be the same)
			if err := tx.Exec(`UPDATE transfer_configs SET use_builtin_auth = use_builtin_auth_source`).Error; err != nil {
				return err
			}

			// Drop the new columns
			if err := tx.Exec(`ALTER TABLE transfer_configs DROP COLUMN use_builtin_auth_source`).Error; err != nil {
				return err
			}
			return tx.Exec(`ALTER TABLE transfer_configs DROP COLUMN use_builtin_auth_dest`).Error
		},
	}
}
