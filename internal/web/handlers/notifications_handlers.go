package handlers

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
)

// HandleNotifications displays all notifications for the current user
func (h *Handlers) HandleNotifications(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get pagination parameters from query
	pageStr := c.DefaultQuery("page", "1")
	perPageStr := c.DefaultQuery("perPage", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	perPage, err := strconv.Atoi(perPageStr)
	if err != nil {
		perPage = 10
	}

	// Ensure perPage is one of the allowed values
	validPerPage := []int{10, 25, 50, 100}
	isValid := false
	for _, v := range validPerPage {
		if v == perPage {
			isValid = true
			break
		}
	}
	if !isValid {
		perPage = 10
	}

	// Get total count for pagination
	totalCount, err := h.DB.GetUserNotificationCount(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to count notifications")
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalCount) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	// Ensure page is within range
	if page > totalPages {
		page = totalPages
	}

	// Calculate offset for database query
	offset := (page - 1) * perPage

	// Get paginated notifications for the user
	notifications, err := h.DB.GetPaginatedUserNotifications(userID, offset, perPage)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load notifications")
		return
	}

	// Get unread count
	unreadCount, err := h.DB.GetUnreadNotificationCount(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load notification count")
		return
	}

	data := components.NotificationsData{
		Notifications: notifications,
		UnreadCount:   unreadCount,
		CurrentPage:   page,
		TotalPages:    totalPages,
		TotalCount:    int(totalCount),
		PerPage:       perPage,
	}

	// Render the notifications page
	components.NotificationsPage(c.Request.Context(), data).Render(c, c.Writer)
}

// HandleLoadNotifications loads the notifications dropdown content
func (h *Handlers) HandleLoadNotifications(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get 10 most recent notifications
	notifications, err := h.DB.GetUserNotifications(userID, 10)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load notifications")
		return
	}

	// Get unread count
	unreadCount, err := h.DB.GetUnreadNotificationCount(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load notification count")
		return
	}

	data := components.NotificationsData{
		Notifications: notifications,
		UnreadCount:   unreadCount,
	}

	// Render just the dropdown content
	components.NotificationDropdown(data).Render(c, c.Writer)
}

// HandleNotificationCount returns the notification count badge
func (h *Handlers) HandleNotificationCount(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get unread count
	unreadCount, err := h.DB.GetUnreadNotificationCount(userID)

	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load notification count")
		return
	}

	// Render just the count badge
	components.NotificationCount(unreadCount).Render(c, c.Writer)
}

// HandleMarkNotificationAsRead marks a single notification as read
func (h *Handlers) HandleMarkNotificationAsRead(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get notification ID from path
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	// Mark as read
	if err := h.DB.MarkNotificationAsRead(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	// Return updated count
	unreadCount, err := h.DB.GetUnreadNotificationCount(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load notification count")
		return
	}

	// Return just the updated count badge
	components.NotificationCount(unreadCount).Render(c, c.Writer)
}

// HandleMarkAllNotificationsAsRead marks all notifications for a user as read
func (h *Handlers) HandleMarkAllNotificationsAsRead(c *gin.Context) {
	userID := c.GetUint("userID")

	// Mark all as read
	if err := h.DB.MarkAllNotificationsAsRead(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notifications as read"})
		return
	}

	// Return empty count (no more unread notifications)
	components.NotificationCount(0).Render(c, c.Writer)
}
