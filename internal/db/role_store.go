package db

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// --- Role Store Methods ---

// CreateRole creates a new role record
func (db *DB) CreateRole(role *Role) error {
	return db.Create(role).Error
}

// GetRole retrieves a role by ID, preloading permissions
func (db *DB) GetRole(id uint) (*Role, error) {
	var role Role
	// Assuming Permissions are handled correctly by GORM or custom type
	err := db.First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetRoleByName retrieves a role by name, preloading permissions
func (db *DB) GetRoleByName(name string) (*Role, error) {
	var role Role
	err := db.Where("name = ?", name).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// UpdateRole updates an existing role record
func (db *DB) UpdateRole(role *Role) error {
	// Use Omit Users to prevent GORM from trying to update the many2many relationship directly here
	return db.Omit("Users").Save(role).Error
}

// DeleteRole deletes a role after checking dependencies and removing assignments
func (db *DB) DeleteRole(id uint) error {
	var role Role
	if err := db.First(&role, id).Error; err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	if role.IsSystemRole() {
		return errors.New("cannot delete system role")
	}

	// Start transaction
	tx := db.Begin()
	if err := tx.Error; err != nil {
		return err
	}

	// Manually delete role assignments from the join table
	if err := tx.Exec("DELETE FROM user_roles WHERE role_id = ?", id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete role assignments: %w", err)
	}

	// Delete the role itself
	if err := tx.Delete(&role).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete role: %w", err)
	}

	// Commit transaction
	return tx.Commit().Error
}

// ListRoles retrieves all roles
func (db *DB) ListRoles() ([]Role, error) {
	var roles []Role
	err := db.Find(&roles).Error
	return roles, err
}

// GetUserRoles retrieves all roles assigned to a specific user ID
func (db *DB) GetUserRoles(userID uint) ([]Role, error) {
	var user User
	// Preload the Roles association
	if err := db.Preload("Roles").First(&user, userID).Error; err != nil {
		// Handle case where user might not be found vs. other errors
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user with ID %d not found", userID)
		}
		return nil, fmt.Errorf("failed to get user %d roles: %w", userID, err)
	}
	return user.Roles, nil
}

// AssignRoleToUser assigns a role to a user, handling the join table
func (db *DB) AssignRoleToUser(roleID, userID, assignedByID uint) error {
	var role Role
	if err := db.First(&role, roleID).Error; err != nil {
		return fmt.Errorf("role with ID %d not found: %w", roleID, err)
	}
	var user User
	if err := db.First(&user, userID).Error; err != nil {
		return fmt.Errorf("user with ID %d not found: %w", userID, err)
	}

	// Use GORM's Association API for many2many
	err := db.Model(&user).Association("Roles").Append(&role)
	if err != nil {
		return fmt.Errorf("failed to assign role %d to user %d: %w", roleID, userID, err)
	}

	// Optionally, log the assignment (consider moving audit logging to a dedicated service/hook)
	// db.Create(&AuditLog{...})

	return nil
}

// UnassignRoleFromUser removes a role from a user, handling the join table
func (db *DB) UnassignRoleFromUser(roleID, userID, unassignedByID uint) error {
	var role Role
	if err := db.First(&role, roleID).Error; err != nil {
		return fmt.Errorf("role with ID %d not found: %w", roleID, err)
	}
	var user User
	// Need to preload roles to check if the association exists before deleting
	if err := db.Preload("Roles").First(&user, userID).Error; err != nil {
		return fmt.Errorf("user with ID %d not found: %w", userID, err)
	}

	// Use GORM's Association API for many2many deletion
	err := db.Model(&user).Association("Roles").Delete(&role)
	if err != nil {
		return fmt.Errorf("failed to unassign role %d from user %d: %w", roleID, userID, err)
	}

	// Optionally, log the unassignment
	// db.Create(&AuditLog{...})

	return nil
}
