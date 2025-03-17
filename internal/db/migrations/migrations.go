package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// InitMigrations initializes the migrations
func InitMigrations(db *gorm.DB) *gormigrate.Gormigrate {
	migrations := []*gormigrate.Migration{
		InitialSchema(),
		UpdateGDriveType(),
		Add2FA(),
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, migrations)
}
