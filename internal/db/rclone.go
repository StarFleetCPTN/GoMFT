package db

import (
	"fmt"
	"strings"
	"time"
)

// RcloneCommand represents a command available in rclone
type RcloneCommand struct {
	ID          uint                `gorm:"primarykey"`
	Name        string              `gorm:"not null;uniqueIndex"`
	Description string              `gorm:"not null"`
	Category    string              `gorm:"not null;index"`
	IsAdvanced  bool                `gorm:"not null;default:false"`
	Flags       []RcloneCommandFlag `gorm:"foreignKey:CommandID;constraint:OnDelete:CASCADE"`
	CreatedAt   time.Time           `gorm:"not null"`
}

// RcloneCommandFlag represents a flag that can be used with an rclone command
type RcloneCommandFlag struct {
	ID           uint          `gorm:"primarykey"`
	CommandID    uint          `gorm:"not null;index"`
	Command      RcloneCommand `gorm:"foreignKey:CommandID"`
	Name         string        `gorm:"not null;index"`
	ShortName    string
	Description  string `gorm:"not null"`
	DataType     string `gorm:"not null"` // string, int, bool, etc.
	IsRequired   bool   `gorm:"not null;default:false"`
	DefaultValue string
	CreatedAt    time.Time `gorm:"not null"`
}

// --- Rclone Helper Methods ---

// GetUsageExample returns a human-readable usage example for a flag
func (flag *RcloneCommandFlag) GetUsageExample() string {
	switch flag.DataType {
	case "bool":
		return flag.Name
	case "int":
		return fmt.Sprintf("%s=<number>", flag.Name)
	case "float":
		return fmt.Sprintf("%s=<decimal>", flag.Name)
	case "string":
		return fmt.Sprintf("%s=<text>", flag.Name)
	default:
		return fmt.Sprintf("%s=<value>", flag.Name)
	}
}

// ParseRcloneFlags parses a string of rclone flags into a map
// Note: This is a general utility function, not tied to a specific struct instance.
// It might be better placed in a more general utility package if one exists,
// but keeping it here for now as per the original file structure.
func ParseRcloneFlags(flagsStr string) map[string]string {
	result := make(map[string]string)
	if flagsStr == "" {
		return result
	}

	// Split the flags string by spaces
	parts := strings.Fields(flagsStr)

	for i := 0; i < len(parts); i++ {
		part := parts[i]

		// Check if it's a flag (starts with --)
		if strings.HasPrefix(part, "--") {
			// Remove the -- prefix
			flagName := part // Keep the '--' prefix in the map key for consistency? Or remove? Plan used remove.
			// flagName := strings.TrimPrefix(part, "--") // Alternative: remove prefix

			// Check if the flag has a value
			if i+1 < len(parts) && !strings.HasPrefix(parts[i+1], "--") {
				// Next part is a value
				result[flagName] = parts[i+1]
				i++ // Skip the value in the next iteration
			} else {
				// Flag without value, treat as boolean true
				result[flagName] = "true"
			}
		}
	}

	return result
}
