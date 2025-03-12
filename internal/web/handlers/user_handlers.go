package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// HandleUsers handles the GET /admin/users route
func (h *Handlers) HandleUsers(c *gin.Context) {
	var users []db.User
	if err := h.DB.Find(&users).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to retrieve users")
		return
	}
	data := components.UsersData{
		Users: users,
	}
	components.Users(components.CreateTemplateContext(c), data).Render(c, c.Writer)
}

// HandleNewUser handles the GET /admin/users/new route
func (h *Handlers) HandleNewUser(c *gin.Context) {
	data := components.UserFormData{
		IsNew:        true,
		ErrorMessage: "",
	}
	components.UserForm(components.CreateTemplateContext(c), data).Render(c, c.Writer)
}

// HandleCreateUser handles the POST /admin/users/new route
func (h *Handlers) HandleCreateUser(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	isAdmin := c.PostForm("is_admin") == "on"

	// Check if email already exists
	var existingUser db.User
	if err := h.DB.Where("email = ?", email).First(&existingUser).Error; err == nil {
		c.String(http.StatusBadRequest, "Email already exists")
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Create the user
	user := db.User{
		Email:              email,
		PasswordHash:       string(hashedPassword),
		IsAdmin:            isAdmin,
		LastPasswordChange: time.Now(),
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to create user")
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/users")
}

// HandleDeleteUser handles the POST /admin/users/delete route
func (h *Handlers) HandleDeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Don't allow deleting the current user
	currentUserID := c.GetUint("userID")
	if uint(userID) == currentUserID {
		c.String(http.StatusBadRequest, "Cannot delete your own account")
		return
	}

	// Delete the user
	if err := h.DB.Delete(&db.User{}, userID).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete user")
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/users")
}

// HandleRegisterPage handles the GET /register route
func (h *Handlers) HandleRegisterPage(c *gin.Context) {
	// Check if any users exist
	var count int64
	h.DB.Model(&db.User{}).Count(&count)

	// If users exist, don't allow registration
	if count > 0 {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	components.Register(c.Request.Context(), "").Render(c, c.Writer)
}

// HandleRegister handles the POST /register route
func (h *Handlers) HandleRegister(c *gin.Context) {
	// Check if any users exist
	var count int64
	h.DB.Model(&db.User{}).Count(&count)

	// If users exist, don't allow registration
	if count > 0 {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	email := c.PostForm("email")
	password := c.PostForm("password")

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Create the admin user
	user := db.User{
		Email:              email,
		PasswordHash:       string(hashedPassword),
		IsAdmin:            true,
		LastPasswordChange: time.Now(),
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Generate JWT
	token, err := h.GenerateJWT(user.ID, user.Email, user.IsAdmin)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Set cookie
	c.SetCookie("jwt", token, 60*60*24, "/", "", false, true)

	c.Redirect(http.StatusSeeOther, "/dashboard")
}
