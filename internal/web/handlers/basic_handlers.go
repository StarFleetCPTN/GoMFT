package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/auth"
)

// HandleHome handles the GET / route
func (h *Handlers) HandleHome(c *gin.Context) {
	// Check for JWT token in cookie
	tokenCookie, err := c.Cookie("jwt_token")
	if err == nil && tokenCookie != "" {
		// Token exists, validate it
		claims, err := auth.ValidateToken(tokenCookie, h.JWTSecret)
		if err == nil && claims != nil {
			// Valid token, redirect to dashboard
			c.Redirect(http.StatusFound, "/dashboard")
			return
		}
	}
	
	// User is not logged in, show home page
	components.Home(c.Request.Context()).Render(c, c.Writer)
} 