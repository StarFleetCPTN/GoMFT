package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
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
		userID, ok := claims["user_id"].(float64)
		if !ok || userID <= 0 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

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

		// Load user's roles and permissions
		var user db.User
		if h.DB != nil && userID > 0 {
			if err := h.DB.Preload("Roles").First(&user, uint(userID)).Error; err != nil {
				log.Printf("Error loading user roles: %v", err)
				// Continue without roles if there's an error
			} else {
				c.Set("user", &user)
			}
		}

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

// PermissionMiddleware creates a middleware that checks for specific permissions
func (h *Handlers) PermissionMiddleware(requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
			c.Abort()
			return
		}

		// Type assert to *db.User
		u, ok := user.(*db.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type in context"})
			c.Abort()
			return
		}

		// Check if user is admin (admins have all permissions)
		isAdmin, exists := c.Get("isAdmin")
		if exists && isAdmin.(bool) {
			c.Next()
			return
		}

		// Check each required permission
		for _, permission := range requiredPermissions {
			if !u.HasPermission(permission) {
				// If it's an API request, return JSON
				if strings.HasPrefix(c.Request.URL.Path, "/api/") {
					c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
				} else {
					// For web requests, redirect to dashboard with error message
					c.Redirect(http.StatusFound, "/dashboard?error=insufficient_permissions")
				}
				c.Abort()
				return
			}
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

		// Safely extract user ID
		userIDFloat, ok := claims["user_id"].(float64)
		if !ok || userIDFloat <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
			c.Abort()
			return
		}

		userID := uint(userIDFloat)
		c.Set("userID", userID)

		// Extract other claims
		if email, ok := claims["email"].(string); ok {
			c.Set("email", email)
		}
		if username, ok := claims["username"].(string); ok {
			c.Set("username", username)
		}
		if isAdmin, ok := claims["is_admin"].(bool); ok {
			c.Set("isAdmin", isAdmin)
		}

		// Load user's roles and permissions for API requests
		if h.DB != nil {
			var user db.User
			if err := h.DB.Preload("Roles").First(&user, userID).Error; err != nil {
				log.Printf("Error loading user roles: %v", err)
				// Continue without roles
			} else {
				c.Set("user", &user)
			}
		}

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

// GetAuthProviders returns the list of enabled authentication providers for login
func (h *Handlers) GetAuthProviders(c *gin.Context) {
	providers, err := h.DB.GetEnabledAuthProviders(c.Request.Context())
	if err != nil {
		log.Printf("Error fetching auth providers: %v", err)
		c.String(http.StatusInternalServerError, "")
		return
	}

	components.AuthProviderButtons(providers).Render(c.Request.Context(), c.Writer)
}

// HandleAuthProviderInit initiates authentication with the selected provider
func (h *Handlers) HandleAuthProviderInit(c *gin.Context) {
	// Get provider ID
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		h.HandleBadRequest(c, "Invalid Provider ID", "The provider ID is not valid")
		return
	}

	// Get the auth provider
	provider, err := h.DB.GetAuthProviderByID(c.Request.Context(), uint(providerID))
	if err != nil || !provider.Enabled {
		h.HandleBadRequest(c, "Provider Not Available", "The authentication provider is not available")
		return
	}

	// Generate state parameter for CSRF protection
	state, err := generateResetToken(32)
	if err != nil {
		h.HandleServerError(c, err)
		return
	}

	// Store state in session/cookie for validation on callback
	c.SetCookie("auth_state", state, 3600, "/", "", false, true)
	c.SetCookie("auth_provider_id", fmt.Sprintf("%d", providerID), 3600, "/", "", false, true)

	// Get base URL for redirect URI
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		// Try to detect the base URL from the request
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}

	// Default redirect URL if not specified in provider
	redirectURI := provider.RedirectURL
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("%s/auth/callback", baseURL)
	}

	// Handle different provider types
	switch provider.Type {
	case db.ProviderTypeOIDC, db.ProviderTypeOAuth2:
		// Build the authorization URL for OIDC/OAuth2
		scopes := "openid profile email"
		if provider.Scopes != "" {
			scopes = provider.Scopes
		}

		// Get config for OIDC
		configData, _ := provider.GetConfig()
		var authEndpoint string

		if provider.Type == db.ProviderTypeOIDC && configData["discovery_url"] != "" {
			// Fetch from discovery endpoint
			discoveryURL, _ := configData["discovery_url"].(string)
			discoveryData, err := h.fetchOIDCDiscovery(discoveryURL)
			if err != nil {
				h.HandleServerError(c, err)
				return
			}
			authEndpoint = discoveryData["authorization_endpoint"].(string)
		} else {
			// Use provider URL as base
			authEndpoint = fmt.Sprintf("%s/oauth2/authorize", provider.ProviderURL)
		}

		// Build the auth URL
		authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&state=%s",
			authEndpoint,
			url.QueryEscape(provider.ClientID),
			url.QueryEscape(redirectURI),
			url.QueryEscape(scopes),
			url.QueryEscape(state))

		// Redirect user to the authorization endpoint
		c.Redirect(http.StatusFound, authURL)
		return

	case db.ProviderTypeAuthentik:
		// Build the authorization URL for Authentik
		configData, _ := provider.GetConfig()
		tenant := "default"
		if tenantID, ok := configData["tenant_id"].(string); ok && tenantID != "" {
			tenant = tenantID
		}

		// Construct Authentik authorization URL
		scopes := "openid profile email"
		if provider.Scopes != "" {
			scopes = provider.Scopes
		}

		authURL := fmt.Sprintf("%s/application/o/authorize/?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&state=%s&tenant=%s",
			strings.TrimSuffix(provider.ProviderURL, "/"),
			url.QueryEscape(provider.ClientID),
			url.QueryEscape(redirectURI),
			url.QueryEscape(scopes),
			url.QueryEscape(state),
			url.QueryEscape(tenant))

		// Redirect user to the Authentik authorization endpoint
		c.Redirect(http.StatusFound, authURL)
		return

	case db.ProviderTypeSAML:
		// Note: SAML flows work differently than OAuth2/OIDC
		// Here you would typically generate a SAML request and redirect the user
		// This is just a placeholder - actual SAML implementation would need a SAML library
		h.HandleBadRequest(c, "SAML Not Implemented", "SAML authentication is not yet implemented")
		return

	default:
		h.HandleBadRequest(c, "Unsupported Provider", "The authentication provider type is not supported")
		return
	}
}

// Helper function to fetch OIDC discovery document
func (h *Handlers) fetchOIDCDiscovery(discoveryURL string) (map[string]interface{}, error) {
	resp, err := http.Get(discoveryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch discovery document, status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// HandleAuthProviderCallback handles the callback from external authentication providers
func (h *Handlers) HandleAuthProviderCallback(c *gin.Context) {
	// Get state and code from query params
	state := c.Query("state")
	code := c.Query("code")

	if state == "" || code == "" {
		h.HandleBadRequest(c, "Invalid Authentication Response", "Missing required parameters from authentication provider")
		return
	}

	// Verify state to prevent CSRF
	storedState, err := c.Cookie("auth_state")
	if err != nil || state != storedState {
		h.HandleBadRequest(c, "Invalid Authentication State", "The authentication process was corrupted or expired")
		return
	}

	// Get provider ID from cookie
	providerIDStr, err := c.Cookie("auth_provider_id")
	if err != nil {
		h.HandleBadRequest(c, "Authentication Error", "Unable to determine authentication provider")
		return
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		h.HandleBadRequest(c, "Invalid Provider ID", "The provider ID is not valid")
		return
	}

	// Get the auth provider
	provider, err := h.DB.GetAuthProviderByID(c.Request.Context(), uint(providerID))
	if err != nil || !provider.Enabled {
		h.HandleBadRequest(c, "Provider Not Available", "The authentication provider is not available")
		return
	}

	// Get base URL for redirect URI
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		// Try to detect the base URL from the request
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}

	// Use provider's redirect URL or default
	redirectURI := provider.RedirectURL
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("%s/auth/callback", baseURL)
	}

	// Exchange code for tokens
	var userInfo map[string]interface{}
	var externalID string
	var email string
	var username string
	var displayName string

	switch provider.Type {
	case db.ProviderTypeOIDC, db.ProviderTypeOAuth2, db.ProviderTypeAuthentik:
		// Get token endpoint
		var tokenEndpoint string
		if provider.Type == db.ProviderTypeOIDC {
			configData, _ := provider.GetConfig()
			if discoveryURL, ok := configData["discovery_url"].(string); ok && discoveryURL != "" {
				discoveryData, err := h.fetchOIDCDiscovery(discoveryURL)
				if err != nil {
					h.HandleServerError(c, err)
					return
				}
				tokenEndpoint = discoveryData["token_endpoint"].(string)
			} else {
				tokenEndpoint = fmt.Sprintf("%s/oauth2/token", provider.ProviderURL)
			}
		} else if provider.Type == db.ProviderTypeAuthentik {
			tokenEndpoint = fmt.Sprintf("%s/application/o/token/", strings.TrimSuffix(provider.ProviderURL, "/"))
		} else {
			tokenEndpoint = fmt.Sprintf("%s/oauth/token", provider.ProviderURL)
		}

		// Exchange code for token
		data := url.Values{}
		data.Set("grant_type", "authorization_code")
		data.Set("code", code)
		data.Set("redirect_uri", redirectURI)
		data.Set("client_id", provider.ClientID)
		data.Set("client_secret", provider.ClientSecret)

		resp, err := http.PostForm(tokenEndpoint, data)
		if err != nil {
			h.HandleServerError(c, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Error exchanging code for token: %s", string(body))
			h.HandleBadRequest(c, "Authentication Failed", "Failed to authenticate with the provider")
			return
		}

		var tokenResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
			h.HandleServerError(c, err)
			return
		}

		// Get access token
		accessToken, ok := tokenResponse["access_token"].(string)
		if !ok {
			h.HandleBadRequest(c, "Authentication Failed", "Invalid token response from provider")
			return
		}

		// Get user info
		userInfo, err = h.fetchUserInfo(provider, accessToken)
		if err != nil {
			h.HandleServerError(c, err)
			return
		}

		// Extract fields based on attribute mapping
		var attributeMapping map[string]string
		if provider.AttributeMapping != "" {
			if err := json.Unmarshal([]byte(provider.AttributeMapping), &attributeMapping); err != nil {
				log.Printf("Error parsing attribute mapping: %v", err)
				// Use defaults if mapping fails
				attributeMapping = map[string]string{
					"username": "preferred_username",
					"email":    "email",
					"name":     "name",
				}
			}
		} else {
			// Default mapping
			attributeMapping = map[string]string{
				"username": "preferred_username",
				"email":    "email",
				"name":     "name",
			}
		}

		// Extract user info using the attribute mapping
		if subValue, ok := userInfo["sub"].(string); ok {
			externalID = subValue
		} else {
			// Generate fallback ID if 'sub' is not available
			externalID = fmt.Sprintf("%s_%d", provider.Type, time.Now().Unix())
		}

		// Extract email - this is critical for user matching
		if emailAttr, ok := attributeMapping["email"]; ok && emailAttr != "" {
			if emailValue, ok := userInfo[emailAttr].(string); ok {
				email = emailValue
			}
		}
		if email == "" && userInfo["email"] != nil {
			email = userInfo["email"].(string)
		}

		// Extract username
		if usernameAttr, ok := attributeMapping["username"]; ok && usernameAttr != "" {
			if usernameValue, ok := userInfo[usernameAttr].(string); ok {
				username = usernameValue
			}
		}
		if username == "" && userInfo["preferred_username"] != nil {
			username = userInfo["preferred_username"].(string)
		}

		// Extract display name
		if nameAttr, ok := attributeMapping["name"]; ok && nameAttr != "" {
			if nameValue, ok := userInfo[nameAttr].(string); ok {
				displayName = nameValue
			}
		}
		if displayName == "" && userInfo["name"] != nil {
			displayName = userInfo["name"].(string)
		}

	default:
		h.HandleBadRequest(c, "Unsupported Provider", "The authentication provider type is not supported")
		return
	}

	// Require email for user identification
	if email == "" {
		h.HandleBadRequest(c, "Authentication Failed", "Unable to retrieve email address from the provider")
		return
	}

	// Look up existing user by email
	existingUser, err := h.DB.GetUserByEmail(email)
	if err != nil {
		// If user doesn't exist, check if auto-provisioning is allowed
		// For now, we'll require existing users
		h.HandleBadRequest(c, "Authentication Failed", "No account exists with this email address")
		return
	}

	// Check for existing identity
	identity, err := h.DB.GetExternalUserIdentity(c.Request.Context(), provider.ID, externalID)
	if err != nil || identity == nil {
		// If identity doesn't exist, create it
		identity = &db.ExternalUserIdentity{
			UserID:       existingUser.ID,
			ProviderID:   provider.ID,
			ProviderType: provider.Type,
			ExternalID:   externalID,
			Email:        email,
			Username:     username,
			DisplayName:  displayName,
			LastLogin:    sql.NullTime{Time: time.Now(), Valid: true},
		}

		// Store provider data
		if err := identity.SetProviderData(userInfo); err != nil {
			log.Printf("Error serializing provider data: %v", err)
		}

		// Extract and store groups if available
		var attributeMapping map[string]string
		if provider.AttributeMapping != "" {
			if err := json.Unmarshal([]byte(provider.AttributeMapping), &attributeMapping); err == nil {
				if groupsAttr, ok := attributeMapping["groups"]; ok && groupsAttr != "" {
					if groupsValue, ok := userInfo[groupsAttr]; ok {
						// Handle different group formats (array or comma-separated string)
						var groups []string
						switch v := groupsValue.(type) {
						case []interface{}:
							for _, g := range v {
								if gs, ok := g.(string); ok {
									groups = append(groups, gs)
								}
							}
						case string:
							groups = strings.Split(v, ",")
						}

						if len(groups) > 0 {
							if err := identity.SetGroups(groups); err != nil {
								log.Printf("Error serializing groups: %v", err)
							}
						}
					}
				}
			} else {
				log.Printf("Error parsing attribute mapping: %v", err)
			}
		}

		// Save the identity
		if err := h.DB.CreateExternalUserIdentity(c.Request.Context(), identity); err != nil {
			log.Printf("Error creating external identity: %v", err)
			h.HandleServerError(c, err)
			return
		}
	} else {
		// Update existing identity
		identity.LastLogin = sql.NullTime{Time: time.Now(), Valid: true}
		identity.Email = email
		identity.Username = username
		identity.DisplayName = displayName

		// Update provider data
		if err := identity.SetProviderData(userInfo); err != nil {
			log.Printf("Error serializing provider data: %v", err)
		}

		// Save the updated identity
		if err := h.DB.UpdateExternalUserIdentity(c.Request.Context(), identity); err != nil {
			log.Printf("Error updating external identity: %v", err)
		}
	}

	// Update provider usage stats
	provider.SuccessfulLogins++
	provider.LastUsed = sql.NullTime{Time: time.Now(), Valid: true}
	if err := h.DB.UpdateAuthProvider(c.Request.Context(), provider); err != nil {
		log.Printf("Error updating provider stats: %v", err)
	}

	// Create session for the user
	isAdmin := false
	if existingUser.IsAdmin != nil {
		isAdmin = *existingUser.IsAdmin
	}
	token, err := h.GenerateJWT(existingUser.ID, existingUser.Email, isAdmin)
	if err != nil {
		h.HandleServerError(c, err)
		return
	}

	// Clear auth cookies
	c.SetCookie("auth_state", "", -1, "/", "", false, true)
	c.SetCookie("auth_provider_id", "", -1, "/", "", false, true)

	// Set session cookie
	c.SetCookie("jwt_token", token, 86400, "/", "", false, true)

	// Redirect to dashboard
	c.Redirect(http.StatusFound, "/dashboard")
}

// Helper function to fetch user info with access token
func (h *Handlers) fetchUserInfo(provider *db.AuthProvider, accessToken string) (map[string]interface{}, error) {
	var userInfoEndpoint string

	// Determine user info endpoint based on provider type
	switch provider.Type {
	case db.ProviderTypeOIDC:
		// For OIDC, check if we have discovery URL
		configData, _ := provider.GetConfig()
		if discoveryURL, ok := configData["discovery_url"].(string); ok && discoveryURL != "" {
			discoveryData, err := h.fetchOIDCDiscovery(discoveryURL)
			if err != nil {
				return nil, err
			}
			userInfoEndpoint = discoveryData["userinfo_endpoint"].(string)
		} else {
			userInfoEndpoint = fmt.Sprintf("%s/oauth2/userinfo", provider.ProviderURL)
		}

	case db.ProviderTypeAuthentik:
		userInfoEndpoint = fmt.Sprintf("%s/application/o/userinfo/", strings.TrimSuffix(provider.ProviderURL, "/"))

	case db.ProviderTypeOAuth2:
		userInfoEndpoint = fmt.Sprintf("%s/oauth/userinfo", provider.ProviderURL)

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}

	// Make request to user info endpoint
	req, err := http.NewRequest("GET", userInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add authorization header
	req.Header.Add("Authorization", "Bearer "+accessToken)

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: %s", string(body))
	}

	// Parse response
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return userInfo, nil
}
