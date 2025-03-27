package db

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Role represents a role in the system with associated permissions
type Role struct {
	gorm.Model
	Name        string      `gorm:"size:255;not null;unique"`
	Description string      `gorm:"type:text"`
	Permissions Permissions `gorm:"type:text"`
	Users       []User      `gorm:"many2many:user_roles;"`
}

// AssignToUser assigns this role to a user
func (r *Role) AssignToUser(tx *gorm.DB, userID uint, assignedByID uint) error {
	// Check if the role is already assigned
	var count int64
	if err := tx.Table("user_roles").Where("user_id = ? AND role_id = ?", userID, r.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("role is already assigned to user")
	}

	// Create the assignment
	if err := tx.Exec("INSERT INTO user_roles (user_id, role_id, created_at, updated_at) VALUES (?, ?, ?, ?)",
		userID, r.ID, time.Now(), time.Now()).Error; err != nil {
		return err
	}

	// Create audit log
	return tx.Create(&AuditLog{
		Action:     "assign_role",
		EntityType: "role",
		EntityID:   r.ID,
		UserID:     assignedByID,
		Details: map[string]interface{}{
			"role_name": r.Name,
			"user_id":   userID,
		},
		Timestamp: time.Now(),
	}).Error
}

// UnassignFromUser removes this role from a user
func (r *Role) UnassignFromUser(tx *gorm.DB, userID uint, unassignedByID uint) error {
	// Check if the role is assigned
	var count int64
	if err := tx.Table("user_roles").Where("user_id = ? AND role_id = ?", userID, r.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("role is not assigned to user")
	}

	// Remove the assignment
	if err := tx.Exec("DELETE FROM user_roles WHERE user_id = ? AND role_id = ?", userID, r.ID).Error; err != nil {
		return err
	}

	// Create audit log
	return tx.Create(&AuditLog{
		Action:     "unassign_role",
		EntityType: "role",
		EntityID:   r.ID,
		UserID:     unassignedByID,
		Details: map[string]interface{}{
			"role_name": r.Name,
			"user_id":   userID,
		},
		Timestamp: time.Now(),
	}).Error
}

// Validate performs validation on the role
func (r *Role) Validate() error {
	// Name validation
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("role name cannot be empty")
	}
	if len(r.Name) > 255 {
		return errors.New("role name cannot exceed 255 characters")
	}
	if len(r.Description) > 1000 {
		return errors.New("description cannot exceed 1000 characters")
	}

	// Permission validation
	if r.Permissions == nil {
		return errors.New("permissions cannot be nil")
	}

	// Validate each permission
	validPerms := map[string]bool{
		// User management
		"users.view":   true,
		"users.create": true,
		"users.edit":   true,
		"users.delete": true,

		// Role management
		"roles.view":   true,
		"roles.create": true,
		"roles.edit":   true,
		"roles.delete": true,

		// Config management
		"configs.view":   true,
		"configs.create": true,
		"configs.edit":   true,
		"configs.delete": true,

		// Job management
		"jobs.view":   true,
		"jobs.create": true,
		"jobs.edit":   true,
		"jobs.delete": true,
		"jobs.run":    true,

		// Audit logs
		"audit.view":   true,
		"audit.export": true,

		// System settings
		"system.settings": true,
		"system.backup":   true,
		"system.restore":  true,
	}

	for _, perm := range r.Permissions {
		if !validPerms[perm] {
			return errors.New("invalid permission: " + perm)
		}
	}

	return nil
}

// BeforeSave is a GORM hook that runs before saving the role
func (r *Role) BeforeSave(tx *gorm.DB) error {
	return r.Validate()
}

// BeforeDelete is a GORM hook that runs before deleting the role
func (r *Role) BeforeDelete(tx *gorm.DB) error {
	if r.IsSystemRole() {
		return errors.New("cannot delete system role")
	}

	// Create audit log for deletion
	return tx.Create(&AuditLog{
		Action:     "delete_role",
		EntityType: "role",
		EntityID:   r.ID,
		Details: map[string]interface{}{
			"role_name":   r.Name,
			"permissions": r.Permissions,
		},
		Timestamp: time.Now(),
	}).Error
}

// AfterCreate is a GORM hook that runs after creating the role
func (r *Role) AfterCreate(tx *gorm.DB) error {
	// Create audit log for creation
	return tx.Create(&AuditLog{
		Action:     "create_role",
		EntityType: "role",
		EntityID:   r.ID,
		Details: map[string]interface{}{
			"role_name":   r.Name,
			"permissions": r.Permissions,
		},
		Timestamp: time.Now(),
	}).Error
}

// AfterUpdate is a GORM hook that runs after updating the role
func (r *Role) AfterUpdate(tx *gorm.DB) error {
	// Create audit log for update
	return tx.Create(&AuditLog{
		Action:     "update_role",
		EntityType: "role",
		EntityID:   r.ID,
		Details: map[string]interface{}{
			"role_name":   r.Name,
			"permissions": r.Permissions,
		},
		Timestamp: time.Now(),
	}).Error
}

// Permissions is a custom type for storing permissions as a JSON array
type Permissions []string

// Scan implements the sql.Scanner interface
func (p *Permissions) Scan(value interface{}) error {
	if value == nil {
		*p = make(Permissions, 0)
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return errors.New("failed to scan Permissions: invalid type")
	}

	return json.Unmarshal([]byte(str), p)
}

// Value implements the driver.Valuer interface
func (p Permissions) Value() (driver.Value, error) {
	if p == nil {
		p = make(Permissions, 0)
	}
	bytes, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// HasPermission checks if the role has a specific permission
func (r *Role) HasPermission(permission string) bool {
	for _, p := range r.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// AddPermission adds a permission to the role if it doesn't already exist
func (r *Role) AddPermission(permission string) {
	if !r.HasPermission(permission) {
		r.Permissions = append(r.Permissions, permission)
	}
}

// RemovePermission removes a permission from the role
func (r *Role) RemovePermission(permission string) {
	if r.Permissions == nil {
		return
	}
	for i, p := range r.Permissions {
		if p == permission {
			r.Permissions = append(r.Permissions[:i], r.Permissions[i+1:]...)
			return
		}
	}
}

// SetPermissions sets the role's permissions
func (r *Role) SetPermissions(permissions []string) {
	r.Permissions = permissions
}

// GetPermissions returns the role's permissions
func (r *Role) GetPermissions() []string {
	if r.Permissions == nil {
		return make([]string, 0)
	}
	return r.Permissions
}

// IsSystemRole checks if this is a system-defined role that shouldn't be modified
func (r *Role) IsSystemRole() bool {
	systemRoles := []string{"admin", "system", "superuser"}
	for _, role := range systemRoles {
		if strings.ToLower(r.Name) == role {
			return true
		}
	}
	return false
}

// AuditLog creates an audit log entry for role changes
func (r *Role) AuditLog(tx *gorm.DB, action string, userID uint) error {
	return tx.Create(&AuditLog{
		Action:     action,
		EntityType: "role",
		EntityID:   r.ID,
		UserID:     userID,
		Details:    map[string]interface{}{"name": r.Name, "permissions": r.Permissions},
	}).Error
}
