package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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
