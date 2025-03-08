package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleBackupDB handles the POST /admin/backup route
func (h *Handlers) HandleBackupDB(c *gin.Context) {
	// TODO: Implement database backup
	c.JSON(http.StatusOK, gin.H{"message": "Database backup initiated"})
} 