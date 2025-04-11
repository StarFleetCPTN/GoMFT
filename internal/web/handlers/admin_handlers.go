package handlers

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleAdminDashboard renders the admin dashboard page
func (h *Handlers) HandleAdminDashboard(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	_ = components.AdminLayout(ctx).Render(c.Request.Context(), c.Writer)
}

// HandleRoles renders the role management page
func (h *Handlers) HandleRoles(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)

	// Fetch roles from the database
	var dbRoles []db.Role
	if err := h.DB.Find(&dbRoles).Error; err != nil {
		data := components.RolesData{
			Error: "Failed to fetch roles: " + err.Error(),
		}
		_ = components.AdminRoles(ctx, data).Render(ctx, c.Writer)
		return
	}

	// Convert db.Role to components.Role
	roles := make([]components.Role, len(dbRoles))
	for i, role := range dbRoles {
		roles[i] = components.Role{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Permissions: role.GetPermissions(),
		}
	}

	data := components.RolesData{
		Roles: roles,
	}

	_ = components.AdminRoles(ctx, data).Render(ctx, c.Writer)
}

// HandleNewRole renders the new role creation page
func (h *Handlers) HandleNewRole(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)

	data := components.RoleFormData{
		Role:           &components.Role{},
		IsNew:          true,
		AllPermissions: GetAllPermissions(),
	}

	_ = components.AdminRoleForm(ctx, data).Render(ctx, c.Writer)
}

// HandleCreateRole processes role creation
func (h *Handlers) HandleCreateRole(c *gin.Context) {
	name := c.PostForm("name")
	description := c.PostForm("description")
	permissions := c.PostFormArray("permissions[]")

	// Create role
	role := &db.Role{
		Name:        name,
		Description: description,
	}
	role.SetPermissions(permissions)

	// Validate role
	if err := role.Validate(); err != nil {
		ctx := components.CreateTemplateContext(c)
		data := components.RoleFormData{
			Role:           &components.Role{Name: name, Description: description, Permissions: permissions},
			IsNew:          true,
			ErrorMessage:   err.Error(),
			AllPermissions: GetAllPermissions(),
		}
		_ = components.AdminRoleForm(ctx, data).Render(ctx, c.Writer)
		return
	}

	// Start transaction
	tx := h.DB.Begin()
	if err := tx.Error; err != nil {
		handleRoleError(c, role, true, "Failed to begin transaction: "+err.Error())
		return
	}

	// Create role in database
	if err := tx.Create(role).Error; err != nil {
		tx.Rollback()
		handleRoleError(c, role, true, "Failed to create role: "+err.Error())
		return
	}

	// Create audit log
	userID := getUserID(c) // Implement this helper to get current user ID
	if err := role.AuditLog(tx, "create", userID); err != nil {
		tx.Rollback()
		handleRoleError(c, role, true, "Failed to create audit log: "+err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		handleRoleError(c, role, true, "Failed to commit transaction: "+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/admin/roles")
}

// HandleEditRole renders the role edit page
func (h *Handlers) HandleEditRole(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	id := c.Param("id")

	var role db.Role
	if err := h.DB.First(&role, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/roles")
		return
	}

	data := components.RoleFormData{
		Role: &components.Role{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Permissions: role.GetPermissions(),
		},
		IsNew:          false,
		AllPermissions: GetAllPermissions(),
	}

	_ = components.AdminRoleForm(ctx, data).Render(ctx, c.Writer)
}

// HandleUpdateRole processes role updates
func (h *Handlers) HandleUpdateRole(c *gin.Context) {
	id := c.Param("id")
	name := c.PostForm("name")
	description := c.PostForm("description")
	permissions := c.PostFormArray("permissions[]")

	// Get existing role
	var role db.Role
	if err := h.DB.First(&role, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/roles")
		return
	}

	// Check if this is a system role
	if role.IsSystemRole() {
		handleRoleError(c, &role, false, "Cannot modify system role")
		return
	}

	// Update role fields
	role.Name = name
	role.Description = description
	role.SetPermissions(permissions)

	// Validate role
	if err := role.Validate(); err != nil {
		handleRoleError(c, &role, false, err.Error())
		return
	}

	// Start transaction
	tx := h.DB.Begin()
	if err := tx.Error; err != nil {
		handleRoleError(c, &role, false, "Failed to begin transaction: "+err.Error())
		return
	}

	// Update role in database
	if err := tx.Save(&role).Error; err != nil {
		tx.Rollback()
		handleRoleError(c, &role, false, "Failed to update role: "+err.Error())
		return
	}

	// Create audit log
	userID := getUserID(c)
	if err := role.AuditLog(tx, "update", userID); err != nil {
		tx.Rollback()
		handleRoleError(c, &role, false, "Failed to create audit log: "+err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		handleRoleError(c, &role, false, "Failed to commit transaction: "+err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/admin/roles")
}

// HandleDeleteRole processes role deletion
func (h *Handlers) HandleDeleteRole(c *gin.Context) {
	id := c.Param("id")

	// Get existing role
	var role db.Role
	if err := h.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}

	// Check if this is a system role
	if role.IsSystemRole() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete system role"})
		return
	}

	// Start transaction
	tx := h.DB.Begin()
	if err := tx.Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Create audit log before deletion
	userID := getUserID(c)
	if err := role.AuditLog(tx, "delete", userID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
		return
	}

	// Delete role
	if err := tx.Delete(&role).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete role"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.Status(http.StatusOK)
}

// GetAllPermissions returns a list of all available permissions
func GetAllPermissions() []string {
	return []string{
		"users.view",
		"users.create",
		"users.edit",
		"users.delete",
		"roles.view",
		"roles.create",
		"roles.edit",
		"roles.delete",
		"configs.view",
		"configs.create",
		"configs.edit",
		"configs.delete",
		"jobs.view",
		"jobs.create",
		"jobs.edit",
		"jobs.delete",
		"jobs.run",
		"audit.view",
		"audit.export",
		"system.settings",
		"system.backup",
		"system.restore",
	}
}

// Helper function to parse uint from string
func parseUint(s string) uint {
	id, _ := strconv.ParseUint(s, 10, 32)
	return uint(id)
}

// HandleAuditLogs renders the audit logs page
func (h *Handlers) HandleAuditLogs(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)

	// Get filter parameters from query
	action := c.Query("action")
	entity := c.Query("entity")
	user := c.Query("user")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	// Get page parameter
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}

	// Items per page
	const perPage = 20

	// Initialize base query
	query := h.DB.Model(&db.AuditLog{}).Order("timestamp DESC")

	// Apply filters
	if action != "" {
		query = query.Where("action = ?", action)
	}

	if entity != "" {
		query = query.Where("entity_type = ?", entity)
	}

	if user != "" {
		// Join with users table to search by username
		query = query.Joins("LEFT JOIN users ON audit_logs.user_id = users.id").
			Where("users.email LIKE ?", "%"+user+"%")
	}

	if dateFrom != "" {
		fromDate, err := time.Parse("2006-01-02", dateFrom)
		if err == nil {
			query = query.Where("timestamp >= ?", fromDate)
		}
	}

	if dateTo != "" {
		toDate, err := time.Parse("2006-01-02", dateTo)
		if err == nil {
			// Add one day to include the end date
			toDate = toDate.Add(24 * time.Hour)
			query = query.Where("timestamp < ?", toDate)
		}
	}

	// Count total records
	var totalRecords int64
	if err := query.Count(&totalRecords).Error; err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Database error", err.Error(), err)
		return
	}

	// Calculate pagination
	totalPages := int(math.Ceil(float64(totalRecords) / float64(perPage)))
	offset := (page - 1) * perPage

	// Get logs for current page
	var dbLogs []db.AuditLog
	if err := query.Limit(perPage).Offset(offset).Find(&dbLogs).Error; err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Database error", err.Error(), err)
		return
	}

	// Get usernames for user IDs
	userIDs := make([]uint, len(dbLogs))
	for i, log := range dbLogs {
		userIDs[i] = log.UserID
	}

	var users []db.User
	if err := h.DB.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Database error", err.Error(), err)
		return
	}

	// Create map of user IDs to usernames
	userMap := make(map[uint]string)
	for _, u := range users {
		userMap[u.ID] = u.Email // Use email as username
	}

	// Convert to display format
	logs := make([]components.AuditLogEntry, len(dbLogs))
	for i, log := range dbLogs {
		// Serialize details to JSON for display
		detailsJSON, _ := json.Marshal(log.Details)

		logs[i] = components.AuditLogEntry{
			ID:             log.ID,
			Action:         log.Action,
			EntityType:     log.EntityType,
			EntityID:       log.EntityID,
			UserID:         log.UserID,
			Username:       userMap[log.UserID],
			Timestamp:      log.Timestamp,
			Details:        log.Details,
			DetailsSummary: string(detailsJSON),
		}
	}

	// Prepare data for template
	data := components.AuditLogsData{
		Logs:           logs,
		TotalPages:     totalPages,
		CurrentPage:    page,
		TotalRecords:   int(totalRecords),
		FilterAction:   action,
		FilterEntity:   entity,
		FilterUser:     user,
		FilterDateFrom: dateFrom,
		FilterDateTo:   dateTo,
	}

	_ = components.AdminAuditLogs(ctx, data).Render(ctx, c.Writer)
}

// HandleExportAuditLogs exports audit logs as CSV
func (h *Handlers) HandleExportAuditLogs(c *gin.Context) {
	// Get filter parameters from query
	action := c.Query("action")
	entity := c.Query("entity")
	user := c.Query("user")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	// Initialize base query
	query := h.DB.Model(&db.AuditLog{}).Order("timestamp DESC")

	// Apply filters
	if action != "" {
		query = query.Where("action = ?", action)
	}

	if entity != "" {
		query = query.Where("entity_type = ?", entity)
	}

	if user != "" {
		// Join with users table to search by username
		query = query.Joins("LEFT JOIN users ON audit_logs.user_id = users.id").
			Where("users.email LIKE ?", "%"+user+"%")
	}

	if dateFrom != "" {
		fromDate, err := time.Parse("2006-01-02", dateFrom)
		if err == nil {
			query = query.Where("timestamp >= ?", fromDate)
		}
	}

	if dateTo != "" {
		toDate, err := time.Parse("2006-01-02", dateTo)
		if err == nil {
			// Add one day to include the end date
			toDate = toDate.Add(24 * time.Hour)
			query = query.Where("timestamp < ?", toDate)
		}
	}

	// Get all logs matching the filters
	var dbLogs []db.AuditLog
	if err := query.Find(&dbLogs).Error; err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Database error", err.Error(), err)
		return
	}

	// Get usernames for user IDs
	userIDs := make([]uint, len(dbLogs))
	for i, log := range dbLogs {
		userIDs[i] = log.UserID
	}

	var users []db.User
	if err := h.DB.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Database error", err.Error(), err)
		return
	}

	// Create map of user IDs to usernames
	userMap := make(map[uint]string)
	for _, u := range users {
		userMap[u.ID] = u.Email // Use email as username
	}

	// Set headers for CSV download
	filename := fmt.Sprintf("audit_logs_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv")

	// Create CSV writer
	writer := csv.NewWriter(c.Writer)

	// Write header row
	headers := []string{"ID", "Timestamp", "Action", "Entity Type", "Entity ID", "User", "Details"}
	if err := writer.Write(headers); err != nil {
		h.HandleError(c, http.StatusInternalServerError, "Failed to write CSV", err.Error(), err)
		return
	}

	// Write data rows
	for _, log := range dbLogs {
		// Serialize details to JSON for CSV
		detailsJSON, _ := json.Marshal(log.Details)

		row := []string{
			fmt.Sprintf("%d", log.ID),
			log.Timestamp.Format("2006-01-02 15:04:05"),
			log.Action,
			log.EntityType,
			fmt.Sprintf("%d", log.EntityID),
			userMap[log.UserID],
			string(detailsJSON),
		}

		if err := writer.Write(row); err != nil {
			// Don't stop on error, just continue
			fmt.Printf("Failed to write CSV row: %v\n", err)
			continue
		}
	}

	// Flush writer
	writer.Flush()

	if err := writer.Error(); err != nil {
		fmt.Printf("Error flushing CSV writer: %v\n", err)
	}
}

// HandleSystemSettings renders the system settings page
func (h *Handlers) HandleSystemSettings(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	// In a real implementation, you'd fetch system settings from the database
	_ = components.AdminLayout(ctx).Render(c.Request.Context(), c.Writer)
}

// HandleUpdateSystemSettings processes system settings updates
func (h *Handlers) HandleUpdateSystemSettings(c *gin.Context) {
	// Implementation for updating system settings
	// ...
	c.Redirect(http.StatusFound, "/admin/settings")
}

// Helper function to handle role errors
func handleRoleError(c *gin.Context, role *db.Role, isNew bool, errorMessage string) {
	ctx := components.CreateTemplateContext(c)
	data := components.RoleFormData{
		Role: &components.Role{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Permissions: role.GetPermissions(),
		},
		IsNew:          isNew,
		ErrorMessage:   errorMessage,
		AllPermissions: GetAllPermissions(),
	}
	_ = components.AdminRoleForm(ctx, data).Render(ctx, c.Writer)
}

// Helper function to get current user ID from context
func getUserID(c *gin.Context) uint {
	user, exists := c.Get("user")
	if !exists {
		return 0
	}
	if u, ok := user.(*db.User); ok {
		return u.ID
	}
	return 0
}

// HandleUsers renders the user management page
func (h *Handlers) HandleUsers(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)

	// Fetch users from the database
	var users []db.User
	if err := h.DB.Find(&users).Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "list",
			ErrorMessage: "Failed to fetch users: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Preload roles for each user
	for i := range users {
		if err := h.DB.Model(&users[i]).Association("Roles").Find(&users[i].Roles); err != nil {
			// Log error but continue
			fmt.Printf("Error loading roles for user %d: %v\n", users[i].ID, err)
		}
	}

	// Get success message from flash if available
	successMsg := c.Query("success")

	data := components.UserManagementData{
		Users:          users,
		ActiveTab:      "list",
		SuccessMessage: successMsg,
	}

	if c.GetHeader("HX-Request") == "true" {
		_ = components.UserManagementContent(data).Render(ctx, c.Writer)
	} else {
		_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
	}
}

// HandleNewUser renders the new user creation page
func (h *Handlers) HandleNewUser(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)

	// Fetch available roles
	var roles []db.Role
	if err := h.DB.Find(&roles).Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Failed to fetch roles: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	data := components.UserManagementData{
		ActiveTab: "create",
		Roles:     roles,
	}

	if c.GetHeader("HX-Request") == "true" {
		_ = components.UserManagementContent(data).Render(ctx, c.Writer)
	} else {
		_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
	}
}

// HandleCreateUser processes user creation
func (h *Handlers) HandleCreateUser(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	email := c.PostForm("email")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")
	isAdmin := c.PostForm("is_admin") == "on"
	roleIDs := c.PostFormArray("roles[]")

	// Validate inputs
	if email == "" || password == "" {
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Email and password are required",
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	if password != passwordConfirm {
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Passwords do not match",
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Create new user
	user := &db.User{
		Email: email,
	}

	// Set password (this would use a proper password hashing mechanism)
	if err := user.SetPassword(password); err != nil {
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Failed to set password: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Set admin status
	user.SetIsAdmin(isAdmin)

	// Start transaction
	tx := h.DB.Begin()
	if err := tx.Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Database error: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Save user
	if err := tx.Create(user).Error; err != nil {
		tx.Rollback()
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Failed to create user: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Add roles if not admin and roles are selected
	if !isAdmin && len(roleIDs) > 0 {
		for _, roleIDStr := range roleIDs {
			roleID, err := strconv.ParseUint(roleIDStr, 10, 32)
			if err != nil {
				continue // Skip invalid IDs
			}

			// Check if role exists
			var role db.Role
			if err := tx.First(&role, roleID).Error; err != nil {
				continue // Skip if role doesn't exist
			}

			// Add role to user
			if err := tx.Model(user).Association("Roles").Append(&role); err != nil {
				tx.Rollback()
				data := components.UserManagementData{
					ActiveTab:    "create",
					ErrorMessage: "Failed to assign roles: " + err.Error(),
				}
				if c.GetHeader("HX-Request") == "true" {
					_ = components.UserManagementContent(data).Render(ctx, c.Writer)
				} else {
					_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
				}
				return
			}
		}
	}

	// Create audit log entry
	creatorID := getUserID(c)
	auditLog := &db.AuditLog{
		Action:     "create",
		EntityType: "user",
		EntityID:   user.ID,
		UserID:     creatorID,
		Timestamp:  time.Now(),
		Details: map[string]interface{}{
			"email":    user.Email,
			"is_admin": user.GetIsAdmin(),
		},
	}

	if err := tx.Create(auditLog).Error; err != nil {
		tx.Rollback()
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Failed to create audit log: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "create",
			ErrorMessage: "Failed to commit transaction: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// For HTMX requests, render the user list with success message
	if c.GetHeader("HX-Request") == "true" {
		// Fetch users from the database
		var users []db.User
		if err := h.DB.Find(&users).Error; err != nil {
			data := components.UserManagementData{
				ActiveTab:    "list",
				ErrorMessage: "Failed to fetch users after creation: " + err.Error(),
			}
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
			return
		}

		// Preload roles for each user
		for i := range users {
			if err := h.DB.Model(&users[i]).Association("Roles").Find(&users[i].Roles); err != nil {
				// Log error but continue
				fmt.Printf("Error loading roles for user %d: %v\n", users[i].ID, err)
			}
		}

		data := components.UserManagementData{
			Users:          users,
			ActiveTab:      "list",
			SuccessMessage: "User created successfully",
		}
		_ = components.UserManagementContent(data).Render(ctx, c.Writer)
	} else {
		// Redirect to user list with success message for regular requests
		c.Redirect(http.StatusFound, "/admin/users?success=User+created+successfully")
	}
}

// HandleEditUser renders the user edit page
func (h *Handlers) HandleEditUser(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	userID := c.Param("id")

	// Fetch user
	var user db.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "list",
			ErrorMessage: "User not found",
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Fetch user's roles
	if err := h.DB.Model(&user).Association("Roles").Find(&user.Roles); err != nil {
		data := components.UserManagementData{
			ActiveTab:    "edit",
			EditUser:     &user,
			ErrorMessage: "Failed to fetch user roles: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Fetch all available roles
	var roles []db.Role
	if err := h.DB.Find(&roles).Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "edit",
			EditUser:     &user,
			ErrorMessage: "Failed to fetch roles: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	data := components.UserManagementData{
		ActiveTab: "edit",
		EditUser:  &user,
		Roles:     roles,
	}

	if c.GetHeader("HX-Request") == "true" {
		_ = components.UserManagementContent(data).Render(ctx, c.Writer)
	} else {
		_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
	}
}

// HandleUpdateUser processes user updates
func (h *Handlers) HandleUpdateUser(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)
	userID := c.Param("id")
	email := c.PostForm("email")
	password := c.PostForm("password")
	passwordConfirm := c.PostForm("password_confirm")
	isAdmin := c.PostForm("is_admin") == "on"
	accountLocked := c.PostForm("account_locked") == "on"
	roleIDs := c.PostFormArray("roles[]")

	// Fetch existing user
	var user db.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "list",
			ErrorMessage: "User not found",
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Check if trying to update the last admin
	if user.GetIsAdmin() && !isAdmin {
		// Count how many admins there are
		var adminCount int64
		if err := h.DB.Model(&db.User{}).Where("metadata->>'is_admin' = 'true'").Count(&adminCount).Error; err != nil {
			data := components.UserManagementData{
				ActiveTab:    "edit",
				EditUser:     &user,
				ErrorMessage: "Failed to check admin count: " + err.Error(),
			}
			if c.GetHeader("HX-Request") == "true" {
				_ = components.UserManagementContent(data).Render(ctx, c.Writer)
			} else {
				_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
			}
			return
		}

		// If this is the last admin, prevent removal of admin status
		if adminCount <= 1 {
			data := components.UserManagementData{
				ActiveTab:    "edit",
				EditUser:     &user,
				ErrorMessage: "Cannot remove admin status from the last administrator",
			}
			if c.GetHeader("HX-Request") == "true" {
				_ = components.UserManagementContent(data).Render(ctx, c.Writer)
			} else {
				_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
			}
			return
		}
	}

	// Start transaction
	tx := h.DB.Begin()
	if err := tx.Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "edit",
			EditUser:     &user,
			ErrorMessage: "Database error: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Update email
	if email != "" && email != user.Email {
		user.Email = email
	}

	// Update password if provided
	if password != "" {
		if password != passwordConfirm {
			tx.Rollback()
			data := components.UserManagementData{
				ActiveTab:    "edit",
				EditUser:     &user,
				ErrorMessage: "Passwords do not match",
			}
			if c.GetHeader("HX-Request") == "true" {
				_ = components.UserManagementContent(data).Render(ctx, c.Writer)
			} else {
				_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
			}
			return
		}

		if err := user.SetPassword(password); err != nil {
			tx.Rollback()
			data := components.UserManagementData{
				ActiveTab:    "edit",
				EditUser:     &user,
				ErrorMessage: "Failed to set password: " + err.Error(),
			}
			if c.GetHeader("HX-Request") == "true" {
				_ = components.UserManagementContent(data).Render(ctx, c.Writer)
			} else {
				_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
			}
			return
		}
	}

	// Update admin status and account lock status
	user.SetIsAdmin(isAdmin)
	user.SetAccountLocked(accountLocked)

	// Save user changes
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		data := components.UserManagementData{
			ActiveTab:    "edit",
			EditUser:     &user,
			ErrorMessage: "Failed to update user: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Update roles if not admin
	if !isAdmin {
		// First, remove all existing roles
		if err := tx.Model(&user).Association("Roles").Clear(); err != nil {
			tx.Rollback()
			data := components.UserManagementData{
				ActiveTab:    "edit",
				EditUser:     &user,
				ErrorMessage: "Failed to clear existing roles: " + err.Error(),
			}
			if c.GetHeader("HX-Request") == "true" {
				_ = components.UserManagementContent(data).Render(ctx, c.Writer)
			} else {
				_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
			}
			return
		}

		// Then add the selected roles
		if len(roleIDs) > 0 {
			for _, roleIDStr := range roleIDs {
				roleID, err := strconv.ParseUint(roleIDStr, 10, 32)
				if err != nil {
					continue // Skip invalid IDs
				}

				// Check if role exists
				var role db.Role
				if err := tx.First(&role, roleID).Error; err != nil {
					continue // Skip if role doesn't exist
				}

				// Add role to user
				if err := tx.Model(&user).Association("Roles").Append(&role); err != nil {
					tx.Rollback()
					data := components.UserManagementData{
						ActiveTab:    "edit",
						EditUser:     &user,
						ErrorMessage: "Failed to assign roles: " + err.Error(),
					}
					if c.GetHeader("HX-Request") == "true" {
						_ = components.UserManagementContent(data).Render(ctx, c.Writer)
					} else {
						_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
					}
					return
				}
			}
		}
	}

	// Create audit log entry
	updaterID := getUserID(c)
	auditLog := &db.AuditLog{
		Action:     "update",
		EntityType: "user",
		EntityID:   user.ID,
		UserID:     updaterID,
		Timestamp:  time.Now(),
		Details: map[string]interface{}{
			"email":            user.Email,
			"is_admin":         user.GetIsAdmin(),
			"account_locked":   user.GetAccountLocked(),
			"password_changed": password != "",
		},
	}

	if err := tx.Create(auditLog).Error; err != nil {
		tx.Rollback()
		data := components.UserManagementData{
			ActiveTab:    "edit",
			EditUser:     &user,
			ErrorMessage: "Failed to create audit log: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		data := components.UserManagementData{
			ActiveTab:    "edit",
			EditUser:     &user,
			ErrorMessage: "Failed to commit transaction: " + err.Error(),
		}
		if c.GetHeader("HX-Request") == "true" {
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
		} else {
			_ = components.UserManagement(ctx, data).Render(ctx, c.Writer)
		}
		return
	}

	// For HTMX requests, render the user list with success message
	if c.GetHeader("HX-Request") == "true" {
		// Fetch users from the database
		var users []db.User
		if err := h.DB.Find(&users).Error; err != nil {
			data := components.UserManagementData{
				ActiveTab:    "list",
				ErrorMessage: "Failed to fetch users after update: " + err.Error(),
			}
			_ = components.UserManagementContent(data).Render(ctx, c.Writer)
			return
		}

		// Preload roles for each user
		for i := range users {
			if err := h.DB.Model(&users[i]).Association("Roles").Find(&users[i].Roles); err != nil {
				// Log error but continue
				fmt.Printf("Error loading roles for user %d: %v\n", users[i].ID, err)
			}
		}

		data := components.UserManagementData{
			Users:          users,
			ActiveTab:      "list",
			SuccessMessage: "User updated successfully",
		}
		_ = components.UserManagementContent(data).Render(ctx, c.Writer)
	} else {
		// Redirect to user list with success message for regular requests
		c.Redirect(http.StatusFound, "/admin/users?success=User+updated+successfully")
	}
}

// HandleDeleteUser processes user deletion
func (h *Handlers) HandleDeleteUser(c *gin.Context) {
	userID := c.Param("id")

	// Fetch user
	var user db.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if this is an admin user
	if user.GetIsAdmin() {
		// Check if other administrators exist
		var otherAdminCount int64
		// Use the actual 'is_admin' column, comparing against true
		if err := h.DB.Model(&db.User{}).
			Where("is_admin = ? AND id != ?", true, user.ID).
			Count(&otherAdminCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for other administrators"})
			return
		}

		// If no other administrators exist, prevent deletion
		if otherAdminCount == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last administrator"})
			return
		}
	}

	// Check if trying to delete yourself
	currentUserID := getUserID(c)
	if currentUserID == user.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
		return
	}

	// Start transaction
	tx := h.DB.Begin()
	if err := tx.Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Create audit log entry before deletion
	auditLog := &db.AuditLog{
		Action:     "delete",
		EntityType: "user",
		EntityID:   user.ID,
		UserID:     currentUserID,
		Timestamp:  time.Now(),
		Details: map[string]interface{}{
			"email":          user.Email,
			"is_admin":       user.GetIsAdmin(),
			"account_locked": user.GetAccountLocked(),
		},
	}

	if err := tx.Create(auditLog).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
		return
	}

	// Remove roles association
	if err := tx.Model(&user).Association("Roles").Clear(); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear user roles"})
		return
	}

	// Delete user
	if err := tx.Delete(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Fetch the updated user list to return
	ctx := components.CreateTemplateContext(c)
	var users []db.User
	if err := h.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated user list"})
		return
	}

	// Preload roles for each user
	for i := range users {
		if err := h.DB.Model(&users[i]).Association("Roles").Find(&users[i].Roles); err != nil {
			// Log error but continue
			fmt.Printf("Error loading roles for user %d: %v\n", users[i].ID, err)
		}
	}

	// Render the updated user list
	data := components.UserManagementData{
		Users:          users,
		ActiveTab:      "list",
		SuccessMessage: "User deleted successfully",
	}

	// Always use the partial for HTMX delete requests
	_ = components.UserManagementContent(data).Render(ctx, c.Writer)
}

// HandleLogViewer renders the log viewer page
func (h *Handlers) HandleLogViewer(c *gin.Context) {
	ctx := components.CreateTemplateContext(c)

	// Create logs data for initial page load
	logFilePath := filepath.Join(h.LogsDir, "scheduler.log")

	// Log the full path for debugging
	log.Printf("Log viewer initialized with log file path: %s", logFilePath)

	data := components.LogViewerData{
		Logs:          []components.LogEntry{},
		CurrentFilter: "",
		LogFilePath:   logFilePath,
	}

	// Render the log viewer component
	components.AdminLogs(ctx, data).Render(ctx, c.Writer)
}

// HandleLogStream handles WebSocket connections for real-time log streaming
func (h *Handlers) HandleLogStream(c *gin.Context) {
	// Configure upgrader
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	fmt.Fprintf(os.Stderr, "[DEBUG-WS] New WebSocket connection request from %s\n", c.ClientIP())

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Failed to upgrade WebSocket for %s: %v\n", c.ClientIP(), err)
		return
	}
	fmt.Fprintf(os.Stderr, "[DEBUG-WS] WebSocket connection upgraded for %s\n", c.ClientIP())

	// Set ping handler to respond with pong
	ws.SetPingHandler(func(data string) error {
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Received ping from %s, responding with pong\n", ws.RemoteAddr())
		return ws.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(5*time.Second))
	})

	// Ensure connection is closed eventually
	defer func() {
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Closing WebSocket connection for %s\n", ws.RemoteAddr())
		ws.Close()
	}()

	// Register the new client and create its mutex
	WebSocketClientsMutex.Lock()
	WebSocketClients[ws] = true
	WebSocketClientWriteMutexes[ws] = &sync.Mutex{}
	numClients := len(WebSocketClients)
	WebSocketClientsMutex.Unlock()

	fmt.Fprintf(os.Stderr, "[DEBUG-WS] Registered client %s. Total clients: %d\n", ws.RemoteAddr(), numClients)

	// De-register the client when the handler exits
	defer func() {
		WebSocketClientsMutex.Lock()
		delete(WebSocketClients, ws)
		delete(WebSocketClientWriteMutexes, ws)
		remainingClients := len(WebSocketClients)
		WebSocketClientsMutex.Unlock()
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] De-registered client %s. Remaining clients: %d\n", ws.RemoteAddr(), remainingClients)
	}()

	// Send recent logs immediately after connection
	h.sendRecentLogs(ws)

	// Start a goroutine to send pings periodically to keep the connection alive
	stopPinger := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
					fmt.Fprintf(os.Stderr, "[DEBUG-WS] Failed to send ping to client %s: %v\n", ws.RemoteAddr(), err)
					return
				}
				fmt.Fprintf(os.Stderr, "[DEBUG-WS] Sent ping to client %s\n", ws.RemoteAddr())
			case <-stopPinger:
				return
			}
		}
	}()

	// Keep the connection alive by reading messages (and discarding them)
	// This also detects when the client closes the connection.
	for {
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Fprintf(os.Stderr, "[DEBUG-WS] WebSocket closed unexpectedly for %s: %v\n", ws.RemoteAddr(), err)
			} else {
				fmt.Fprintf(os.Stderr, "[DEBUG-WS] WebSocket closed normally for %s.\n", ws.RemoteAddr())
			}
			break // Exit loop on Read error
		}

		// Handle client messages (like ping)
		if messageType == websocket.TextMessage && len(message) > 0 {
			fmt.Fprintf(os.Stderr, "[DEBUG-WS] Received message from client %s: %s\n", ws.RemoteAddr(), message)

			// Try to parse as JSON and check for ping
			var msgData map[string]interface{}
			if err := json.Unmarshal(message, &msgData); err == nil {
				if msgType, ok := msgData["type"].(string); ok && msgType == "ping" {
					fmt.Fprintf(os.Stderr, "[DEBUG-WS] Received ping from client %s, responding with pong\n", ws.RemoteAddr())

					// Send a pong response
					pongResp := map[string]interface{}{
						"type": "pong",
						"time": time.Now().Unix(),
					}

					WebSocketClientsMutex.Lock()
					mutex, exists := WebSocketClientWriteMutexes[ws]
					WebSocketClientsMutex.Unlock()

					if exists {
						mutex.Lock()
						err := ws.WriteJSON(pongResp)
						mutex.Unlock()

						if err != nil {
							fmt.Fprintf(os.Stderr, "[DEBUG-WS] Error sending pong to client %s: %v\n", ws.RemoteAddr(), err)
						}
					}
				}
			}
		}
	}

	// Stop the ping goroutine
	close(stopPinger)
}

// sendRecentLogs sends recent log entries to a new WebSocket client
func (h *Handlers) sendRecentLogs(ws *websocket.Conn) {
	// Get the mutex for this client *first*
	WebSocketClientsMutex.Lock()
	mutex, exists := WebSocketClientWriteMutexes[ws]
	if !exists {
		// This shouldn't happen if HandleLogStream is correct, but handle defensively
		WebSocketClientsMutex.Unlock()
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Mutex not found for client %v during sendRecentLogs. Aborting recent logs send.\n", ws.RemoteAddr())
		return
	}
	WebSocketClientsMutex.Unlock()

	// Construct the path to the log file
	logFilePath := filepath.Join(h.LogsDir, "scheduler.log")

	// Check if the log file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Log file not found at %s for sendRecentLogs. Sending example logs.\n", logFilePath)
		h.sendExampleLogs(ws) // Send examples if main log file isn't there
		return
	}

	// Open the log file
	file, err := os.Open(logFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Error opening log file %s: %v. Sending example logs.\n", logFilePath, err)
		h.sendExampleLogs(ws)
		return
	}
	defer file.Close()

	// Read the last 20 lines
	lines, err := readLastLines(file, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Error reading log file %s: %v. Sending example logs.\n", logFilePath, err)
		h.sendExampleLogs(ws)
		return
	}

	fmt.Fprintf(os.Stderr, "[DEBUG-WS] Read %d lines from %s for client %v.\n", len(lines), logFilePath, ws.RemoteAddr())

	// Parse and send each line as a log entry, protected by the client's mutex
	for i, line := range lines {
		level, source, message := parseLogLine(line)
		timestamp := extractTimestamp(line)

		logEntry := components.LogEntry{
			Timestamp: timestamp,
			Level:     level,
			Message:   message,
			Source:    source,
		}

		// Use the specific client's mutex
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Sending recent log %d/%d to client %v\n", i+1, len(lines), ws.RemoteAddr())
		mutex.Lock()
		err := ws.WriteJSON(logEntry)
		mutex.Unlock()

		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG-WS] Error sending recent log %d to client %v: %v. Stopping recent logs send.\n", i+1, ws.RemoteAddr(), err)
			// Don't try to remove the client here, let the main read loop handle it
			break // Stop sending recent logs on first error
		}
	}
	fmt.Fprintf(os.Stderr, "[DEBUG-WS] Finished sending %d recent logs to client %v.\n", len(lines), ws.RemoteAddr())
}

// readLastLines reads the last n lines from a file
func readLastLines(file *os.File, n int) ([]string, error) {
	// Implement a simpler version that reads the whole file and keeps the last n lines
	scanner := bufio.NewScanner(file)
	var lines []string

	// Read all lines
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return the last n lines (or all if less than n)
	if len(lines) <= n {
		return lines, nil
	}

	return lines[len(lines)-n:], nil
}

// parseLogLine extracts level, source, and message from a log line
// This is specific to the format found in the log file being read,
// NOT the format generated by the standard Go logger directly.
func parseLogLine(line string) (level, source, message string) {
	// Default values
	level = "info"
	source = "system"
	originalLine := line // Keep original for prefix check

	// Check for level prefixes first
	foundPrefix := false
	if strings.HasPrefix(originalLine, "DEBUG:") { // Check original line for prefix
		level = "debug"
		line = strings.TrimSpace(strings.TrimPrefix(originalLine, "DEBUG:"))
		foundPrefix = true
	} else if strings.HasPrefix(originalLine, "INFO:") {
		level = "info"
		line = strings.TrimSpace(strings.TrimPrefix(originalLine, "INFO:"))
		foundPrefix = true
	} else if strings.HasPrefix(originalLine, "ERROR:") {
		level = "error"
		line = strings.TrimSpace(strings.TrimPrefix(originalLine, "ERROR:"))
		foundPrefix = true
	} else if strings.HasPrefix(originalLine, "WARN:") {
		level = "warn"
		line = strings.TrimSpace(strings.TrimPrefix(originalLine, "WARN:"))
		foundPrefix = true
	} else if strings.HasPrefix(originalLine, "WARNING:") {
		level = "warn"
		line = strings.TrimSpace(strings.TrimPrefix(originalLine, "WARNING:"))
		foundPrefix = true
	} else if strings.HasPrefix(originalLine, "FATAL:") {
		level = "fatal"
		line = strings.TrimSpace(strings.TrimPrefix(originalLine, "FATAL:"))
		foundPrefix = true
	}

	// Now parse the rest (timestamp + message) using the potentially modified 'line'
	parts := strings.SplitN(line, " ", 3)
	if len(parts) >= 3 {
		message = parts[2] // The rest is the message

		// Try to extract source *only if no level prefix was found initially*
		// Assumes standard log format prefixes message with file:line
		if !foundPrefix {
			if fileStart := strings.Index(message, " "); fileStart > 0 {
				filePath := message[:fileStart]
				if strings.Contains(filePath, ":") {
					filePathParts := strings.Split(filePath, "/")
					if len(filePathParts) > 0 {
						fileNameWithLine := filePathParts[len(filePathParts)-1]
						fileName := strings.Split(fileNameWithLine, ":")[0]
						source = strings.TrimSuffix(fileName, ".go")
					}
				}
				// Update message to remove the file info
				message = message[fileStart+1:]
			}
		}

		// If no prefix was found, attempt level detection from message content (e.g., [info])
		if !foundPrefix {
			parsedLevel, cleanMessage := parseLevelFromMessageContent(message)
			level = parsedLevel
			message = cleanMessage
		}

	} else {
		// Fallback if split doesn't work as expected, use the (potentially prefix-stripped) line
		message = line
	}

	return level, source, message
}

// parseLevelFromMessageContent checks for bracketed level indicators
func parseLevelFromMessageContent(msg string) (string, string) {
	level := "info" // Default
	cleanMsg := msg

	if strings.HasPrefix(msg, "[debug]") {
		level = "debug"
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[debug]"))
	} else if strings.HasPrefix(msg, "[info]") {
		level = "info"
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[info]"))
	} else if strings.HasPrefix(msg, "[warn]") {
		level = "warn"
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[warn]"))
	} else if strings.HasPrefix(msg, "[warning]") {
		level = "warn"
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[warning]"))
	} else if strings.HasPrefix(msg, "[error]") {
		level = "error"
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[error]"))
	} else if strings.HasPrefix(msg, "[fatal]") {
		level = "fatal"
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[fatal]"))
	}
	// Optional: Add inference based on keywords like the logger does
	// else if strings.Contains(strings.ToLower(msg), "error") { level = "error" } ...
	return level, cleanMsg
}

// extractTimestamp extracts the timestamp from a log line, handling potential prefixes
func extractTimestamp(line string) time.Time {
	now := time.Now() // Default
	originalLine := line

	// Remove known level prefixes for timestamp parsing
	prefixes := []string{"DEBUG:", "INFO:", "ERROR:", "WARN:", "WARNING:", "FATAL:"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			line = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			break
		}
	}

	// Try to extract timestamp parts (date and time)
	parts := strings.SplitN(line, " ", 3)
	if len(parts) >= 2 {
		dateStr := parts[0]
		timeStr := parts[1]
		timestampStr := dateStr + " " + timeStr

		// List of timestamp formats to try
		formats := []string{
			"2006/01/02 15:04:05",        // Standard Go log with slashes
			"2006/01/02 15:04:05.999",    // With milliseconds
			"2006/01/02 15:04:05.999999", // With microseconds
			"2006-01-02 15:04:05",        // Standard Go log with dashes
			"2006-01-02 15:04:05.999",    // With milliseconds
			"2006-01-02 15:04:05.999999", // With microseconds
		}

		for _, format := range formats {
			timestamp, err := time.Parse(format, timestampStr)
			if err == nil {
				return timestamp // Successfully parsed
			}
		}
		// If all formats failed, log the original attempt
		fmt.Fprintf(os.Stderr, "[DEBUG-TIMESTAMP] Failed to parse timestamp from '%s' (derived from line: %s)\n", timestampStr, originalLine)
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG-TIMESTAMP] Could not split timestamp parts from line: %s\n", originalLine)
	}

	return now // Return current time if parsing failed
}

// sendExampleLogs sends example log entries for demonstration
func (h *Handlers) sendExampleLogs(ws *websocket.Conn) {
	// Get the mutex for this client *first*
	WebSocketClientsMutex.Lock()
	mutex, exists := WebSocketClientWriteMutexes[ws]
	if !exists {
		WebSocketClientsMutex.Unlock()
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Mutex not found for client %v during sendExampleLogs. Aborting example logs send.\n", ws.RemoteAddr())
		return
	}
	WebSocketClientsMutex.Unlock()

	fmt.Fprintf(os.Stderr, "[DEBUG-WS] Sending example logs to client %v\n", ws.RemoteAddr())

	// Example log entries for demonstration
	exampleLogs := []components.LogEntry{
		{
			Timestamp: time.Now().UTC().Add(-time.Minute * 5),
			Level:     "info",
			Message:   "Application started successfully",
			Source:    "main",
		},
		{
			Timestamp: time.Now().UTC().Add(-time.Minute * 3),
			Level:     "debug",
			Message:   "Connected to database",
			Source:    "database",
		},
		{
			Timestamp: time.Now().UTC().Add(-time.Minute * 2),
			Level:     "warn",
			Message:   "High memory usage detected: 85%",
			Source:    "monitor",
		},
	}

	for i, logEntry := range exampleLogs {
		// Use the specific client's mutex
		fmt.Fprintf(os.Stderr, "[DEBUG-WS] Sending example log %d/%d to client %v\n", i+1, len(exampleLogs), ws.RemoteAddr())
		mutex.Lock()
		err := ws.WriteJSON(logEntry)
		mutex.Unlock()

		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG-WS] Error sending example log %d to client %v: %v. Stopping example logs send.\n", i+1, ws.RemoteAddr(), err)
			break // Stop sending example logs on first error
		}
	}
	fmt.Fprintf(os.Stderr, "[DEBUG-WS] Finished sending example logs to client %v.\n", ws.RemoteAddr())
}

// HandleStartLogGenerator handles requests to start the log generator for testing
func (h *Handlers) HandleStartLogGenerator(c *gin.Context) {
	// Directly send a log to verify the WebSocket is working
	h.BroadcastLog("info", "Starting log generator...", "test")

	// Start a goroutine to generate some test logs
	go func() {
		logLevels := []string{"debug", "info", "warn", "error"}
		sources := []string{"test", "generator", "system", "scheduler"}

		// First, send a direct log message to all clients
		for i := 0; i < 20; i++ {
			level := logLevels[i%len(logLevels)]
			source := sources[i%len(sources)]
			message := fmt.Sprintf("Test log entry #%d generated at %s", i+1, time.Now().Format(time.RFC3339))

			// First directly broadcast without going through normal logging
			h.BroadcastLog(level, message, source)

			// Wait a short time between logs
			time.Sleep(500 * time.Millisecond)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Log generator started"})
}
