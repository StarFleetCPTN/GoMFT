package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleHome(t *testing.T) {
	// Set up test environment
	handlers, router := setupTestHandlers(t)

	// Set up the route
	router.GET("/", handlers.HandleHome)

	// Create a test request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Home - GoMFT")
	assert.Contains(t, w.Body.String(), "Welcome to GoMFT")
}
