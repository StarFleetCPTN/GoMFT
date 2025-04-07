package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// CleanupInvalidBooleans updates boolean columns represented as integers
// to ensure they only contain valid values (0, 1, or NULL).
func CleanupInvalidBooleans() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "013_cleanup_invalid_booleans",
		Migrate: func(tx *gorm.DB) error {
			fmt.Println("Running migration 013: Cleaning up invalid boolean values...")

			// Target: transfer_configs.delete_after_transfer
			// Set any non-NULL value that is not 0 or 1 to 0 (false)
			sql := `UPDATE transfer_configs
					SET delete_after_transfer = 0
					WHERE delete_after_transfer IS NOT NULL AND delete_after_transfer NOT IN (0, 1);`

			if err := tx.Exec(sql).Error; err != nil {
				return fmt.Errorf("failed to cleanup delete_after_transfer in transfer_configs: %w", err)
			}
			fmt.Println("Cleaned up invalid values in transfer_configs.delete_after_transfer.")

			// Target: transfer_configs.archive_enabled
			sql = `UPDATE transfer_configs
					SET archive_enabled = 0
					WHERE archive_enabled IS NOT NULL AND archive_enabled NOT IN (0, 1);`
			if err := tx.Exec(sql).Error; err != nil {
				return fmt.Errorf("failed to cleanup archive_enabled in transfer_configs: %w", err)
			}
			fmt.Println("Cleaned up invalid values in transfer_configs.archive_enabled.")

			// Target: transfer_configs.skip_processed_files
			sql = `UPDATE transfer_configs
					SET skip_processed_files = 0
					WHERE skip_processed_files IS NOT NULL AND skip_processed_files NOT IN (0, 1);`
			if err := tx.Exec(sql).Error; err != nil {
				return fmt.Errorf("failed to cleanup skip_processed_files in transfer_configs: %w", err)
			}
			fmt.Println("Cleaned up invalid values in transfer_configs.skip_processed_files.")

			// Target: notification_services.is_enabled
			sql = `UPDATE notification_services
					SET is_enabled = 0
					WHERE is_enabled IS NOT NULL AND is_enabled NOT IN (0, 1);`
			if err := tx.Exec(sql).Error; err != nil {
				// Check if the table exists before failing hard
				var tableExists int
				tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='notification_services'").Scan(&tableExists)
				if tableExists == 0 {
					fmt.Println("Skipping cleanup for notification_services.is_enabled: table does not exist.")
				} else {
					return fmt.Errorf("failed to cleanup is_enabled in notification_services: %w", err)
				}
			} else {
				fmt.Println("Cleaned up invalid values in notification_services.is_enabled.")
			}

			// Target: auth_providers.enabled
			sql = `UPDATE auth_providers
					SET enabled = 0
					WHERE enabled IS NOT NULL AND enabled NOT IN (0, 1);`
			if err := tx.Exec(sql).Error; err != nil {
				// Check if the table exists before failing hard
				var tableExists int
				tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='auth_providers'").Scan(&tableExists)
				if tableExists == 0 {
					fmt.Println("Skipping cleanup for auth_providers.enabled: table does not exist.")
				} else {
					return fmt.Errorf("failed to cleanup enabled in auth_providers: %w", err)
				}
			} else {
				fmt.Println("Cleaned up invalid values in auth_providers.enabled.")
			}

			fmt.Println("Migration 013 completed successfully.")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// This migration cleans up data. Rolling back doesn't make sense
			// as we don't know the original invalid values.
			fmt.Println("Rollback for migration 013_cleanup_invalid_booleans is not applicable.")
			return nil
		},
	}
}
