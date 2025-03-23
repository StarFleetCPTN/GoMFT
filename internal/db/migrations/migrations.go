package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var migrations []*gormigrate.Migration

// GetMigrations returns all migrations
func GetMigrations(db *gorm.DB) *gormigrate.Gormigrate {
	// Add all migrations in order
	migrations = append(migrations,
		InitialSchema(),               // 001
		UpdateGDriveType(),            // 002
		Add2FA(),                      // 003
		AddAuditLogs(),                // 004
		AddDefaultRoles(),             // 005
		AddTimestampsToJobHistories(), // 006
		AddNotificationServices(),     // 007
		AddUserNotifications(),        // 008
		AddRcloneTables(),             // 009
		AddRcloneCommandToConfig(),    // 010
	)

	return gormigrate.New(db, gormigrate.DefaultOptions, migrations)
}
