package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/db/middleware"
	"github.com/starfleetcptn/gomft/internal/db/migrations"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
	encryptionMiddleware *middleware.EncryptionMiddleware
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

	// Initialize and register the encryption middleware
	encryptionMiddleware, err := middleware.NewEncryptionMiddleware()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryption middleware: %v", err)
	}
	encryptionMiddleware.RegisterHooks(db)

	return &DB{
		DB:                   db,
		encryptionMiddleware: encryptionMiddleware,
	}, nil
}

// ReopenWithoutMigrations reopens the database connection without running migrations
// This should be used when temporarily closing and reopening the database
func ReopenWithoutMigrations(dbPath string) (*DB, error) {
	// Open database connection with modernc.org/sqlite driver
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Initialize and register the encryption middleware
	encryptionMiddleware, err := middleware.NewEncryptionMiddleware()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryption middleware: %v", err)
	}
	encryptionMiddleware.RegisterHooks(db)

	return &DB{
		DB:                   db,
		encryptionMiddleware: encryptionMiddleware,
	}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// EnableEncryption enables the encryption middleware
func (db *DB) EnableEncryption() {
	if db.encryptionMiddleware != nil {
		db.encryptionMiddleware.Enable()
	}
}

// DisableEncryption disables the encryption middleware
func (db *DB) DisableEncryption() {
	if db.encryptionMiddleware != nil {
		db.encryptionMiddleware.Disable()
	}
}

// IsEncryptionEnabled returns whether the encryption middleware is enabled
func (db *DB) IsEncryptionEnabled() bool {
	if db.encryptionMiddleware != nil {
		return db.encryptionMiddleware.IsEnabled()
	}
	return false
}

// Connect initializes a database connection using the default path
// This is used by CLI commands to connect to the database
func Connect() (*DB, error) {
	// Get data directory from environment or use default
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Use default database path
	dbPath := filepath.Join(dataDir, "gomft.db")

	// Initialize the database
	return Initialize(dbPath)
}
