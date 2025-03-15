package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddWebhookSupport adds webhook notification fields to the jobs table
func AddWebhookSupport() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20240618_add_webhook_support",
		Migrate: func(tx *gorm.DB) error {
			// Add webhook URL field
			if err := tx.Exec("ALTER TABLE jobs ADD COLUMN webhook_enabled BOOLEAN DEFAULT false").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs ADD COLUMN webhook_url VARCHAR(255)").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs ADD COLUMN webhook_secret VARCHAR(255)").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs ADD COLUMN webhook_headers TEXT").Error; err != nil {
				return err
			}

			// Add notification settings
			if err := tx.Exec("ALTER TABLE jobs ADD COLUMN notify_on_success BOOLEAN DEFAULT true").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs ADD COLUMN notify_on_failure BOOLEAN DEFAULT true").Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop the webhook fields from jobs
			if err := tx.Exec("ALTER TABLE jobs DROP COLUMN webhook_enabled").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs DROP COLUMN webhook_url").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs DROP COLUMN webhook_secret").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs DROP COLUMN webhook_headers").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs DROP COLUMN notify_on_success").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE jobs DROP COLUMN notify_on_failure").Error; err != nil {
				return err
			}

			return nil
		},
	}
}
