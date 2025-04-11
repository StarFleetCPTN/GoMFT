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

	// Close the database connection after migrations
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		return nil, fmt.Errorf("failed to close database after migrations: %v", err)
	}

	// Reopen the database connection for a clean state
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to reconnect to database after migrations: %v", err)
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
