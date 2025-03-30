package handlers

import (
	"encoding/json"
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
	// Get the current command ID if provided (for pre-selection)
	currentCommandIDStr := c.DefaultQuery("commandId", "1") // Default to 1 (copy)
	currentCommandID, err := strconv.ParseUint(currentCommandIDStr, 10, 64)
	if err != nil {
		log.Printf("Error parsing current command ID: %v, using default 1", err)
		currentCommandID = 1
	}

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

	// Get the initial flag JSON strings passed from the placeholder's hx-vals
	commandFlagsJSON := c.DefaultQuery("commandFlags", "[]")
	commandFlagValuesJSON := c.DefaultQuery("commandFlagValues", "{}")

	// Pass the current command ID and flag JSON strings to the template
	_ = common.RcloneCommandOptionsContent(categoryMap, categories, uint(currentCommandID), commandFlagsJSON, commandFlagValuesJSON).Render(c.Request.Context(), c.Writer)
}

// RcloneCommandFlags renders the rclone command flags for the selected command
func (h *RcloneHandler) RcloneCommandFlags(c *gin.Context) {
	commandIDStr := c.DefaultQuery("command_id", "")
	commandFlagsJSON := c.DefaultQuery("commandFlags", "[]")           // Get selected flag IDs JSON
	commandFlagValuesJSON := c.DefaultQuery("commandFlagValues", "{}") // Get selected flag values JSON

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

	// Parse the selected flags and values
	var selectedFlagIDs []uint
	if err := json.Unmarshal([]byte(commandFlagsJSON), &selectedFlagIDs); err != nil {
		log.Printf("Error unmarshaling commandFlags JSON: %v, JSON: %s", err, commandFlagsJSON)
		// Don't fail, just proceed with no flags selected
		selectedFlagIDs = []uint{}
	}
	selectedFlagsMap := make(map[uint]bool)
	for _, id := range selectedFlagIDs {
		selectedFlagsMap[id] = true
	}

	var selectedFlagValues map[uint]string
	if err := json.Unmarshal([]byte(commandFlagValuesJSON), &selectedFlagValues); err != nil {
		log.Printf("Error unmarshaling commandFlagValues JSON: %v, JSON: %s", err, commandFlagValuesJSON)
		// Don't fail, just proceed with no values
		selectedFlagValues = make(map[uint]string)
	}

	if command == nil {
		c.String(http.StatusNotFound, "Command not found")
		return
	}

	// Sort the flags alphabetically by name
	sort.Slice(command.Flags, func(i, j int) bool {
		return command.Flags[i].Name < command.Flags[j].Name
	})

	// Pass the parsed selected flags and values to the template
	_ = common.RcloneCommandFlagsContent(command, selectedFlagsMap, selectedFlagValues).Render(c.Request.Context(), c.Writer)
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
