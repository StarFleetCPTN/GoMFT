package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddStorageProviders adds the storage_providers table
func AddStorageProviders() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "014_add_storage_providers",
		Migrate: func(tx *gorm.DB) error {
			// Create the storage_providers table
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS storage_providers (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name VARCHAR(255) NOT NULL,
				type VARCHAR(50) NOT NULL,
				host VARCHAR(255),
				port INTEGER DEFAULT 22,
				username VARCHAR(255),
				encrypted_password TEXT,
				key_file TEXT,
				bucket VARCHAR(255),
				region VARCHAR(255),
				access_key VARCHAR(255),
				encrypted_secret_key TEXT,
				endpoint VARCHAR(255),
				share VARCHAR(255),
				domain VARCHAR(255),
				passive_mode BOOLEAN DEFAULT TRUE,
				client_id VARCHAR(255),
				encrypted_client_secret TEXT,
				encrypted_refresh_token TEXT,
				drive_id VARCHAR(255),
				team_drive VARCHAR(255),
				read_only BOOLEAN DEFAULT FALSE,
				start_year INTEGER,
				include_archived BOOLEAN DEFAULT FALSE,
				use_builtin_auth BOOLEAN DEFAULT TRUE,
				authenticated BOOLEAN DEFAULT FALSE,
				created_by INTEGER NOT NULL,
				created_at DATETIME,
				updated_at DATETIME,
				FOREIGN KEY (created_by) REFERENCES users(id),
				UNIQUE(name, created_by)
			)`).Error; err != nil {
				return err
			}

			// Create index on type for faster filtering
			if err := tx.Exec(`CREATE INDEX idx_storage_providers_type ON storage_providers(type)`).Error; err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop the storage_providers table
			if err := tx.Exec(`DROP TABLE IF EXISTS storage_providers`).Error; err != nil {
				return err
			}

			return nil
		},
	}
}
