package handlers

import (
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components/providers/common"
	"github.com/starfleetcptn/gomft/internal/db"
)

// RcloneHandler contains handlers for rclone-related routes
type RcloneHandler struct {
	DB *db.DB
}

// NewRcloneHandler creates a new RcloneHandler
func NewRcloneHandler(db *db.DB) *RcloneHandler {
	return &RcloneHandler{
		DB: db,
	}
}

// RcloneCommandOptions renders the rclone command options for the config form
func (h *RcloneHandler) RcloneCommandOptions(c *gin.Context) {
	commands, err := h.DB.GetRcloneCommands()
	if err != nil {
		log.Printf("Error getting rclone commands: %v", err)
		c.String(http.StatusInternalServerError, "Error getting rclone commands")
		return
	}

	// Group commands by category
	categories, err := h.DB.GetRcloneCategories()
	if err != nil {
		log.Printf("Error getting rclone categories: %v", err)
		c.String(http.StatusInternalServerError, "Error getting rclone categories")
		return
	}

	// Create a map of category -> commands
	categoryMap := make(map[string][]db.RcloneCommand)
	for _, cmd := range commands {
		categoryMap[cmd.Category] = append(categoryMap[cmd.Category], cmd)
	}

	_ = common.RcloneCommandOptionsContent(categoryMap, categories).Render(c.Request.Context(), c.Writer)
}

// RcloneCommandFlags renders the rclone command flags for the selected command
func (h *RcloneHandler) RcloneCommandFlags(c *gin.Context) {
	commandIDStr := c.DefaultQuery("command_id", "")
	if commandIDStr == "" {
		c.String(http.StatusBadRequest, "Command ID is required")
		return
	}

	commandID, err := strconv.ParseUint(commandIDStr, 10, 64)
	if err != nil {
		log.Printf("Error parsing command ID: %v", err)
		c.String(http.StatusBadRequest, "Invalid command ID")
		return
	}

	command, err := h.DB.GetRcloneCommandWithFlags(uint(commandID))
	if err != nil {
		log.Printf("Error getting rclone command flags: %v", err)
		c.String(http.StatusInternalServerError, "Error getting rclone command flags")
		return
	}

	if command == nil {
		c.String(http.StatusNotFound, "Command not found")
		return
	}

	// Sort the flags alphabetically by name
	sort.Slice(command.Flags, func(i, j int) bool {
		return command.Flags[i].Name < command.Flags[j].Name
	})

	_ = common.RcloneCommandFlagsContent(command).Render(c.Request.Context(), c.Writer)
}

// RcloneCommandUsage renders the usage information for a command
func (h *RcloneHandler) RcloneCommandUsage(c *gin.Context) {
	commandIDStr := c.Param("id")
	if commandIDStr == "" {
		c.String(http.StatusBadRequest, "Command ID is required")
		return
	}

	commandID, err := strconv.ParseUint(commandIDStr, 10, 64)
	if err != nil {
		log.Printf("Error parsing command ID: %v", err)
		c.String(http.StatusBadRequest, "Invalid command ID")
		return
	}

	usage, err := h.DB.GetRcloneCommandUsage(uint(commandID))
	if err != nil {
		log.Printf("Error getting rclone command usage: %v", err)
		c.String(http.StatusInternalServerError, "Error getting rclone command usage")
		return
	}

	c.HTML(http.StatusOK, "command_usage.html", gin.H{
		"Usage": template.HTML(usage),
	})
}
