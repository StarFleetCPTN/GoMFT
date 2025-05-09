package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
	"golang.org/x/crypto/bcrypt"
)

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

	ctx := h.CreateTemplateContext(c)
	components.Register(ctx, "").Render(ctx, c.Writer)
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

	// Create the user
	user := db.User{
		Email:              email,
		PasswordHash:       string(hashedPassword),
		LastPasswordChange: time.Now(),
	}
	// Set as regular user (not admin)
	user.SetIsAdmin(false)

	if err := h.DB.Create(&user).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Generate JWT token
	token, err := h.GenerateJWT(user.ID, user.Email, user.GetIsAdmin())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Set cookie
	c.SetCookie("jwt", token, 60*60*24, "/", "", false, true)

	c.Redirect(http.StatusSeeOther, "/dashboard")
}

// AdminUsersPage handles the GET /admin/users route
func (h *Handlers) AdminUsersPage(c *gin.Context) {
	var users []db.User
	if err := h.DB.Find(&users).Error; err != nil {
		c.String(http.StatusInternalServerError, "Error fetching users")
		return
	}

	// Use the Users component instead of HTML templates
	data := components.UsersData{
		Users: users,
	}

	ctx := h.CreateTemplateContext(c)
	components.Users(ctx, data).Render(ctx, c.Writer)
}

// AdminNewUserPage handles the GET /admin/users/new route
func (h *Handlers) AdminNewUserPage(c *gin.Context) {
	// Get all available roles
	var allRoles []db.Role
	if err := h.DB.Find(&allRoles).Error; err != nil {
		log.Printf("Error fetching roles: %v", err)
	}

	data := components.UserEditData{
		User:      &db.User{},
		Roles:     allRoles,
		UserRoles: []db.Role{},
		IsNew:     true,
	}

	ctx := h.CreateTemplateContext(c)
	components.UserEdit(ctx, data).Render(ctx, c.Writer)
}

// AdminCreateUser handles the POST /admin/users route
func (h *Handlers) AdminCreateUser(c *gin.Context) {
	var user db.User
	if err := c.ShouldBind(&user); err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid form data: %v", err))
		return
	}

	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")

	if password != passwordConfirm {
		c.String(http.StatusBadRequest, "Passwords do not match")
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error hashing password")
		return
	}
	user.PasswordHash = string(hashedPassword)

	// Set defaults
	isAdmin := false
	accountLocked := false
	user.IsAdmin = &isAdmin
	user.AccountLocked = &accountLocked
	user.LastPasswordChange = time.Now()

	// Process roles
	selectedRoleIDs := c.PostFormArray("roles[]")

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		c.String(http.StatusInternalServerError, "Failed to begin transaction")
		return
	}

	// Save the user
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", err))
		return
	}

	// Add role assignments
	adminID := c.GetUint("userID")
	for _, roleIDStr := range selectedRoleIDs {
		roleID, err := strconv.ParseUint(roleIDStr, 10, 32)
		if err != nil {
			continue
		}
		if err := user.AssignRole(tx, uint(roleID), adminID); err != nil {
			log.Printf("Error assigning role %d to user %d: %v", roleID, user.ID, err)
		}
	}

	// Create audit log for user creation
	auditDetails := map[string]interface{}{
		"email":    user.Email,
		"is_admin": *user.IsAdmin,
		"roles":    selectedRoleIDs,
	}

	auditLog := db.AuditLog{
		Action:     "create",
		EntityType: "user",
		EntityID:   user.ID,
		UserID:     adminID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		c.String(http.StatusInternalServerError, "Failed to create audit log")
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

// AdminUpdateUser handles the PUT /admin/users/:id route
func (h *Handlers) AdminUpdateUser(c *gin.Context) {
	id := c.Param("id")
	adminID := c.GetUint("userID")

	var user db.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.String(http.StatusNotFound, "User not found")
		return
	}

	// Store original user state for audit log
	oldUser := user

	// Get original roles for comparison
	oldRoles, err := h.DB.GetUserRoles(user.ID)
	if err != nil {
		log.Printf("Error fetching original user roles: %v", err)
	}
	oldRoleIDs := make([]uint, len(oldRoles))
	for i, role := range oldRoles {
		oldRoleIDs[i] = role.ID
	}

	// Update user with form data
	email := c.PostForm("email")
	if email != "" {
		user.Email = email
	}

	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")

	if password != "" {
		if password != passwordConfirm {
			c.String(http.StatusBadRequest, "Passwords do not match")
			return
		}

		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error hashing password")
			return
		}
		user.PasswordHash = string(hashedPassword)
		user.LastPasswordChange = time.Now()
	}

	// Update admin status
	isAdminStr := c.PostForm("is_admin")
	isAdmin := isAdminStr == "on" || isAdminStr == "true"
	user.SetIsAdmin(isAdmin)

	// Update locked status
	accountLockedStr := c.PostForm("account_locked")
	accountLocked := accountLockedStr == "on" || accountLockedStr == "true"
	user.SetAccountLocked(accountLocked)

	// Get selected roles
	selectedRoleIDs := c.PostFormArray("roles[]")

	// Convert role IDs to uint
	var newRoleIDs []uint
	for _, roleIDStr := range selectedRoleIDs {
		roleID, err := strconv.ParseUint(roleIDStr, 10, 32)
		if err != nil {
			continue
		}
		newRoleIDs = append(newRoleIDs, uint(roleID))
	}

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		c.String(http.StatusInternalServerError, "Failed to begin transaction")
		return
	}

	// Save the user
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to update user: %v", err))
		return
	}

	// Get current role assignments to determine changes
	currentRoles, err := h.DB.GetUserRoles(user.ID)
	if err != nil {
		log.Printf("Error fetching current user roles: %v", err)
		tx.Rollback()
		c.String(http.StatusInternalServerError, "Failed to fetch current roles")
		return
	}

	// Map of current role IDs
	currentRoleIDs := make(map[uint]bool)
	for _, role := range currentRoles {
		currentRoleIDs[role.ID] = true
	}

	// Map of new role IDs
	newRoleIDsMap := make(map[uint]bool)
	for _, roleID := range newRoleIDs {
		newRoleIDsMap[roleID] = true
	}

	// Remove roles that are no longer selected
	for _, role := range currentRoles {
		if !newRoleIDsMap[role.ID] {
			if err := user.UnassignRole(tx, role.ID, adminID); err != nil {
				log.Printf("Error unassigning role %d from user %d: %v", role.ID, user.ID, err)
			}
		}
	}

	// Add newly selected roles
	for _, roleID := range newRoleIDs {
		if !currentRoleIDs[roleID] {
			if err := user.AssignRole(tx, roleID, adminID); err != nil {
				log.Printf("Error assigning role %d to user %d: %v", roleID, user.ID, err)
			}
		}
	}

	// Create audit log entry
	auditDetails := map[string]interface{}{
		"email":    user.Email,
		"is_admin": *user.IsAdmin,
		"roles":    newRoleIDs,
		"previous_state": map[string]interface{}{
			"email":    oldUser.Email,
			"is_admin": *oldUser.IsAdmin,
			"roles":    oldRoleIDs,
		},
	}

	auditLog := db.AuditLog{
		Action:     "update",
		EntityType: "user",
		EntityID:   user.ID,
		UserID:     adminID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		c.String(http.StatusInternalServerError, "Failed to create audit log")
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

// AdminDeleteUser handles the DELETE /admin/users/:id route
func (h *Handlers) AdminDeleteUser(c *gin.Context) {
	id := c.Param("id")
	adminID := c.GetUint("userID")

	var user db.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Cannot delete yourself
	if user.ID == adminID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete yourself"})
		return
	}

	// Get user roles for audit log
	userRoles, err := h.DB.GetUserRoles(user.ID)
	if err != nil {
		log.Printf("Error fetching user roles: %v", err)
	}
	roleIDs := make([]uint, len(userRoles))
	for i, role := range userRoles {
		roleIDs[i] = role.ID
	}

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Create audit log before deletion
	auditDetails := map[string]interface{}{
		"email":    user.Email,
		"is_admin": *user.IsAdmin,
		"roles":    roleIDs,
	}

	auditLog := db.AuditLog{
		Action:     "delete",
		EntityType: "user",
		EntityID:   user.ID,
		UserID:     adminID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
		return
	}

	// Delete the user
	if err := tx.Delete(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete user: %v", err)})
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// AdminUserRoles handles the GET /admin/users/:id/roles route
func (h *Handlers) AdminUserRoles(c *gin.Context) {
	id := c.Param("id")

	fmt.Println("AdminUserRoles")

	var user db.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	// Get user's roles
	userRoles, err := h.DB.GetUserRoles(user.ID)
	if err != nil {
		log.Printf("Error fetching user roles: %v", err)
	}

	// Get all available roles
	var allRoles []db.Role
	if err := h.DB.Find(&allRoles).Error; err != nil {
		log.Printf("Error fetching roles: %v", err)
	}

	// Use the UserManagement component instead of HTML templates
	data := components.UserManagementData{
		EditUser:     &user,
		UserRoles:    userRoles,
		Roles:        allRoles,
		ActiveTab:    "roles",
		ErrorMessage: "",
	}

	ctx := h.CreateTemplateContext(c)
	components.UserManagement(ctx, data).Render(ctx, c.Writer)
}

// AdminAssignRoleToUser handles the POST /admin/users/:id/roles/:role_id route
func (h *Handlers) AdminAssignRoleToUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	adminID := c.GetUint("userID")

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Assign role
	if err := h.DB.AssignRoleToUser(uint(roleID), uint(userID), adminID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to assign role: %v", err)})
		return
	}

	// Create audit log for role assignment
	var role db.Role
	if err := tx.First(&role, roleID).Error; err == nil {
		auditDetails := map[string]interface{}{
			"user_id":   userID,
			"role_id":   roleID,
			"role_name": role.Name,
		}

		auditLog := db.AuditLog{
			Action:     "assign_role",
			EntityType: "user_role",
			EntityID:   uint(userID),
			UserID:     adminID,
			Details:    auditDetails,
			Timestamp:  time.Now(),
		}

		if err := tx.Create(&auditLog).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
			return
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully"})
}

// AdminUnassignRoleFromUser handles the DELETE /admin/users/:id/roles/:role_id route
func (h *Handlers) AdminUnassignRoleFromUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	adminID := c.GetUint("userID")

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Get role name for audit log before unassigning
	var role db.Role
	var roleName string
	if err := tx.First(&role, roleID).Error; err == nil {
		roleName = role.Name
	}

	// Unassign role
	if err := h.DB.UnassignRoleFromUser(uint(roleID), uint(userID), adminID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unassign role: %v", err)})
		return
	}

	// Create audit log for role unassignment
	auditDetails := map[string]interface{}{
		"user_id":   userID,
		"role_id":   roleID,
		"role_name": roleName,
	}

	auditLog := db.AuditLog{
		Action:     "unassign_role",
		EntityType: "user_role",
		EntityID:   uint(userID),
		UserID:     adminID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role unassigned successfully"})
}

// AdminToggleLockUser handles the PUT /admin/users/:id/toggle-lock route
func (h *Handlers) AdminToggleLockUser(c *gin.Context) {
	id := c.Param("id")
	adminID := c.GetUint("userID")

	var user db.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%s/edit?error=User+not+found&details=%s", id, url.QueryEscape(err.Error())))
		return
	}

	// Store current state for audit log
	currentLockState := user.GetAccountLocked()

	// Toggle the locked state
	newLockState := !currentLockState
	user.SetAccountLocked(newLockState)

	// Also reset failed login attempts if unlocking
	if !newLockState {
		user.FailedLoginAttempts = 0
		user.LockoutUntil = nil
	}

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%s/edit?error=Database+error&details=%s", id, url.QueryEscape(tx.Error.Error())))
		return
	}

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%s/edit?error=Failed+to+update+user&details=%s", id, url.QueryEscape(err.Error())))
		return
	}

	// Create audit log entry
	auditDetails := map[string]interface{}{
		"account_locked": newLockState,
		"previous_state": map[string]interface{}{
			"account_locked": currentLockState,
		},
	}

	action := "unlock_account"
	if newLockState {
		action = "lock_account"
	}

	auditLog := db.AuditLog{
		Action:     action,
		EntityType: "user",
		EntityID:   user.ID,
		UserID:     adminID,
		Details:    auditDetails,
		Timestamp:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		tx.Rollback()
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%s/edit?error=Failed+to+create+audit+log&details=%s", id, url.QueryEscape(err.Error())))
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%s/edit?error=Failed+to+commit+transaction&details=%s", id, url.QueryEscape(err.Error())))
		return
	}

	// Redirect back to the edit page with a status message
	statusMsg := "User account unlocked successfully"
	if newLockState {
		statusMsg = "User account locked successfully"
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/users/%s/edit?status=%s", id, url.QueryEscape(statusMsg)))
}

// CreateTemplateContext creates a context for the HTMX template rendering
func (h *Handlers) CreateTemplateContext(c *gin.Context) context.Context {
	return components.CreateTemplateContext(c)
}
