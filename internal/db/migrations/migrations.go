package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// InitMigrations initializes the migrations
func InitMigrations(db *gorm.DB) *gormigrate.Gormigrate {
	migrations := []*gormigrate.Migration{
		// ... existing migrations
		AddDeleteAfterTransferColumn(),
		AddCloudStorageFields(),
		AddSkipProcessedFilesColumn(),
		AddMaxConcurrentTransfersColumn(),
		AddMultiConfigSupport(),
		UpdateSkipProcessedFilesToNullable(),
		AddWebhookSupport(),
		AddGoogleDriveAuthenticated(),
		AddGooglePhotosSupport(),
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, migrations)
}
