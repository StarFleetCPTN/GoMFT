package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/testutils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupUserTest(t *testing.T) (*Handlers, *gin.Engine, *db.DB, uint) {
	// Set up test database
	database := testutils.SetupTestDB(t)

	// Create admin user
	admin := testutils.CreateTestUser(t, database, "admin@example.com", true)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handlers with JWT configuration
	config := testutils.SetupTestConfig(t)
	handlers := &Handlers{
		DB:        database,
		JWTSecret: config.JWTSecret,
	}

	// Set up authentication middleware for admins
	router.Use(func(c *gin.Context) {
		c.Set("userID", admin.ID)
		c.Set("isAdmin", true)
		c.Next()
	})

	return handlers, router, database, admin.ID
}

func TestHandleUsers(t *testing.T) {
	handlers, router, database, _ := setupUserTest(t)

	// Create additional test users
	testutils.CreateTestUser(t, database, "user1@example.com", false)
	testutils.CreateTestUser(t, database, "user2@example.com", false)

	// Set up route
	router.GET("/admin/users", handlers.HandleUsers)

	// Create request
	req, _ := http.NewRequest("GET", "/admin/users", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	// Should contain the admin and two test users
	assert.Contains(t, resp.Body.String(), "admin@example.com")
	assert.Contains(t, resp.Body.String(), "user1@example.com")
	assert.Contains(t, resp.Body.String(), "user2@example.com")
}

func TestHandleNewUser(t *testing.T) {
	handlers, router, _, _ := setupUserTest(t)

	// Set up route
	router.GET("/admin/users/new", handlers.HandleNewUser)

	// Create request
	req, _ := http.NewRequest("GET", "/admin/users/new", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "New User")
	assert.Contains(t, resp.Body.String(), "Email")
	assert.Contains(t, resp.Body.String(), "Password")
	assert.Contains(t, resp.Body.String(), "Admin")
}

func TestHandleCreateUser(t *testing.T) {
	handlers, router, database, _ := setupUserTest(t)

	// Set up route
	router.POST("/admin/users/new", handlers.HandleCreateUser)

	// Test cases
	testCases := []struct {
		name         string
		formData     url.Values
		expectedCode int
		checkUser    bool
	}{
		{
			name: "Valid user creation",
			formData: url.Values{
				"email":    {"newuser@example.com"},
				"password": {"testpassword"},
				"is_admin": {"on"},
			},
			expectedCode: http.StatusSeeOther,
			checkUser:    true,
		},
		{
			name: "Duplicate email",
			formData: url.Values{
				"email":    {"admin@example.com"}, // Already exists
				"password": {"testpassword"},
			},
			expectedCode: http.StatusBadRequest,
			checkUser:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request with form data
			req, _ := http.NewRequest("POST", "/admin/users/new", strings.NewReader(tc.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(resp, req)

			// Check response code
			assert.Equal(t, tc.expectedCode, resp.Code)

			// If we expect user creation, verify the user exists in the database
			if tc.checkUser {
				var user db.User
				err := database.Where("email = ?", tc.formData.Get("email")).First(&user).Error
				assert.NoError(t, err)
				assert.Equal(t, tc.formData.Get("email"), user.Email)
				assert.Equal(t, tc.formData.Get("is_admin") == "on", user.IsAdmin)
			}
		})
	}
}

func TestHandleDeleteUser(t *testing.T) {
	handlers, router, database, adminID := setupUserTest(t)

	// Create a user to delete
	userToDelete := testutils.CreateTestUser(t, database, "delete-me@example.com", false)

	// Set up route
	router.POST("/admin/users/delete/:id", handlers.HandleDeleteUser)

	// Test cases
	testCases := []struct {
		name         string
		userID       uint
		expectedCode int
		userDeleted  bool
	}{
		{
			name:         "Delete valid user",
			userID:       userToDelete.ID,
			expectedCode: http.StatusSeeOther,
			userDeleted:  true,
		},
		{
			name:         "Cannot delete own account",
			userID:       adminID,
			expectedCode: http.StatusBadRequest,
			userDeleted:  false,
		},
		{
			name:         "Invalid user ID",
			userID:       9999,                // Doesn't exist
			expectedCode: http.StatusSeeOther, // Gorm soft delete doesn't error on non-existent IDs
			userDeleted:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, _ := http.NewRequest("POST", "/admin/users/delete/"+strconv.Itoa(int(tc.userID)), nil)
			resp := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(resp, req)

			// Check response code
			assert.Equal(t, tc.expectedCode, resp.Code)

			// Check if the user exists in the database
			var user db.User
			result := database.Unscoped().Where("id = ?", tc.userID).First(&user)

			if tc.userDeleted {
				// For deleted users, check that they exist but are deleted
				assert.NoError(t, result.Error)
				// Check for deletion status using Gorm's DeletedAt field
				assert.True(t, database.Unscoped().Where("id = ?", tc.userID).Where("deleted_at IS NOT NULL").First(&user).Error == nil)
			} else if tc.userID != 9999 { // Skip check for non-existent user
				// For non-deleted users, they should exist and not be soft-deleted
				assert.NoError(t, result.Error)
				assert.Equal(t, gorm.ErrRecordNotFound, database.Unscoped().Where("id = ?", tc.userID).Where("deleted_at IS NOT NULL").First(&user).Error)
			}
		})
	}
}

func TestHandleRegisterPage(t *testing.T) {
	// Set up clean database with no users
	database := testutils.SetupTestDB(t)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handlers
	handlers := &Handlers{
		DB: database,
	}

	// Set up route
	router.GET("/register", handlers.HandleRegisterPage)

	// Test with no existing users - should show registration page
	req, _ := http.NewRequest("GET", "/register", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Register")

	// Create a user
	testutils.CreateTestUser(t, database, "existing@example.com", true)

	// Test with existing user - should redirect
	req, _ = http.NewRequest("GET", "/register", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusSeeOther, resp.Code)
	assert.Equal(t, "/", resp.Header().Get("Location"))
}

func TestHandleRegister(t *testing.T) {
	// Set up clean database with no users
	database := testutils.SetupTestDB(t)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handlers with JWT configuration
	config := testutils.SetupTestConfig(t)
	handlers := &Handlers{
		DB:        database,
		JWTSecret: config.JWTSecret,
	}

	// Set up route
	router.POST("/register", handlers.HandleRegister)

	// Test user registration
	formData := url.Values{
		"email":    {"firstuser@example.com"},
		"password": {"testpassword"},
	}

	req, _ := http.NewRequest("POST", "/register", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should redirect to dashboard
	assert.Equal(t, http.StatusSeeOther, resp.Code)
	assert.Equal(t, "/dashboard", resp.Header().Get("Location"))

	// Verify user was created as admin
	var user db.User
	err := database.Where("email = ?", formData.Get("email")).First(&user).Error
	assert.NoError(t, err)
	assert.Equal(t, formData.Get("email"), user.Email)
	assert.True(t, user.IsAdmin)

	// Verify JWT cookie was set
	cookies := resp.Result().Cookies()
	assert.GreaterOrEqual(t, len(cookies), 1)
	var jwtCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "jwt" {
			jwtCookie = cookie
			break
		}
	}
	assert.NotNil(t, jwtCookie)
	assert.NotEmpty(t, jwtCookie.Value)

	// Try registering a second user - should be redirected
	formData = url.Values{
		"email":    {"seconduser@example.com"},
		"password": {"testpassword"},
	}

	req, _ = http.NewRequest("POST", "/register", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should redirect to home
	assert.Equal(t, http.StatusSeeOther, resp.Code)
	assert.Equal(t, "/", resp.Header().Get("Location"))

	// Second user should not exist
	var count int64
	database.Model(&db.User{}).Where("email = ?", formData.Get("email")).Count(&count)
	assert.Equal(t, int64(0), count)
}
