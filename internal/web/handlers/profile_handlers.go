package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleProfile handles the GET /profile route
func (h *Handlers) HandleProfile(c *gin.Context) {
	userID := c.GetUint("userID")
	var user db.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to retrieve user profile")
		return
	}
	components.Profile(c.Request.Context(), user).Render(c, c.Writer)
}

// HandleUpdateTheme handles the POST /profile/theme route
func (h *Handlers) HandleUpdateTheme(c *gin.Context) {
	userID := c.GetUint("userID")
	theme := c.PostForm("theme")

	// Validate theme value
	validThemes := map[string]bool{
		"light":  true,
		"dark":   true,
		"system": true,
	}

	if !validThemes[theme] {
		c.Status(http.StatusBadRequest)
		return
	}

	// Update user theme preference
	var user db.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	user.Theme = theme
	if err := h.DB.Save(&user).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Set theme cookie for client-side theme switching
	c.SetCookie("theme", theme, 60*60*24*365, "/", "", false, false)

	c.Status(http.StatusOK)
}
