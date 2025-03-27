package migrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddDefaultRoles creates a migration for adding default system roles
func AddDefaultRoles() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "005_add_default_roles",
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

			// Define default roles with their permissions
			defaultRoles := []struct {
				Name        string
				Description string
				Permissions []string
			}{
				{
					Name:        "admin",
					Description: "Full system administrator access",
					Permissions: []string{
						"users.view", "users.create", "users.edit", "users.delete",
						"roles.view", "roles.create", "roles.edit", "roles.delete",
						"configs.view", "configs.create", "configs.edit", "configs.delete",
						"jobs.view", "jobs.create", "jobs.edit", "jobs.delete", "jobs.run",
						"audit.view", "audit.export",
						"system.settings", "system.backup", "system.restore",
					},
				},
				{
					Name:        "system",
					Description: "System-level access for automated processes",
					Permissions: []string{
						"jobs.view", "jobs.create", "jobs.edit", "jobs.delete", "jobs.run",
						"configs.view", "configs.create", "configs.edit",
						"audit.view",
					},
				},
				{
					Name:        "user",
					Description: "Standard user access",
					Permissions: []string{
						"jobs.view", "jobs.run",
						"configs.view",
						"audit.view",
					},
				},
			}

			// Create each role
			for _, role := range defaultRoles {
				// Convert permissions to JSON string
				permsJSON, err := json.Marshal(role.Permissions)
				if err != nil {
					return err
				}

				// Check if role already exists
				var count int64
				if err := tx.Table("roles").Where("name = ?", role.Name).Count(&count).Error; err != nil {
					return err
				}

				// Skip if role already exists
				if count > 0 {
					continue
				}

				// Create the role
				now := time.Now()
				if err := tx.Exec(`
					INSERT INTO roles (name, description, permissions, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?)
				`, role.Name, role.Description, string(permsJSON), now, now).Error; err != nil {
					return err
				}

				// Create audit log entry with system user ID (1)
				details, err := json.Marshal(map[string]interface{}{
					"role_name":      role.Name,
					"permissions":    role.Permissions,
					"description":    role.Description,
					"system_created": true,
				})
				if err != nil {
					return err
				}

				if err := tx.Exec(`
					INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
					SELECT 'create_role', 'role', id, 1, ?, ?, ?, ?
					FROM roles WHERE name = ?
				`, string(details), now, now, now, role.Name).Error; err != nil {
					return err
				}
			}

			// Assign admin role to admin user (ID 1)
			now := time.Now()

			// Check if admin user exists
			var adminUserCount int64
			if err := tx.Table("users").Where("id = ?", 1).Count(&adminUserCount).Error; err != nil {
				return err
			}
			if adminUserCount == 0 {
				return nil // Skip if admin user doesn't exist yet
			}

			// Get admin role ID
			var adminRoleID uint
			if err := tx.Table("roles").Select("id").Where("name = ?", "admin").Row().Scan(&adminRoleID); err != nil {
				return err
			}

			// Check if role is already assigned
			var assignmentCount int64
			if err := tx.Table("user_roles").Where("user_id = ? AND role_id = ?", 1, adminRoleID).Count(&assignmentCount).Error; err != nil {
				return err
			}
			if assignmentCount > 0 {
				return nil // Skip if already assigned
			}

			// Assign admin role to admin user
			if err := tx.Exec(`
				INSERT INTO user_roles (user_id, role_id, created_at, updated_at)
				VALUES (?, ?, ?, ?)
			`, 1, adminRoleID, now, now).Error; err != nil {
				return err
			}

			// Create audit log for role assignment
			details, err := json.Marshal(map[string]interface{}{
				"role_name":      "admin",
				"user_id":        1,
				"system_created": true,
			})
			if err != nil {
				return err
			}

			return tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('assign_role', 'role', ?, 1, ?, ?, ?, ?)
			`, adminRoleID, string(details), now, now, now).Error

		},
		Rollback: func(tx *gorm.DB) error {
			// Don't delete the roles on rollback as they might be in use
			// Instead, log that we're skipping role deletion
			now := time.Now()
			details, err := json.Marshal(map[string]interface{}{
				"message": "Skipped deletion of default roles for safety",
			})
			if err != nil {
				return err
			}

			return tx.Exec(`
				INSERT INTO audit_logs (action, entity_type, entity_id, user_id, details, created_at, updated_at, timestamp)
				VALUES ('migration_rollback', 'roles', 0, 1, ?, ?, ?, ?)
			`, string(details), now, now, now).Error
		},
	}
}
