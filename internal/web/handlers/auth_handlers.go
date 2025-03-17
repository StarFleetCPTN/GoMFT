package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/auth"
	"github.com/starfleetcptn/gomft/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// Define a custom type for context keys to avoid string collisions
type contextKey string

// Context keys
const (
	themeKey contextKey = "theme"
	emailKey contextKey = "email"
)

// AuthMiddleware is a middleware function that checks if the user is authenticated
func (h *Handlers) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the JWT token from the cookie
		tokenString, err := c.Cookie("jwt_token")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Parse and validate the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(h.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Safely extract claims with type assertions and defaults
		userID, _ := claims["user_id"].(float64)
		email, _ := claims["email"].(string)
		username, _ := claims["username"].(string)
		isAdmin, _ := claims["is_admin"].(bool)

		// Set user information in the context
		c.Set("userID", uint(userID))
		if email != "" {
			c.Set("email", email)
		}
		if username != "" {
			c.Set("username", username)
		}
		c.Set("isAdmin", isAdmin)

		c.Next()
	}
}

// AdminMiddleware is a middleware function that checks if the user is an admin
func (h *Handlers) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("isAdmin")
		if !exists || !isAdmin.(bool) {
			c.Redirect(http.StatusFound, "/dashboard")
			c.Abort()
			return
		}
		c.Next()
	}
}

// APIAuthMiddleware is a middleware function that checks if the API request is authenticated
func (h *Handlers) APIAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Check if the header is in the correct format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		// Parse and validate the token
		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(h.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Set user information in the context
		c.Set("userID", uint(claims["user_id"].(float64)))
		c.Set("email", claims["email"].(string))
		c.Set("username", claims["username"].(string))
		c.Set("isAdmin", claims["is_admin"].(bool))

		c.Next()
	}
}

// APIAdminMiddleware is a middleware function that checks if the API request is from an admin
func (h *Handlers) APIAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("isAdmin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// GenerateJWT generates a JWT token for the given user
func (h *Handlers) GenerateJWT(userID uint, email string, isAdmin bool) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"email":    email,
		"username": strings.Split(email, "@")[0], // Use email prefix as username
		"is_admin": isAdmin,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(h.JWTSecret))
}

// HandleLoginPage handles the GET /login route
func (h *Handlers) HandleLoginPage(c *gin.Context) {
	// Check if user is already logged in
	if userID, exists := c.Get("userID"); exists && userID != nil {
		// User is logged in, redirect to dashboard
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// Create template context and set email if available
	ctx := components.CreateTemplateContext(c)
	if email, exists := c.Get("email"); exists {
		ctx = context.WithValue(ctx, emailKey, email)
	}

	// Check for message query param (used for password expired, etc.)
	message := c.Query("message")

	// User is not logged in, show login page
	if message != "" {
		components.Login(ctx, message).Render(c.Request.Context(), c.Writer)
	} else {
		components.Login(ctx, "").Render(c.Request.Context(), c.Writer)
	}
}

// HandleLogin handles the POST /login route
func (h *Handlers) HandleLogin(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	// Get user by email
	var user db.User
	if err := h.DB.Where("email = ?", email).First(&user).Error; err != nil {
		components.Login(components.CreateTemplateContext(c), "Invalid credentials").Render(c, c.Writer)
		return
	}

	// Check if account is locked
	if user.GetAccountLocked() {
		if user.LockoutUntil != nil && time.Now().After(*user.LockoutUntil) {
			// Lockout period has expired, reset the lockout
			user.SetAccountLocked(false)
			user.FailedLoginAttempts = 0
			user.LockoutUntil = nil
			h.DB.Save(&user)
		} else {
			// Account is still locked
			components.Login(components.CreateTemplateContext(c), "Account is locked due to too many failed login attempts. Please try again later.").Render(c, c.Writer)
			return
		}
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Increment failed login attempts
		user.FailedLoginAttempts++

		// Check if we need to lock the account
		policy := auth.DefaultPasswordPolicy()
		if user.FailedLoginAttempts >= policy.MaxLoginAttempts {
			user.SetAccountLocked(true)
			lockoutTime := time.Now().Add(policy.LockoutDuration)
			user.LockoutUntil = &lockoutTime
			h.DB.Save(&user)
			components.Login(components.CreateTemplateContext(c), "Account is locked due to too many failed login attempts. Please try again later.").Render(c, c.Writer)
			return
		}

		h.DB.Save(&user)
		components.Login(components.CreateTemplateContext(c), "Invalid credentials").Render(c, c.Writer)
		return
	}

	// Reset failed login attempts on successful login
	user.FailedLoginAttempts = 0
	user.SetAccountLocked(false)
	user.LockoutUntil = nil
	h.DB.Save(&user)

	// Check password expiration
	policy := auth.DefaultPasswordPolicy()
	if auth.IsPasswordExpired(user.LastPasswordChange, policy) {
		// Add flash message about password expiration
		// We're simplifying by just redirecting to login with a message
		c.SetCookie("jwt_token", "", -1, "/", "", false, true) // Logout the user
		c.Redirect(http.StatusFound, "/login?message=Your+password+has+expired.+Please+contact+an+administrator.")
		return
	}

	// Check if 2FA is enabled
	if user.TwoFactorEnabled {
		// Store user ID temporarily for 2FA verification
		c.SetCookie("temp_user_id", fmt.Sprintf("%d", user.ID), 300, "/", "", false, true) // 5 minutes expiry

		// Redirect to 2FA verification page
		c.Redirect(http.StatusFound, "/login/verify")
		return
	}

	// If 2FA is not enabled, proceed with normal login
	// Generate JWT token with all necessary user information
	isAdmin := false
	if user.IsAdmin != nil {
		isAdmin = *user.IsAdmin
	}
	token, err := h.GenerateJWT(user.ID, user.Email, isAdmin)
	if err != nil {
		components.Login(components.CreateTemplateContext(c), "Authentication error").Render(c, c.Writer)
		return
	}

	// Set token in cookie
	c.SetCookie("jwt_token", token, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, "/dashboard")
}

// HandleLogout handles the POST /logout route
func (h *Handlers) HandleLogout(c *gin.Context) {
	c.SetCookie("jwt_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

// HandleChangePassword handles the POST /change-password route
// This is now only for use from the profile page
func (h *Handlers) HandleChangePassword(c *gin.Context) {
	// Get user ID from token
	tokenCookie, err := c.Cookie("jwt_token")
	if err != nil || tokenCookie == "" {
		if c.GetHeader("HX-Request") == "true" {
			c.Data(http.StatusUnauthorized, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
				<span class="block sm:inline">Authentication required</span>
			</div>`))
			return
		}
		c.Redirect(http.StatusFound, "/login")
		return
	}

	claims, err := auth.ValidateToken(tokenCookie, h.JWTSecret)
	if err != nil {
		if c.GetHeader("HX-Request") == "true" {
			c.Data(http.StatusUnauthorized, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
				<span class="block sm:inline">Invalid authentication</span>
			</div>`))
			return
		}
		c.SetCookie("jwt_token", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
		return
	}
	userID := claims.UserID

	// Get form values
	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate new password matches confirmation
	if newPassword != confirmPassword {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">New password and confirmation do not match</span>
		</div>`))
		return
	}

	// Get user
	var user db.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">User not found</span>
		</div>`))
		return
	}

	// Verify current password
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Current password is incorrect</span>
		</div>`))
		return
	}

	// Validate password against policy
	policy := auth.DefaultPasswordPolicy()
	if err := auth.ValidatePassword(newPassword, policy); err != nil {
		errorMsg := `<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">` + err.Error() + `</span>
		</div>`
		c.Data(http.StatusOK, "text/html", []byte(errorMsg))
		return
	}

	// Check password history
	if err := auth.CheckPasswordHistory(user.ID, newPassword, user.PasswordHash, h.DB.DB, policy); err != nil {
		errorMsg := `<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">` + err.Error() + `</span>
		</div>`
		c.Data(http.StatusOK, "text/html", []byte(errorMsg))
		return
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Error processing password</span>
		</div>`))
		return
	}

	// Update password history
	if err := auth.UpdatePasswordHistory(user.ID, string(hashedPassword), h.DB.DB, policy); err != nil {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Error updating password history</span>
		</div>`))
		return
	}

	// Update user's password
	user.PasswordHash = string(hashedPassword)
	user.LastPasswordChange = time.Now()
	if err := h.DB.Save(&user).Error; err != nil {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Error updating password</span>
		</div>`))
		return
	}

	// Return success message
	c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4" role="alert">
		<span class="block sm:inline">Password updated successfully!</span>
	</div>`))
}

// HandleForgotPasswordPage displays the forgot password form
func (h *Handlers) HandleForgotPasswordPage(c *gin.Context) {
	ctx := context.WithValue(c.Request.Context(), themeKey, "light")
	components.ForgotPassword(ctx, "", "").Render(c.Request.Context(), c.Writer)
}

// HandleForgotPassword processes the forgot password form submission
func (h *Handlers) HandleForgotPassword(c *gin.Context) {
	email := c.PostForm("email")
	if email == "" {
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ForgotPassword(ctx, "Email is required", "").Render(c.Request.Context(), c.Writer)
		return
	}

	// Check if user exists
	user, err := h.DB.GetUserByEmail(email)
	if err != nil {
		// Don't reveal that the email doesn't exist for security reasons
		// But we'll log it for debugging
		log.Printf("Password reset requested for non-existent email: %s", email)
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ForgotPassword(ctx, "", "If your email is registered, you will receive a password reset link.").Render(c.Request.Context(), c.Writer)
		return
	}

	// Generate reset token
	token, err := generateResetToken(32)
	if err != nil {
		log.Printf("Error generating reset token: %v", err)
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ForgotPassword(ctx, "An error occurred. Please try again later.", "").Render(c.Request.Context(), c.Writer)
		return
	}

	// Save token in database with expiration time (15 minutes)
	expiration := time.Now().Add(15 * time.Minute)
	resetToken := &db.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: expiration,
	}

	if err := h.DB.CreatePasswordResetToken(resetToken); err != nil {
		log.Printf("Error saving reset token: %v", err)
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ForgotPassword(ctx, "An error occurred. Please try again later.", "").Render(c.Request.Context(), c.Writer)
		return
	}

	// Send password reset email
	err = h.Email.SendPasswordResetEmail(user.Email, user.Email, token)
	if err != nil {
		// If email sending fails, log the error but don't expose this to the user
		log.Printf("Error sending password reset email: %v", err)

		// If email is disabled, log the reset link
		if strings.Contains(err.Error(), "email service is disabled") {
			log.Printf("Email service is disabled, reset link: %v", err)
		}
	}

	// Show success message regardless of whether email was sent
	// This prevents user enumeration attacks
	ctx := context.WithValue(c.Request.Context(), themeKey, "light")
	components.ForgotPassword(ctx, "", "If your email is registered, you will receive a password reset link.").Render(c.Request.Context(), c.Writer)
}

// HandleResetPasswordPage displays the reset password form
func (h *Handlers) HandleResetPasswordPage(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.Redirect(http.StatusFound, "/forgot-password")
		return
	}

	// Validate token exists and hasn't expired
	_, err := h.DB.GetPasswordResetToken(token)
	if err != nil {
		log.Printf("Invalid reset token: %s, error: %v", token, err)
		c.Redirect(http.StatusFound, "/forgot-password")
		return
	}

	ctx := context.WithValue(c.Request.Context(), themeKey, "light")
	components.ResetPassword(ctx, token, "").Render(c.Request.Context(), c.Writer)
}

// HandleResetPassword processes the reset password form submission
func (h *Handlers) HandleResetPassword(c *gin.Context) {
	token := c.PostForm("token")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm-password")

	if token == "" {
		c.Redirect(http.StatusFound, "/forgot-password")
		return
	}

	if password == "" || confirmPassword == "" {
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ResetPassword(ctx, token, "Both password fields are required.").Render(c.Request.Context(), c.Writer)
		return
	}

	if password != confirmPassword {
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ResetPassword(ctx, token, "Passwords do not match.").Render(c.Request.Context(), c.Writer)
		return
	}

	if len(password) < 8 {
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ResetPassword(ctx, token, "Password must be at least 8 characters long.").Render(c.Request.Context(), c.Writer)
		return
	}

	// Validate token and get user
	resetToken, err := h.DB.GetPasswordResetToken(token)
	if err != nil {
		log.Printf("Invalid reset token: %s, error: %v", token, err)
		c.Redirect(http.StatusFound, "/forgot-password")
		return
	}

	// Get the user
	user, err := h.DB.GetUserByID(resetToken.UserID)
	if err != nil {
		log.Printf("User not found for token: %s, user ID: %d, error: %v", token, resetToken.UserID, err)
		c.Redirect(http.StatusFound, "/forgot-password")
		return
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ResetPassword(ctx, token, "An error occurred. Please try again later.").Render(c.Request.Context(), c.Writer)
		return
	}

	// Update user's password
	user.PasswordHash = string(hashedPassword)
	user.LastPasswordChange = time.Now()
	if err := h.DB.UpdateUser(user); err != nil {
		log.Printf("Error updating user password: %v", err)
		ctx := context.WithValue(c.Request.Context(), themeKey, "light")
		components.ResetPassword(ctx, token, "An error occurred. Please try again later.").Render(c.Request.Context(), c.Writer)
		return
	}

	// Record password history
	passwordHistory := &auth.PasswordHistory{
		UserID:       user.ID,
		PasswordHash: string(hashedPassword),
	}
	if err := h.DB.DB.Create(passwordHistory).Error; err != nil {
		log.Printf("Error recording password history: %v", err)
	}

	// Mark token as used
	if err := h.DB.MarkPasswordResetTokenAsUsed(resetToken.ID); err != nil {
		log.Printf("Error marking token as used: %v", err)
	}

	// Redirect to login with success message
	c.Redirect(http.StatusFound, "/login?message=Password+reset+successful.+Please+log+in+with+your+new+password.")
}

// Helper function to generate a random token
func generateResetToken(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
