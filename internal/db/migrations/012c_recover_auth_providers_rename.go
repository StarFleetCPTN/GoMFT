package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// RecoverAuthProvidersRename checks for and corrects a specific inconsistent state
// left by a potentially failed run of migration 012, where the auth_providers
// table might have been left renamed as _auth_providers_old.
func RecoverAuthProvidersRename() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "012c_recover_auth_providers_rename",
		Migrate: func(tx *gorm.DB) error {
			fmt.Println("Running migration 012c: Checking for auth_providers rename recovery...")

			var oldTableExists int
			tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_auth_providers_old'").Scan(&oldTableExists)

			var newTableExists int
			tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='auth_providers'").Scan(&newTableExists)

			if oldTableExists > 0 && newTableExists == 0 {
				fmt.Println("Found _auth_providers_old table but not auth_providers. Attempting recovery rename...")
				if err := tx.Exec("ALTER TABLE _auth_providers_old RENAME TO auth_providers").Error; err != nil {
					return fmt.Errorf("failed to rename _auth_providers_old back to auth_providers: %w", err)
				}
				fmt.Println("Successfully renamed _auth_providers_old to auth_providers.")
			} else if oldTableExists > 0 && newTableExists > 0 {
				fmt.Println("Warning: Both auth_providers and _auth_providers_old tables exist. Manual inspection might be needed.")
			} else {
				fmt.Println("No recovery needed for auth_providers rename.")
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Rollback doesn't make sense for a recovery step.
			fmt.Println("Rollback for migration 012c_recover_auth_providers_rename is not applicable.")
			return nil
		},
	}
}
