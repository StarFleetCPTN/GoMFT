package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddMultiConfigSupport adds support for multiple configurations per job
func AddMultiConfigSupport() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20250315_add_multi_config_support",
		Migrate: func(tx *gorm.DB) error {
			// Add config_ids column to jobs table
			if err := tx.Exec("ALTER TABLE jobs ADD COLUMN config_ids TEXT").Error; err != nil {
				return err
			}

			// Add config_id column to job_histories table
			if err := tx.Exec("ALTER TABLE job_histories ADD COLUMN config_id INTEGER").Error; err != nil {
				return err
			}

			// Add config_id column to file_metadata table
			if err := tx.Exec("ALTER TABLE file_metadata ADD COLUMN config_id INTEGER").Error; err != nil {
				return err
			}

			// Update existing jobs to set the config_ids field to match the current config_id
			if err := tx.Exec("UPDATE jobs SET config_ids = config_id WHERE config_id > 0").Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop the config_id columns from job_histories and file_metadata
			if err := tx.Exec("ALTER TABLE job_histories DROP COLUMN config_id").Error; err != nil {
				return err
			}

			if err := tx.Exec("ALTER TABLE file_metadata DROP COLUMN config_id").Error; err != nil {
				return err
			}

			// Drop the config_ids column from jobs
			return tx.Exec("ALTER TABLE jobs DROP COLUMN config_ids").Error
		},
	}
}
