package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// RecoverTransferConfigsRename checks for and corrects a specific inconsistent state
// left by a potentially failed run of migration 012, where the transfer_configs
// table might have been left renamed as _transfer_configs_old.
func RecoverTransferConfigsRename() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "011a_recover_transfer_configs_rename",
		Migrate: func(tx *gorm.DB) error {
			fmt.Println("Running migration 011a: Checking for transfer_configs rename recovery...")

			var oldTableExists int
			tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_transfer_configs_old'").Scan(&oldTableExists)

			var newTableExists int
			tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='transfer_configs'").Scan(&newTableExists)

			if oldTableExists > 0 && newTableExists == 0 {
				fmt.Println("Found _transfer_configs_old table but not transfer_configs. Attempting recovery rename...")
				if err := tx.Exec("ALTER TABLE _transfer_configs_old RENAME TO transfer_configs").Error; err != nil {
					return fmt.Errorf("failed to rename _transfer_configs_old back to transfer_configs: %w", err)
				}
				fmt.Println("Successfully renamed _transfer_configs_old to transfer_configs.")
			} else if oldTableExists > 0 && newTableExists > 0 {
				// This state shouldn't ideally happen if migration 012 followed its logic,
				// but indicates a potential issue. Maybe drop the old one? For now, just log.
				fmt.Println("Warning: Both transfer_configs and _transfer_configs_old tables exist. Manual inspection might be needed.")
			} else {
				fmt.Println("No recovery needed for transfer_configs rename.")
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Rollback doesn't make sense for a recovery step.
			fmt.Println("Rollback for migration 011a_recover_transfer_configs_rename is not applicable.")
			return nil
		},
	}
}
