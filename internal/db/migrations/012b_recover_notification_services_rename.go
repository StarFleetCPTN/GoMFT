package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// RecoverNotificationServicesRename checks for and corrects a specific inconsistent state
// left by a potentially failed run of migration 012, where the notification_services
// table might have been left renamed as _notification_services_old.
func RecoverNotificationServicesRename() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "012b_recover_notification_services_rename",
		Migrate: func(tx *gorm.DB) error {
			fmt.Println("Running migration 012b: Checking for notification_services rename recovery...")

			var oldTableExists int
			tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_notification_services_old'").Scan(&oldTableExists)

			var newTableExists int
			tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='notification_services'").Scan(&newTableExists)

			if oldTableExists > 0 && newTableExists == 0 {
				fmt.Println("Found _notification_services_old table but not notification_services. Attempting recovery rename...")
				if err := tx.Exec("ALTER TABLE _notification_services_old RENAME TO notification_services").Error; err != nil {
					return fmt.Errorf("failed to rename _notification_services_old back to notification_services: %w", err)
				}
				fmt.Println("Successfully renamed _notification_services_old to notification_services.")
			} else if oldTableExists > 0 && newTableExists > 0 {
				fmt.Println("Warning: Both notification_services and _notification_services_old tables exist. Manual inspection might be needed.")
			} else {
				fmt.Println("No recovery needed for notification_services rename.")
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Rollback doesn't make sense for a recovery step.
			fmt.Println("Rollback for migration 012b_recover_notification_services_rename is not applicable.")
			return nil
		},
	}
}
