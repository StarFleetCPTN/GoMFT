package db

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents a user account in the system
type User struct {
	ID                  uint   `gorm:"primarykey"`
	Email               string `gorm:"unique;not null"`
	PasswordHash        string `gorm:"not null"`
	IsAdmin             *bool  `gorm:"default:false"`
	LastPasswordChange  time.Time
	FailedLoginAttempts int   `gorm:"default:0"`
	AccountLocked       *bool `gorm:"default:false"`
	LockoutUntil        *time.Time
	Theme               string `gorm:"default:'light'"`
	TwoFactorSecret     string `gorm:"type:varchar(32)"`
	TwoFactorEnabled    bool   `gorm:"default:false"`
	BackupCodes         string `gorm:"type:text"` // Comma-separated backup codes
	Roles               []Role `gorm:"many2many:user_roles"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// PasswordHistory stores previous passwords for a user
type PasswordHistory struct {
	ID           uint   `gorm:"primarykey"`
	UserID       uint   `gorm:"not null"`
	User         User   `gorm:"foreignkey:UserID"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}

// PasswordResetToken stores tokens for password reset requests
type PasswordResetToken struct {
	ID        uint      `gorm:"primarykey"`
	UserID    uint      `gorm:"not null"`
	User      User      `gorm:"foreignkey:UserID"`
	Token     string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Used      *bool     `gorm:"default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// --- User Helper Methods ---

// GetIsAdmin returns the value of IsAdmin with a default if nil
func (u *User) GetIsAdmin() bool {
	if u.IsAdmin == nil {
		return false // Default to false if not set
	}
	return *u.IsAdmin
}

// SetIsAdmin sets the IsAdmin field
func (u *User) SetIsAdmin(value bool) {
	u.IsAdmin = &value
}

// GetAccountLocked returns the value of AccountLocked with a default if nil
func (u *User) GetAccountLocked() bool {
	if u.AccountLocked == nil {
		return false // Default to false if not set
	}
	return *u.AccountLocked
}

// SetAccountLocked sets the AccountLocked field
func (u *User) SetAccountLocked(value bool) {
	u.AccountLocked = &value
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(roleName string) bool {
	for _, role := range u.Roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

// HasPermission checks if the user has a specific permission through any of their roles
func (u *User) HasPermission(permission string) bool {
	for _, role := range u.Roles {
		if role.HasPermission(permission) {
			return true
		}
	}
	return false
}

// GetRoles returns all roles assigned to the user
// Note: This requires preloading Roles when fetching the user
func (u *User) GetRoles(tx *gorm.DB) ([]Role, error) {
	var roles []Role
	err := tx.Model(u).Association("Roles").Find(&roles)
	return roles, err
}

// AssignRole assigns a role to the user
func (u *User) AssignRole(tx *gorm.DB, roleID uint, assignedByID uint) error {
	var role Role
	if err := tx.First(&role, roleID).Error; err != nil {
		return err
	}
	// Assuming Role struct has AssignToUser method (from role.go)
	return role.AssignToUser(tx, u.ID, assignedByID)
}

// UnassignRole removes a role from the user
func (u *User) UnassignRole(tx *gorm.DB, roleID uint, unassignedByID uint) error {
	var role Role
	if err := tx.First(&role, roleID).Error; err != nil {
		return err
	}
	// Assuming Role struct has UnassignFromUser method (from role.go)
	return role.UnassignFromUser(tx, u.ID, unassignedByID)
}

// SetPassword sets the user's password with secure hashing
func (u *User) SetPassword(password string) error {
	// Validate password length
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	// Hash the password using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Store the hashed password
	u.PasswordHash = string(hashedPassword)
	u.LastPasswordChange = time.Now()

	return nil
}

// CheckPassword verifies if the provided password matches the stored hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// --- PasswordResetToken Helper Methods ---

// GetUsed returns the value of Used with a default if nil
func (t *PasswordResetToken) GetUsed() bool {
	if t.Used == nil {
		return false // Default to false if not set
	}
	return *t.Used
}

// SetUsed sets the Used field
func (t *PasswordResetToken) SetUsed(value bool) {
	t.Used = &value
}
