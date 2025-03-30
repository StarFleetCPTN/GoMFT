package db

import (
	"time"
)

// --- User Store Methods ---

// CreateUser creates a new user record
func (db *DB) CreateUser(user *User) error {
	return db.Create(user).Error
}

// GetUserByEmail retrieves a user by their email address
func (db *DB) GetUserByEmail(email string) (*User, error) {
	var user User
	// Preload Roles to ensure they are available for permission checks
	err := db.Preload("Roles").Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves a user by their ID
func (db *DB) GetUserByID(id uint) (*User, error) {
	var user User
	// Preload Roles
	err := db.Preload("Roles").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates an existing user record
func (db *DB) UpdateUser(user *User) error {
	// Use Omit to prevent accidentally changing Roles association directly
	// Role assignments should use AssignRole/UnassignRole methods
	return db.Omit("Roles").Save(user).Error
}

// --- PasswordResetToken Store Methods ---

// CreatePasswordResetToken creates a new password reset token record
func (db *DB) CreatePasswordResetToken(token *PasswordResetToken) error {
	return db.Create(token).Error
}

// GetPasswordResetToken retrieves a valid, unused password reset token
func (db *DB) GetPasswordResetToken(token string) (*PasswordResetToken, error) {
	var resetToken PasswordResetToken
	err := db.Where("token = ? AND used = ? AND expires_at > ?", token, false, time.Now()).First(&resetToken).Error
	if err != nil {
		return nil, err
	}
	return &resetToken, nil
}

// MarkPasswordResetTokenAsUsed marks a password reset token as used
func (db *DB) MarkPasswordResetTokenAsUsed(tokenID uint) error {
	return db.Model(&PasswordResetToken{}).Where("id = ?", tokenID).Update("used", true).Error
}
