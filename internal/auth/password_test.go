package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// MockDB is a mock implementation of *gorm.DB for testing
type MockDB struct {
	mock.Mock
}

func (m *MockDB) Where(query interface{}, args ...interface{}) *gorm.DB {
	m.Called(query, args)
	return &gorm.DB{}
}

func (m *MockDB) Order(value interface{}) *gorm.DB {
	m.Called(value)
	return &gorm.DB{}
}

func (m *MockDB) Limit(limit int) *gorm.DB {
	m.Called(limit)
	return &gorm.DB{}
}

func (m *MockDB) Find(dest interface{}, conds ...interface{}) *gorm.DB {
	m.Called(dest, conds)
	return &gorm.DB{}
}

func (m *MockDB) Create(value interface{}) *gorm.DB {
	m.Called(value)
	return &gorm.DB{}
}

func (m *MockDB) Delete(value interface{}, conds ...interface{}) *gorm.DB {
	m.Called(value, conds)
	return &gorm.DB{}
}

func (m *MockDB) Model(value interface{}) *gorm.DB {
	m.Called(value)
	return &gorm.DB{}
}

func (m *MockDB) Count(count *int64) *gorm.DB {
	m.Called(count)
	*count = 10 // Mock count for testing
	return &gorm.DB{}
}

func TestDefaultPasswordPolicy(t *testing.T) {
	policy := DefaultPasswordPolicy()

	assert.Equal(t, 8, policy.MinLength, "Default min length should be 8")
	assert.True(t, policy.RequireUppercase, "Should require uppercase by default")
	assert.True(t, policy.RequireLowercase, "Should require lowercase by default")
	assert.True(t, policy.RequireNumbers, "Should require numbers by default")
	assert.True(t, policy.RequireSpecial, "Should require special chars by default")
	assert.Equal(t, 90, policy.ExpirationDays, "Default expiration should be 90 days")
	assert.Equal(t, 5, policy.HistoryCount, "Default history count should be 5")
	assert.True(t, policy.DisallowCommon, "Should disallow common passwords by default")
	assert.Equal(t, 5, policy.MaxLoginAttempts, "Default max login attempts should be 5")
	assert.Equal(t, 15*time.Minute, policy.LockoutDuration, "Default lockout duration should be 15 minutes")
}

func TestValidatePassword(t *testing.T) {
	policy := DefaultPasswordPolicy()

	// Test valid password
	err := ValidatePassword("Test1234!", policy)
	assert.NoError(t, err, "Valid password should pass validation")

	// Test password too short
	err = ValidatePassword("Test1!", policy)
	assert.Error(t, err, "Password shorter than minimum length should fail")
	assert.Contains(t, err.Error(), "at least 8 characters")

	// Test password without uppercase
	err = ValidatePassword("test1234!", policy)
	assert.Error(t, err, "Password without uppercase should fail")
	assert.Contains(t, err.Error(), "uppercase letter")

	// Test password without lowercase
	err = ValidatePassword("TEST1234!", policy)
	assert.Error(t, err, "Password without lowercase should fail")
	assert.Contains(t, err.Error(), "lowercase letter")

	// Test password without numbers
	err = ValidatePassword("TestTest!", policy)
	assert.Error(t, err, "Password without numbers should fail")
	assert.Contains(t, err.Error(), "number")

	// Test password without special characters
	err = ValidatePassword("Test1234", policy)
	assert.Error(t, err, "Password without special characters should fail")
	assert.Contains(t, err.Error(), "special character")

	// Test common password - we need to disable other validations to test just the common password check
	customPolicy := DefaultPasswordPolicy()
	customPolicy.RequireUppercase = false
	customPolicy.RequireLowercase = false
	customPolicy.RequireNumbers = false
	customPolicy.RequireSpecial = false

	err = ValidatePassword("password", customPolicy)
	assert.Error(t, err, "Common password should fail even with relaxed requirements")
	assert.Contains(t, err.Error(), "common or easily guessable")

	// Test with custom policy (all validations disabled)
	verySimplePolicy := PasswordPolicy{
		MinLength:        6,
		RequireUppercase: false,
		RequireLowercase: false,
		RequireNumbers:   false,
		RequireSpecial:   false,
		DisallowCommon:   false,
	}

	err = ValidatePassword("simple", verySimplePolicy)
	assert.NoError(t, err, "Simple password should pass with all validations disabled")
}

func TestComparePasswords(t *testing.T) {
	// Generate a hashed password
	plainPassword := "TestPassword123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	assert.NoError(t, err, "Password hashing should not error")

	// Test valid password comparison
	err = ComparePasswords(string(hashedPassword), plainPassword)
	assert.NoError(t, err, "Correct password should match hash")

	// Test invalid password comparison
	err = ComparePasswords(string(hashedPassword), "WrongPassword123!")
	assert.Error(t, err, "Incorrect password should not match hash")
}

func TestIsPasswordExpired(t *testing.T) {
	policy := DefaultPasswordPolicy()

	// Test password within expiration period
	lastChange := time.Now().Add(-80 * 24 * time.Hour) // 80 days ago
	assert.False(t, IsPasswordExpired(lastChange, policy), "Password changed 80 days ago should not be expired")

	// Test expired password
	lastChange = time.Now().Add(-100 * 24 * time.Hour) // 100 days ago
	assert.True(t, IsPasswordExpired(lastChange, policy), "Password changed 100 days ago should be expired")

	// Test with expiration disabled
	customPolicy := PasswordPolicy{
		ExpirationDays: 0, // Disabled
	}
	lastChange = time.Now().Add(-1000 * 24 * time.Hour) // 1000 days ago
	assert.False(t, IsPasswordExpired(lastChange, customPolicy), "Password should not expire when expiration is disabled")
}

func TestIsCommonPassword(t *testing.T) {
	// Test with common passwords
	assert.True(t, isCommonPassword("password"), "Should detect 'password' as common")
	assert.True(t, isCommonPassword("admin123"), "Should detect 'admin123' as common")
	assert.True(t, isCommonPassword("QWERTY"), "Should detect 'QWERTY' as common (case insensitive)")

	// Test with uncommon passwords
	assert.False(t, isCommonPassword("G4x8qT2!pL9z"), "Should not detect complex password as common")
	assert.False(t, isCommonPassword("UniquePassword123!"), "Should not detect unique password as common")
}
