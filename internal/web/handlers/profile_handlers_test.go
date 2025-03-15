package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func setupProfileTest(t *testing.T) (*Handlers, *gin.Engine, *db.DB, *db.User) {
	// Set up test database
	database := testutils.SetupTestDB(t)

	// Create test user
	user := testutils.CreateTestUser(t, database, "test@example.com", false)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handlers
	handlers := &Handlers{
		DB: database,
	}

	// Set up authentication middleware
	router.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})

	return handlers, router, database, user
}

func TestHandleProfile(t *testing.T) {
	handlers, router, _, user := setupProfileTest(t)

	// Set up route
	router.GET("/profile", handlers.HandleProfile)

	// Create request
	req, _ := http.NewRequest("GET", "/profile", nil)
	resp := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(resp, req)

	// Check response
	assert.Equal(t, http.StatusOK, resp.Code)

	// Check profile content
	assert.Contains(t, resp.Body.String(), user.Email)

	// Test with non-existent user
	invalidRouter := gin.New()
	invalidRouter.Use(func(c *gin.Context) {
		c.Set("userID", uint(9999)) // Non-existent user ID
		c.Next()
	})
	invalidRouter.GET("/profile", handlers.HandleProfile)

	req, _ = http.NewRequest("GET", "/profile", nil)
	resp = httptest.NewRecorder()
	invalidRouter.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "Failed to retrieve user profile")
}

func TestHandleUpdateTheme(t *testing.T) {
	handlers, router, database, user := setupProfileTest(t)

	// Set up route
	router.POST("/profile/theme", handlers.HandleUpdateTheme)

	// Test cases
	testCases := []struct {
		name         string
		theme        string
		expectedCode int
	}{
		{
			name:         "Valid light theme",
			theme:        "light",
			expectedCode: http.StatusOK,
		},
		{
			name:         "Valid dark theme",
			theme:        "dark",
			expectedCode: http.StatusOK,
		},
		{
			name:         "Valid system theme",
			theme:        "system",
			expectedCode: http.StatusOK,
		},
		{
			name:         "Invalid theme",
			theme:        "invalid",
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build form data
			formData := url.Values{
				"theme": {tc.theme},
			}

			// Create request
			req, _ := http.NewRequest("POST", "/profile/theme", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(resp, req)

			// Check response code
			assert.Equal(t, tc.expectedCode, resp.Code)

			// If valid theme, check that user's theme was updated
			if tc.expectedCode == http.StatusOK {
				// Fetch the user from the database
				var updatedUser db.User
				err := database.First(&updatedUser, user.ID).Error
				assert.NoError(t, err)

				// Check that theme was updated
				assert.Equal(t, tc.theme, updatedUser.Theme)

				// Check that theme cookie was set
				cookies := resp.Result().Cookies()
				var themeCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == "theme" {
						themeCookie = cookie
						break
					}
				}
				assert.NotNil(t, themeCookie)
				assert.Equal(t, tc.theme, themeCookie.Value)
				assert.Equal(t, 60*60*24*365, themeCookie.MaxAge) // 1 year
			}
		})
	}

	// Test with non-existent user
	invalidRouter := gin.New()
	invalidRouter.Use(func(c *gin.Context) {
		c.Set("userID", uint(9999)) // Non-existent user ID
		c.Next()
	})
	invalidRouter.POST("/profile/theme", handlers.HandleUpdateTheme)

	formData := url.Values{
		"theme": {"light"},
	}

	req, _ := http.NewRequest("POST", "/profile/theme", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	invalidRouter.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}
