package auth

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// PasswordPolicy defines the requirements for password strength and management
type PasswordPolicy struct {
	MinLength        int           // Minimum password length
	RequireUppercase bool          // Require at least one uppercase letter
	RequireLowercase bool          // Require at least one lowercase letter
	RequireNumbers   bool          // Require at least one number
	RequireSpecial   bool          // Require at least one special character
	ExpirationDays   int           // Number of days until password expires (0 = never)
	HistoryCount     int           // Number of previous passwords to remember (0 = disabled)
	DisallowCommon   bool          // Disallow common passwords
	MaxLoginAttempts int           // Maximum failed login attempts before lockout
	LockoutDuration  time.Duration // Duration of account lockout after max failed attempts
}

// PasswordHistory represents a historical password entry
type PasswordHistory struct {
	ID           uint   `gorm:"primarykey"`
	UserID       uint   `gorm:"not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}

// DefaultPasswordPolicy returns the default password policy
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSpecial:   true,
		ExpirationDays:   90,
		HistoryCount:     5,
		DisallowCommon:   true,
		MaxLoginAttempts: 5,
		LockoutDuration:  15 * time.Minute,
	}
}

// ValidatePassword checks if a password meets the policy requirements
func ValidatePassword(password string, policy PasswordPolicy) error {
	// Check minimum length
	if len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters long", policy.MinLength)
	}

	// Check for uppercase letters
	if policy.RequireUppercase {
		match, _ := regexp.MatchString("[A-Z]", password)
		if !match {
			return errors.New("password must contain at least one uppercase letter")
		}
	}

	// Check for lowercase letters
	if policy.RequireLowercase {
		match, _ := regexp.MatchString("[a-z]", password)
		if !match {
			return errors.New("password must contain at least one lowercase letter")
		}
	}

	// Check for numbers
	if policy.RequireNumbers {
		match, _ := regexp.MatchString("[0-9]", password)
		if !match {
			return errors.New("password must contain at least one number")
		}
	}

	// Check for special characters
	if policy.RequireSpecial {
		match, _ := regexp.MatchString("[^a-zA-Z0-9]", password)
		if !match {
			return errors.New("password must contain at least one special character")
		}
	}

	// Check for common passwords
	if policy.DisallowCommon && isCommonPassword(password) {
		return errors.New("password is too common or easily guessable")
	}

	return nil
}

// CheckPasswordHistory verifies the password against the user's password history
func CheckPasswordHistory(userID uint, newPassword string, hashedPassword string, db *gorm.DB, policy PasswordPolicy) error {
	if policy.HistoryCount <= 0 {
		return nil
	}

	var passwordHistories []PasswordHistory
	if err := db.Where("user_id = ?", userID).Order("created_at desc").Limit(policy.HistoryCount).Find(&passwordHistories).Error; err != nil {
		return err
	}

	// Check current password
	if ComparePasswords(hashedPassword, newPassword) == nil {
		return errors.New("new password cannot be the same as your current password")
	}

	// Check password history
	for _, history := range passwordHistories {
		if ComparePasswords(history.PasswordHash, newPassword) == nil {
			return fmt.Errorf("password was used in the last %d passwords", policy.HistoryCount)
		}
	}

	return nil
}

// IsPasswordExpired checks if the user's password has expired
func IsPasswordExpired(lastPasswordChange time.Time, policy PasswordPolicy) bool {
	if policy.ExpirationDays <= 0 {
		return false
	}

	expirationTime := lastPasswordChange.Add(time.Duration(policy.ExpirationDays) * 24 * time.Hour)
	return time.Now().After(expirationTime)
}

// UpdatePasswordHistory adds the new password to the user's password history
func UpdatePasswordHistory(userID uint, hashedPassword string, db *gorm.DB, policy PasswordPolicy) error {
	if policy.HistoryCount <= 0 {
		return nil
	}

	// Add new password to history
	passwordHistory := PasswordHistory{
		UserID:       userID,
		PasswordHash: hashedPassword,
	}

	if err := db.Create(&passwordHistory).Error; err != nil {
		return err
	}

	// Trim history if needed
	var count int64
	db.Model(&PasswordHistory{}).Where("user_id = ?", userID).Count(&count)

	if count > int64(policy.HistoryCount) {
		var oldestHistories []PasswordHistory
		if err := db.Where("user_id = ?", userID).Order("created_at asc").Limit(int(count) - policy.HistoryCount).Find(&oldestHistories).Error; err != nil {
			return err
		}

		for _, history := range oldestHistories {
			if err := db.Delete(&history).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// ComparePasswords compares a hashed password with a plain text password
func ComparePasswords(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

// isCommonPassword checks if a password is in the list of common passwords
func isCommonPassword(password string) bool {
	commonPasswords := []string{
		"password", "123456", "12345678", "qwerty", "abc123", "monkey",
		"1234567", "letmein", "trustno1", "dragon", "baseball", "111111",
		"iloveyou", "master", "sunshine", "ashley", "bailey", "passw0rd",
		"shadow", "123123", "654321", "superman", "qazwsx", "michael",
		"football", "welcome", "jesus", "ninja", "mustang", "password1",
		"admin", "admin123", "root", "toor", "qwerty123", "123qwe",
	}

	lowercasePassword := strings.ToLower(password)
	for _, common := range commonPasswords {
		if lowercasePassword == common {
			return true
		}
	}

	return false
}
