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
