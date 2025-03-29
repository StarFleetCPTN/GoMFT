package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/db/migrations"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

func Initialize(dbPath string) (*DB, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	// Open database connection with modernc.org/sqlite driver
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Initialize and run migrations
	m := migrations.GetMigrations(db)
	if err := m.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return &DB{DB: db}, nil
}

// ReopenWithoutMigrations reopens the database connection without running migrations
// This should be used when temporarily closing and reopening the database
func ReopenWithoutMigrations(dbPath string) (*DB, error) {
	// Open database connection with modernc.org/sqlite driver
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	return &DB{DB: db}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetEnabledAuthProviders returns all enabled authentication providers
// func (db *DB) GetEnabledAuthProviders(ctx context.Context) ([]AuthProvider, error) {
// 	var providers []AuthProvider
// 	result := db.WithContext(ctx).Where("enabled = ?", true).Find(&providers)
// 	return providers, result.Error
// }
