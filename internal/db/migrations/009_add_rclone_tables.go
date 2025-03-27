package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddRcloneTables adds tables for rclone commands and their flags
func AddRcloneTables() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "009_add_rclone_tables",
		Migrate: func(tx *gorm.DB) error {
			// Check if any tables exist (indicating an existing database)
			var count int64
			if err := tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
				return fmt.Errorf("failed to check for existing tables: %v", err)
			}

			// If tables exist, create a backup
			if count > 0 {
				// Get the database path
				sqlDB, err := tx.DB()
				if err != nil {
					return fmt.Errorf("failed to get underlying database: %v", err)
				}

				var seq int
				var name, dbPath string
				if err := sqlDB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); err != nil {
					return fmt.Errorf("failed to get database path: %v", err)
				}

				// Get backup directory from environment variable or use default
				backupDir := os.Getenv("BACKUP_DIR")
				if backupDir == "" {
					backupDir = "/app/backups" // Default Docker path
					// Check if we're not in Docker
					if _, err := os.Stat(backupDir); os.IsNotExist(err) {
						backupDir = "backups" // Fallback to local directory
					}
				}

				// Create backup directory if it doesn't exist
				if err := os.MkdirAll(backupDir, 0755); err != nil {
					return fmt.Errorf("failed to create backup directory: %v", err)
				}

				// Create backup file with timestamp in the backup directory
				dbFileName := filepath.Base(dbPath)
				backupFileName := fmt.Sprintf("%s.backup.%s", dbFileName, time.Now().Format("20060102_150405"))
				backupFile := filepath.Join(backupDir, backupFileName)

				// Read original database
				data, err := os.ReadFile(dbPath)
				if err != nil {
					return fmt.Errorf("failed to read database for backup: %v", err)
				}

				// Write backup
				if err := os.WriteFile(backupFile, data, 0600); err != nil {
					return fmt.Errorf("failed to create database backup: %v", err)
				}

				fmt.Printf("Created database backup at: %s\n", backupFile)
			}

			// Create the rclone_commands table
			err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS rclone_commands (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT NOT NULL,
					description TEXT NOT NULL,
					category TEXT NOT NULL,
					is_advanced BOOLEAN NOT NULL DEFAULT 0,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`).Error
			if err != nil {
				return err
			}

			// Create an index on name for faster lookups
			err = tx.Exec(`
				CREATE UNIQUE INDEX IF NOT EXISTS idx_rclone_commands_name ON rclone_commands(name)
			`).Error
			if err != nil {
				return err
			}

			// Create an index on category for faster filtering
			err = tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_rclone_commands_category ON rclone_commands(category)
			`).Error
			if err != nil {
				return err
			}

			// Create the rclone_command_flags table
			err = tx.Exec(`
				CREATE TABLE IF NOT EXISTS rclone_command_flags (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					command_id INTEGER NOT NULL,
					name TEXT NOT NULL,
					short_name TEXT,
					description TEXT NOT NULL,
					data_type TEXT NOT NULL,
					is_required BOOLEAN NOT NULL DEFAULT 0,
					default_value TEXT,
					created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (command_id) REFERENCES rclone_commands(id) ON DELETE CASCADE
				)
			`).Error
			if err != nil {
				return err
			}

			// Create index on command_id for faster lookups
			err = tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_rclone_command_flags_command_id ON rclone_command_flags(command_id)
			`).Error
			if err != nil {
				return err
			}

			// Create index on flag name for faster searches
			err = tx.Exec(`
				CREATE INDEX IF NOT EXISTS idx_rclone_command_flags_name ON rclone_command_flags(name)
			`).Error
			if err != nil {
				return err
			}

			// Populate the tables with default rclone commands and flags
			return populateRcloneTablesWithDefaults(tx)
		},
		Rollback: func(tx *gorm.DB) error {
			// Drop the flags table first due to foreign key constraints
			err := tx.Exec(`DROP TABLE IF EXISTS rclone_command_flags`).Error
			if err != nil {
				return err
			}

			// Then drop the commands table
			return tx.Exec(`DROP TABLE IF EXISTS rclone_commands`).Error
		},
	}
}

// Helper function to populate rclone tables with default data
func populateRcloneTablesWithDefaults(tx *gorm.DB) error {
	// Define base commands
	commands := []struct {
		Name        string
		Description string
		Category    string
		IsAdvanced  bool
	}{
		// Core commands
		{"copy", "Copy files from source to dest, skipping already copied", "sync", false},
		{"sync", "Make source and dest identical, modifying destination only", "sync", false},
		{"bisync", "Bidirectional synchronization between two paths", "sync", false},
		{"move", "Move files from source to dest", "sync", false},
		{"delete", "Remove the contents of path", "sync", false},
		{"purge", "Remove the path and all of its contents", "sync", false},
		{"mkdir", "Make the path if it doesn't already exist", "sync", false},
		{"rmdir", "Remove the path", "sync", false},
		{"rmdirs", "Remove any empty directories under the path", "sync", false},
		{"check", "Check if the files in the source and destination match", "sync", false},
		{"ls", "List all the objects in the path with size and path", "listing", false},
		{"lsd", "List all directories/containers/buckets in the path", "listing", false},
		{"lsl", "List all the objects in the path with size, modification time and path", "listing", false},
		{"lsf", "List the objects in the path (obey formatting parameters)", "listing", false},
		{"lsjson", "List the objects in the path in JSON format", "listing", false},
		{"md5sum", "Produce an md5sum file for all the objects in the path", "hash", false},
		{"sha1sum", "Produce a sha1sum file for all the objects in the path", "hash", false},
		{"size", "Return the total size and number of objects in remote:path", "info", false},
		{"version", "Show the version number", "info", false},
		{"cleanup", "Clean up the remote if possible", "maintenance", false},
		{"dedupe", "Interactively find duplicate files and delete/rename them", "maintenance", false},
		{"copyto", "Copy files from source to dest, skipping already copied", "sync", false},
		{"moveto", "Move file or directory from source to dest", "sync", false},
		{"listremotes", "List all the remotes in the config file", "config", false},
		{"obscure", "Obscure password for use in the rclone.conf", "config", false},
		{"cryptcheck", "Check the integrity of an encrypted remote", "crypto", false},
	}

	// Insert commands
	for _, cmd := range commands {
		err := tx.Exec(`
			INSERT INTO rclone_commands 
			(name, description, category, is_advanced, created_at) 
			VALUES (?, ?, ?, ?, ?)
		`, cmd.Name, cmd.Description, cmd.Category, cmd.IsAdvanced, time.Now()).Error

		if err != nil {
			return fmt.Errorf("failed to insert command %s: %v", cmd.Name, err)
		}
	}

	// Define all flags
	allFlags := []struct {
		CommandName  string
		Name         string
		ShortName    string
		Description  string
		DataType     string
		IsRequired   bool
		DefaultValue string
	}{
		// Global flags - apply to most commands
		{"global", "transfers", "n", "Number of file transfers to run in parallel", "int", false, "4"},
		{"global", "checkers", "", "Number of checkers to run in parallel", "int", false, "8"},
		{"global", "log-level", "", "Log level DEBUG|INFO|NOTICE|ERROR", "string", false, "NOTICE"},
		{"global", "stats", "", "Interval between logging stats, e.g 500ms, 60s, 5m", "string", false, "1m0s"},
		{"global", "stats-file-name-length", "", "Max file name length in stats", "int", false, "45"},
		{"global", "stats-one-line", "", "Make the stats fit on one line", "bool", false, "false"},
		{"global", "progress", "p", "Show progress during transfer", "bool", false, "false"},
		{"global", "verbose", "v", "Show verbose output", "bool", false, "false"},
		{"global", "quiet", "q", "Print as little stuff as possible", "bool", false, "false"},
		{"global", "retries", "", "Retry operations this many times if they fail", "int", false, "3"},
		{"global", "retries-sleep", "", "Interval between retrying operations if they fail", "string", false, "0s"},
		{"global", "timeout", "", "IO idle timeout", "string", false, "5m0s"},
		{"global", "tpslimit", "", "Limit HTTP transactions per second", "float", false, "0"},
		{"global", "tpslimit-burst", "", "Max burst of transactions for --tpslimit", "int", false, "1"},
		{"global", "size-only", "", "Skip based on size only, not mod-time or checksum", "bool", false, "false"},
		{"global", "ignore-checksum", "", "Skip post copy check of checksums", "bool", false, "false"},
		{"global", "ignore-existing", "", "Skip all files that exist on destination", "bool", false, "false"},
		{"global", "ignore-size", "", "Skip size checks to calculate if file changed", "bool", false, "false"},
		{"global", "ignore-case-sync", "", "Ignore case when synchronizing", "bool", false, "false"},
		{"global", "no-update-modtime", "", "Don't update destination mod-time if files identical", "bool", false, "false"},
		{"global", "no-check-certificate", "", "Do not verify server SSL certificates", "bool", false, "false"},
		{"global", "ask-password", "", "Allow prompt for password for encrypted config", "bool", false, "false"},
		{"global", "dump", "", "List of items to dump from: headers,bodies,requests,responses,auth,filters,goroutines,openfiles", "string", false, ""},
		{"global", "metadata", "M", "Preserve metadata when copying objects", "bool", false, "false"},
		{"global", "metadata-set", "", "Add metadata key=value when uploading", "string", false, ""},

		// copy command specific flags
		{"copy", "dry-run", "", "Do a trial run with no permanent changes", "bool", false, "false"},
		{"copy", "create-empty-src-dirs", "", "Create empty source dirs on destination", "bool", false, "false"},
		{"copy", "cutoff-mode", "", "Mode to stop transfers when reaching the cutoff threshold", "string", false, "hard"},
		{"copy", "max-transfer", "", "Maximum size of data to transfer", "string", false, "off"},
		{"copy", "max-backlog", "", "Maximum size of upload or download backlog", "int", false, "10000"},
		{"copy", "track-renames", "", "Track file renames during sync", "bool", false, "false"},
		{"copy", "track-renames-strategy", "", "Strategies to detect renames (hash|modtime|leaf)", "string", false, "hash"},

		// sync command specific flags
		{"sync", "dry-run", "", "Do a trial run with no permanent changes", "bool", false, "false"},
		{"sync", "create-empty-src-dirs", "", "Create empty source dirs on destination", "bool", false, "false"},
		{"sync", "backup-dir", "", "Make backups into this directory", "string", false, ""},
		{"sync", "suffix", "", "Suffix to add to changed files", "string", false, ""},
		{"sync", "delete-before", "", "Delete before transferring", "bool", false, "false"},
		{"sync", "delete-during", "", "Delete during transferring", "bool", false, "false"},
		{"sync", "delete-after", "", "Delete after transferring", "bool", false, "false"},
		{"sync", "track-renames", "", "Track file renames during sync", "bool", false, "false"},
		{"sync", "track-renames-strategy", "", "Strategies to detect renames (hash|modtime|leaf)", "string", false, "hash"},

		// bisync command specific flags
		{"bisync", "dry-run", "", "Do a trial run with no permanent changes", "bool", false, "false"},
		{"bisync", "resync", "", "Performs the resync run", "bool", false, "false"},
		{"bisync", "check-access", "", "Ensure destination has write access", "bool", false, "true"},
		{"bisync", "conflict-resolve", "", "Automatically resolve conflicts (newer|larger|older|smaller)", "string", false, ""},
		{"bisync", "max-delete", "", "Safety check on maximum files to delete", "int", false, "10"},

		// move command specific flags
		{"move", "dry-run", "", "Do a trial run with no permanent changes", "bool", false, "false"},
		{"move", "create-empty-src-dirs", "", "Create empty source dirs on destination", "bool", false, "false"},
		{"move", "delete-empty-src-dirs", "", "Delete empty source dirs after move", "bool", false, "false"},

		// ls/listing related flags
		{"ls", "recursive", "R", "Recurse into the listing", "bool", false, "false"},
		{"ls", "max-depth", "", "Maximum depth to recursively list", "int", false, ""},
		{"ls", "format", "", "Format for the output", "string", false, ""},
		{"ls", "absolute", "", "Put a leading / in front of path names", "bool", false, "false"},

		{"lsl", "recursive", "R", "Recurse into the listing", "bool", false, "false"},
		{"lsl", "max-depth", "", "Maximum depth to recursively list", "int", false, ""},

		{"lsd", "max-depth", "", "Maximum depth to show in the listing", "int", false, ""},
		{"lsd", "dir-sort", "", "Directory sorting (alphabetical|size|time)", "string", false, "alphabetical"},

		{"lsf", "format", "", "Format for the output", "string", false, ""},
		{"lsf", "recursive", "R", "Recurse into the listing", "bool", false, "false"},
		{"lsf", "max-depth", "", "Maximum depth to recursively list", "int", false, ""},

		{"lsjson", "recursive", "R", "Recurse into the listing", "bool", false, "false"},
		{"lsjson", "max-depth", "", "Maximum depth to recursively list", "int", false, ""},
		{"lsjson", "files-only", "", "Show only files, not directories", "bool", false, "false"},
		{"lsjson", "encrypted", "", "Show the encrypted names", "bool", false, "false"},
		{"lsjson", "stat", "", "Show status of objects", "bool", false, "false"},

		// Other specialized command flags
		{"cryptcheck", "one-way", "", "Check one way only, source files must exist on destination", "bool", false, "false"},

		{"cleanup", "dry-run", "", "Do a trial run with no permanent changes", "bool", false, "false"},

		{"dedupe", "dry-run", "", "Do a trial run with no permanent changes", "bool", false, "false"},
		{"dedupe", "mode", "", "Dedupe mode interactive|skip|first|newest|oldest|largest|smallest|rename", "string", false, "interactive"},
	}

	// Insert flags with associated command IDs
	for _, flag := range allFlags {
		// For global flags, add them to all commands
		if flag.CommandName == "global" {
			// Get all command IDs
			var commandIDs []int64
			err := tx.Raw(`SELECT id FROM rclone_commands`).Scan(&commandIDs).Error
			if err != nil {
				return fmt.Errorf("failed to get all command IDs: %v", err)
			}

			// Add global flag to each command
			for _, cmdID := range commandIDs {
				err = tx.Exec(`
					INSERT INTO rclone_command_flags 
					(command_id, name, short_name, description, data_type, is_required, default_value, created_at) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				`, cmdID, flag.Name, flag.ShortName, flag.Description, flag.DataType, flag.IsRequired, flag.DefaultValue, time.Now()).Error

				if err != nil {
					return fmt.Errorf("failed to insert global flag %s for command ID %d: %v", flag.Name, cmdID, err)
				}
			}
		} else {
			// Add command-specific flag
			var commandID int64
			err := tx.Raw(`SELECT id FROM rclone_commands WHERE name = ?`, flag.CommandName).Scan(&commandID).Error
			if err != nil {
				return fmt.Errorf("failed to find command %s: %v", flag.CommandName, err)
			}

			err = tx.Exec(`
				INSERT INTO rclone_command_flags 
				(command_id, name, short_name, description, data_type, is_required, default_value, created_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, commandID, flag.Name, flag.ShortName, flag.Description, flag.DataType, flag.IsRequired, flag.DefaultValue, time.Now()).Error

			if err != nil {
				return fmt.Errorf("failed to insert flag %s for command %s: %v", flag.Name, flag.CommandName, err)
			}
		}
	}

	return nil
}
