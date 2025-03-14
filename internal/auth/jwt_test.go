package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAndValidateToken(t *testing.T) {
	// Setup test data
	userID := uint(1)
	email := "test@example.com"
	secret := "test-jwt-secret"
	expirationTime := 1 * time.Hour

	// Generate a token
	token, err := GenerateToken(userID, email, secret, expirationTime)
	assert.NoError(t, err, "Should not return an error when generating a token")
	assert.NotEmpty(t, token, "Token should not be empty")

	// Validate the token
	claims, err := ValidateToken(token, secret)
	assert.NoError(t, err, "Should not return an error when validating a valid token")
	assert.NotNil(t, claims, "Claims should not be nil")
	assert.Equal(t, userID, claims.UserID, "UserID should match")
	assert.Equal(t, email, claims.Email, "Email should match")
}

func TestInvalidToken(t *testing.T) {
	// Setup
	invalidToken := "invalid.token.string"
	secret := "test-jwt-secret"

	// Validate the invalid token
	claims, err := ValidateToken(invalidToken, secret)
	assert.Error(t, err, "Should return an error when validating an invalid token")
	assert.Nil(t, claims, "Claims should be nil for an invalid token")
}

func TestExpiredToken(t *testing.T) {
	// Setup test data
	userID := uint(1)
	email := "test@example.com"
	secret := "test-jwt-secret"
	expirationTime := -1 * time.Hour // Negative duration to create an expired token

	// Generate an expired token
	token, err := GenerateToken(userID, email, secret, expirationTime)
	assert.NoError(t, err, "Should not return an error when generating a token")

	// Validate the expired token
	claims, err := ValidateToken(token, secret)
	assert.Error(t, err, "Should return an error when validating an expired token")
	assert.Nil(t, claims, "Claims should be nil for an expired token")
}

func TestInvalidSecret(t *testing.T) {
	// Setup test data
	userID := uint(1)
	email := "test@example.com"
	secret := "original-secret"
	wrongSecret := "wrong-secret"
	expirationTime := 1 * time.Hour

	// Generate a token with the original secret
	token, err := GenerateToken(userID, email, secret, expirationTime)
	assert.NoError(t, err, "Should not return an error when generating a token")

	// Validate the token with the wrong secret
	claims, err := ValidateToken(token, wrongSecret)
	assert.Error(t, err, "Should return an error when validating with the wrong secret")
	assert.Nil(t, claims, "Claims should be nil when validating with the wrong secret")
}
