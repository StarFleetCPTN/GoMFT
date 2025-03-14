package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/email"
	"github.com/starfleetcptn/gomft/internal/testutils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup
	handlers, router := setupTestHandlers(t)
	jwtSecret := "test-jwt-secret"
	handlers.JWTSecret = jwtSecret

	// Create test route with auth middleware
	router.GET("/protected", handlers.AuthMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "protected content")
	})

	// Test case 1: No JWT token
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to login page
	assert.Equal(t, http.StatusFound, resp.Code, "Should redirect to login page")
	assert.Equal(t, "/login", resp.Header().Get("Location"), "Should redirect to /login")

	// Test case 2: Invalid JWT token
	req, _ = http.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt_token",
		Value: "invalid-token",
	})
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to login page due to invalid token
	assert.Equal(t, http.StatusFound, resp.Code, "Should redirect to login page on invalid token")
	assert.Equal(t, "/login", resp.Header().Get("Location"), "Should redirect to /login on invalid token")

	// Test case 3: Valid JWT token
	// Generate a valid token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  1,
		"email":    "test@example.com",
		"username": "testuser",
		"is_admin": false,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(jwtSecret))

	req, _ = http.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt_token",
		Value: tokenString,
	})
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should allow access to protected content
	assert.Equal(t, http.StatusOK, resp.Code, "Should allow access with valid token")
	assert.Equal(t, "protected content", resp.Body.String(), "Should return protected content")
}

func TestAdminMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup
	handlers, router := setupTestHandlers(t)

	// Create test route with admin middleware
	router.GET("/admin", handlers.AuthMiddleware(), handlers.AdminMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "admin content")
	})

	// Test case 1: Regular user (non-admin)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  1,
		"email":    "test@example.com",
		"username": "testuser",
		"is_admin": false,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(handlers.JWTSecret))

	req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt_token",
		Value: tokenString,
	})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to dashboard
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Equal(t, "/dashboard", resp.Header().Get("Location"))

	// Test case 2: Admin user
	adminToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  2,
		"email":    "admin@example.com",
		"username": "admin",
		"is_admin": true,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	adminTokenString, _ := adminToken.SignedString([]byte(handlers.JWTSecret))

	req, _ = http.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt_token",
		Value: adminTokenString,
	})
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should allow access
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "admin content", resp.Body.String())
}

func TestAPIAuthMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup
	handlers, router := setupTestHandlers(t)

	// Create test route with API auth middleware
	router.GET("/api/test", handlers.APIAuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// Test case 1: No Authorization header
	req, _ := http.NewRequest(http.MethodGet, "/api/test", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should return 401 Unauthorized
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Contains(t, resp.Body.String(), "Authorization header is required")

	// Test case 2: Invalid Authorization format
	req, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should return 401 Unauthorized
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Contains(t, resp.Body.String(), "Authorization header format must be Bearer")

	// Test case 3: Invalid token
	req, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should return 401 Unauthorized
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Contains(t, resp.Body.String(), "Invalid or expired token")

	// Test case 4: Valid token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  1,
		"email":    "test@example.com",
		"username": "testuser",
		"is_admin": false,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(handlers.JWTSecret))

	req, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should allow access
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "success")
}

func TestAPIAdminMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup
	handlers, router := setupTestHandlers(t)

	// Create test route with API auth and admin middleware
	router.GET("/api/admin", handlers.APIAuthMiddleware(), handlers.APIAdminMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "admin success"})
	})

	// Test case 1: Regular user (non-admin)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  1,
		"email":    "test@example.com",
		"username": "testuser",
		"is_admin": false,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(handlers.JWTSecret))

	req, _ := http.NewRequest(http.MethodGet, "/api/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should return 403 Forbidden
	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Contains(t, resp.Body.String(), "Admin privileges required")

	// Test case 2: Admin user
	adminToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  2,
		"email":    "admin@example.com",
		"username": "admin",
		"is_admin": true,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	adminTokenString, _ := adminToken.SignedString([]byte(handlers.JWTSecret))

	req, _ = http.NewRequest(http.MethodGet, "/api/admin", nil)
	req.Header.Set("Authorization", "Bearer "+adminTokenString)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should allow access
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "admin success")
}

func TestGenerateJWT(t *testing.T) {
	// Setup
	handlers, _ := setupTestHandlers(t)
	handlers.JWTSecret = "test-jwt-secret"

	// Generate JWT
	token, err := handlers.GenerateJWT(1, "testuser", false)

	// Check token was generated
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate token
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(handlers.JWTSecret), nil
	})

	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	// Check claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, float64(1), claims["user_id"])
	assert.Equal(t, "testuser", claims["username"])
	assert.Equal(t, false, claims["is_admin"])
	assert.NotEmpty(t, claims["exp"])
}

func TestHandleLoginPage(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup
	handlers, router := setupTestHandlers(t)

	// Add route
	router.GET("/login", handlers.HandleLoginPage)

	// Test case 1: Basic login page
	req, _ := http.NewRequest(http.MethodGet, "/login", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Login - GoMFT")
	assert.Contains(t, resp.Body.String(), "Sign In")
	assert.Contains(t, resp.Body.String(), "Access your GoMFT account")

	// Test case 2: Login page with message
	req, _ = http.NewRequest(http.MethodGet, "/login?message=Password+expired", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Password expired")
}

func TestHandleLogin(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup database and test user
	database := testutils.SetupTestDB(t)

	// Create test user with password "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := &db.User{
		Email:               "test@example.com",
		PasswordHash:        string(hashedPassword),
		IsAdmin:             false,
		FailedLoginAttempts: 0,
		AccountLocked:       false,
		LastPasswordChange:  time.Now(),
	}
	database.Create(user)

	// Setup handlers
	handlers := &Handlers{
		DB:        database,
		JWTSecret: "test-jwt-secret",
	}

	// Setup router
	router := gin.New()
	router.POST("/login", handlers.HandleLogin)

	// Test case 1: Successful login
	formData := url.Values{
		"email":    {"test@example.com"},
		"password": {"password123"},
	}
	req, _ := http.NewRequest(http.MethodPost, "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to dashboard
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Equal(t, "/dashboard", resp.Header().Get("Location"))

	// Should set JWT cookie
	cookies := resp.Result().Cookies()
	var jwtCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "jwt_token" {
			jwtCookie = cookie
			break
		}
	}
	assert.NotNil(t, jwtCookie)
	assert.NotEmpty(t, jwtCookie.Value)

	// Test case 2: Invalid password
	formData = url.Values{
		"email":    {"test@example.com"},
		"password": {"wrongpassword"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show error message
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Invalid credentials")

	// Test case 3: Non-existent user
	formData = url.Values{
		"email":    {"nonexistent@example.com"},
		"password": {"password123"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show error message
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Invalid credentials")
}

func TestHandleLogout(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup
	handlers, router := setupTestHandlers(t)

	// Add route
	router.GET("/logout", handlers.HandleLogout)

	// Create request
	req, _ := http.NewRequest(http.MethodGet, "/logout", nil)
	resp := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusFound, resp.Code, "Should redirect")
	assert.Equal(t, "/login", resp.Header().Get("Location"), "Should redirect to login page")

	// Check that cookie is cleared
	cookies := resp.Result().Cookies()
	found := false
	for _, cookie := range cookies {
		if cookie.Name == "jwt_token" {
			assert.Equal(t, "", cookie.Value, "JWT cookie should be cleared")
			assert.True(t, cookie.Expires.Before(time.Now()), "Cookie should be expired")
			found = true
			break
		}
	}
	assert.True(t, found, "Should find jwt_token cookie in response")
}

func TestHandleChangePassword(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup database and test user
	database := testutils.SetupTestDB(t)

	// Create test user with password "OldPassword123!"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPassword123!"), bcrypt.DefaultCost)
	user := &db.User{
		Email:               "test@example.com",
		PasswordHash:        string(hashedPassword),
		IsAdmin:             false,
		FailedLoginAttempts: 0,
		AccountLocked:       false,
		LastPasswordChange:  time.Now().Add(-24 * time.Hour), // 1 day ago
	}
	database.Create(user)

	// Setup handlers with email mock
	mockEmail := email.NewMockService()
	handlers := &Handlers{
		DB:        database,
		JWTSecret: "test-jwt-secret",
		Email:     mockEmail,
	}

	// Create JWT token for this user
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"email":    user.Email,
		"username": "testuser",
		"is_admin": false,
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(handlers.JWTSecret))

	// Setup router
	router := gin.New()
	router.POST("/change-password", handlers.HandleChangePassword)

	// Test case 1: Successful password change
	formData := url.Values{
		"current_password": {"OldPassword123!"},
		"new_password":     {"NewPassword456@"},
		"confirm_password": {"NewPassword456@"},
	}
	req, _ := http.NewRequest(http.MethodPost, "/change-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  "jwt_token",
		Value: tokenString,
	})
	req.Header.Set("HX-Request", "true") // Simulate HTMX request
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show success message
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Password updated successfully")
	assert.Contains(t, resp.Body.String(), "bg-green-100")
	assert.Contains(t, resp.Body.String(), "border-green-400")

	// Verify password was updated in the database
	var updatedUser db.User
	err := database.First(&updatedUser, user.ID).Error
	assert.NoError(t, err, "Should be able to find the user")

	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte("NewPassword456@"))
	assert.NoError(t, err, "Password should be updated in the database")

	// Test case 2: Incorrect current password
	formData = url.Values{
		"current_password": {"WrongPassword123!"},
		"new_password":     {"AnotherPassword789#"},
		"confirm_password": {"AnotherPassword789#"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/change-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  "jwt_token",
		Value: tokenString,
	})
	req.Header.Set("HX-Request", "true") // Simulate HTMX request
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show error message
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Current password is incorrect")
	assert.Contains(t, resp.Body.String(), "bg-red-100")
	assert.Contains(t, resp.Body.String(), "border-red-400")

	// Test case 3: Passwords don't match
	formData = url.Values{
		"current_password": {"NewPassword456@"}, // Using the updated password
		"new_password":     {"DiffPassword123!"},
		"confirm_password": {"DiffPassword456@"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/change-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  "jwt_token",
		Value: tokenString,
	})
	req.Header.Set("HX-Request", "true") // Simulate HTMX request
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show error message
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "New password and confirmation do not match")
	assert.Contains(t, resp.Body.String(), "bg-red-100")
	assert.Contains(t, resp.Body.String(), "border-red-400")
}

func TestHandleForgotPasswordPage(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup
	handlers, router := setupTestHandlers(t)

	// Add route
	router.GET("/forgot-password", handlers.HandleForgotPasswordPage)

	// Create request
	req, _ := http.NewRequest(http.MethodGet, "/forgot-password", nil)
	resp := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Forgot Password - GoMFT")
	assert.Contains(t, resp.Body.String(), "Password Reset")
	assert.Contains(t, resp.Body.String(), "Enter your email to receive a reset link")
}

func TestHandleForgotPassword(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup database and test user
	database := testutils.SetupTestDB(t)

	// Create test user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := &db.User{
		Email:              "test@example.com",
		PasswordHash:       string(hashedPassword),
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(user)

	// Setup handlers with email mock
	mockEmail := email.NewMockService()
	handlers := &Handlers{
		DB:        database,
		JWTSecret: "test-jwt-secret",
		Email:     mockEmail,
	}

	// Setup router
	router := gin.New()
	router.POST("/forgot-password", handlers.HandleForgotPassword)

	// Test case 1: Valid email
	formData := url.Values{
		"email": {"test@example.com"},
	}
	req, _ := http.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show generic success message
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "If your email is registered")

	// Check if reset token was created
	var resetToken db.PasswordResetToken
	result := database.Where("user_id = ?", user.ID).First(&resetToken)
	assert.NoError(t, result.Error, "Reset token should be created")
	assert.NotEmpty(t, resetToken.Token, "Token should not be empty")
	assert.False(t, resetToken.Used, "Token should not be marked as used")

	// Verify email would have been sent (if not mocked)
	// Note: We can't check SendPasswordResetEmailCalls with our current mock
	// assert.Equal(t, 1, mockEmail.SendPasswordResetEmailCalls)

	// Test case 2: Non-existent email
	formData = url.Values{
		"email": {"nonexistent@example.com"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show generic success message (even though user doesn't exist)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "If your email is registered")

	// Test case 3: Missing email
	formData = url.Values{}
	req, _ = http.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show error message
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Email is required")
}

func TestHandleResetPasswordPage(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup database
	database := testutils.SetupTestDB(t)

	// Create test user
	user := &db.User{
		Email:              "test@example.com",
		PasswordHash:       "hashedpassword",
		IsAdmin:            false,
		LastPasswordChange: time.Now(),
	}
	database.Create(user)

	// Create reset token
	token := "valid-reset-token"
	resetToken := &db.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}
	database.Create(resetToken)

	// Setup handlers
	handlers := &Handlers{
		DB: database,
	}

	// Setup router
	router := gin.New()
	router.GET("/reset-password", handlers.HandleResetPasswordPage)

	// Test case 1: Valid token
	req, _ := http.NewRequest(http.MethodGet, "/reset-password?token="+token, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show reset password form
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Reset Password")
	assert.Contains(t, resp.Body.String(), token) // Token should be in the form

	// Test case 2: No token
	req, _ = http.NewRequest(http.MethodGet, "/reset-password", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to forgot password page
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Equal(t, "/forgot-password", resp.Header().Get("Location"))

	// Test case 3: Invalid token
	req, _ = http.NewRequest(http.MethodGet, "/reset-password?token=invalid-token", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to forgot password page
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Equal(t, "/forgot-password", resp.Header().Get("Location"))
}

func TestHandleResetPassword(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup database
	database := testutils.SetupTestDB(t)

	// Create test user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"), bcrypt.DefaultCost)
	user := &db.User{
		Email:              "test@example.com",
		PasswordHash:       string(hashedPassword),
		IsAdmin:            false,
		LastPasswordChange: time.Now().Add(-24 * time.Hour), // 1 day ago
	}
	database.Create(user)

	// Create reset token
	token := "valid-reset-token"
	resetToken := &db.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}
	database.Create(resetToken)

	// Setup handlers
	handlers := &Handlers{
		DB: database,
	}

	// Setup router
	router := gin.New()
	router.POST("/reset-password", handlers.HandleResetPassword)

	// Test case 1: Successful password reset
	formData := url.Values{
		"token":            {token},
		"password":         {"newpassword123"},
		"confirm-password": {"newpassword123"},
	}
	req, _ := http.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to login with success message
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Contains(t, resp.Header().Get("Location"), "/login?message=Password+reset+successful")

	// Verify password was updated
	var updatedUser db.User
	database.First(&updatedUser, user.ID)
	err := bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte("newpassword123"))
	assert.NoError(t, err, "Password should be updated in the database")

	// Verify token is marked as used
	var updatedToken db.PasswordResetToken
	database.First(&updatedToken, resetToken.ID)
	assert.True(t, updatedToken.Used, "Token should be marked as used")

	// Test case 2: Passwords don't match
	// Create another token first
	token2 := "another-valid-token"
	resetToken2 := &db.PasswordResetToken{
		UserID:    user.ID,
		Token:     token2,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}
	database.Create(resetToken2)

	formData = url.Values{
		"token":            {token2},
		"password":         {"newpass1"},
		"confirm-password": {"newpass2"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show error
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Passwords do not match")

	// Test case 3: Password too short
	token3 := "yet-another-valid-token"
	resetToken3 := &db.PasswordResetToken{
		UserID:    user.ID,
		Token:     token3,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}
	database.Create(resetToken3)

	formData = url.Values{
		"token":            {token3},
		"password":         {"short"},
		"confirm-password": {"short"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should show error
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Password must be at least 8 characters long")

	// Test case 4: No token
	formData = url.Values{
		"password":         {"validpassword"},
		"confirm-password": {"validpassword"},
	}
	req, _ = http.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should redirect to forgot password page
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Equal(t, "/forgot-password", resp.Header().Get("Location"))
}
