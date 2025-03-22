package migrations

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddTimestampsToJobHistories creates a migration for adding timestamp columns to job_histories table
func AddTimestampsToJobHistories() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "006_add_timestamps_to_job_histories",
		Migrate: func(tx *gorm.DB) error {
			// Add created_at column
			if err := tx.Exec("ALTER TABLE job_histories ADD COLUMN created_at DATETIME").Error; err != nil {
				return fmt.Errorf("failed to add created_at column: %v", err)
			}

			// Add updated_at column
			if err := tx.Exec("ALTER TABLE job_histories ADD COLUMN updated_at DATETIME").Error; err != nil {
				return fmt.Errorf("failed to add updated_at column: %v", err)
			}

			// Set default values for existing records
			now := time.Now()
			if err := tx.Exec("UPDATE job_histories SET created_at = ?, updated_at = ? WHERE created_at IS NULL", now, now).Error; err != nil {
				return fmt.Errorf("failed to update existing records: %v", err)
			}

			// Create audit log entry for the migration
			details, err := json.Marshal(map[string]interface{}{
				"columns_added": []string{"created_at", "updated_at"},
				"table":         "job_histories",
			})
			if err != nil {
				return err
			}

			return tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('schema_update', 'job_histories', 0, 1, ?, ?, ?, ?)
			`, string(details), now, now, now).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// In SQLite, we can't drop columns directly, so we'd need to:
			// 1. Create a new table without the columns
			// 2. Copy data from the old table
			// 3. Drop the old table
			// 4. Rename the new table
			//
			// However, keeping the timestamp columns is generally harmless,
			// so we'll just log a message instead of performing a risky rollback

			now := time.Now()
			details, err := json.Marshal(map[string]interface{}{
				"message": "Skipped removal of timestamp columns from job_histories due to SQLite limitations",
			})
			if err != nil {
				return err
			}

			return tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('migration_rollback', 'job_histories', 0, 1, ?, ?, ?, ?)
			`, string(details), now, now, now).Error
		},
	}
}
