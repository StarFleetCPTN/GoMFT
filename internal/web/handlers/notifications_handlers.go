package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
)

// HandleNotifications displays all notifications for the current user
func (h *Handlers) HandleNotifications(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get notifications for the user
	notifications, err := h.DB.GetUserNotifications(userID, 50) // Get more for the full page
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
