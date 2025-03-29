package db

import (
	"fmt"
	"strconv"
	"strings"
)

// --- Rclone Store Methods ---

// GetRcloneCommands returns all rclone commands
func (db *DB) GetRcloneCommands() ([]RcloneCommand, error) {
	var commands []RcloneCommand
	err := db.Find(&commands).Error
	return commands, err
}

// GetRcloneCommand returns a specific rclone command by ID
func (db *DB) GetRcloneCommand(id uint) (*RcloneCommand, error) {
	var command RcloneCommand
	err := db.First(&command, id).Error
	if err != nil {
		return nil, err
	}
	return &command, nil
}

// GetRcloneCommandByName returns a specific rclone command by name
func (db *DB) GetRcloneCommandByName(name string) (*RcloneCommand, error) {
	var command RcloneCommand
	err := db.Where("name = ?", name).First(&command).Error
	if err != nil {
		return nil, err
	}
	return &command, nil
}

// GetRcloneCommandsInCategory returns all commands in a specific category
func (db *DB) GetRcloneCommandsInCategory(category string) ([]RcloneCommand, error) {
	var commands []RcloneCommand
	err := db.Where("category = ?", category).Find(&commands).Error
	return commands, err
}

// GetRcloneCommandFlag returns a specific flag by ID
func (db *DB) GetRcloneCommandFlag(id uint) (*RcloneCommandFlag, error) {
	var flag RcloneCommandFlag
	err := db.First(&flag, id).Error
	if err != nil {
		return nil, err
	}
	return &flag, nil
}

// GetRcloneCommandFlagByName returns a specific flag by name for a command
func (db *DB) GetRcloneCommandFlagByName(commandID uint, name string) (*RcloneCommandFlag, error) {
	var flag RcloneCommandFlag
	err := db.Where("command_id = ? AND name = ?", commandID, name).First(&flag).Error
	if err != nil {
		return nil, err
	}
	return &flag, nil
}

// GetRcloneCommandFlags returns all flags for a specific command
func (db *DB) GetRcloneCommandFlags(commandID uint) ([]RcloneCommandFlag, error) {
	var flags []RcloneCommandFlag
	err := db.Where("command_id = ?", commandID).Find(&flags).Error
	return flags, err
}

// GetRcloneCommandWithFlags returns a command with all its flags
func (db *DB) GetRcloneCommandWithFlags(commandID uint) (*RcloneCommand, error) {
	var command RcloneCommand
	err := db.Preload("Flags").First(&command, commandID).Error
	if err != nil {
		return nil, err
	}
	return &command, nil
}

// BuildRcloneCommand builds an rclone command string with the specified command and flags
func (db *DB) BuildRcloneCommand(commandName string, flags map[string]string) (string, error) {
	// Get the command details
	command, err := db.GetRcloneCommandByName(commandName)
	if err != nil {
		return "", fmt.Errorf("command not found: %s", commandName)
	}

	// Start building the command string
	cmdStr := "rclone " + command.Name

	// Get all flags for this command
	allFlags, err := db.GetRcloneCommandFlags(command.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get flags for command: %v", err)
	}

	// Create a map of flag details for easy lookup
	flagDetails := make(map[string]RcloneCommandFlag)
	for _, f := range allFlags {
		flagDetails[f.Name] = f
	}

	// Add the flags to the command
	for name, value := range flags {
		// Check if the flag exists for this command
		flag, exists := flagDetails[name]
		if !exists {
			// Allow passing flags not explicitly defined in the DB (e.g., global flags)
			// Consider adding validation or logging for unknown flags if stricter control is needed
			cmdStr += " " + name
			if value != "true" { // Assume boolean flags are passed as "true" if value is needed
				cmdStr += " " + value
			}
			continue
			// return "", fmt.Errorf("invalid flag for command %s: %s", commandName, name)
		}

		// Handle different flag types
		switch flag.DataType {
		case "bool":
			if value == "true" {
				cmdStr += " " + flag.Name // Use flag.Name which includes '--'
			}
		default:
			cmdStr += " " + flag.Name + " " + value // Use flag.Name which includes '--'
		}
	}

	return cmdStr, nil
}

// ValidateRcloneFlags validates if the provided flags are valid for the command
func (db *DB) ValidateRcloneFlags(commandName string, flags map[string]string) (bool, map[string]string) {
	// Initialize errors map
	errorsMap := make(map[string]string)

	// Get the command details
	command, err := db.GetRcloneCommandByName(commandName)
	if err != nil {
		errorsMap["command"] = "Command not found: " + commandName
		return false, errorsMap
	}

	// Get all flags for this command
	allFlags, err := db.GetRcloneCommandFlags(command.ID)
	if err != nil {
		errorsMap["command"] = "Failed to get flags for command"
		return false, errorsMap
	}

	// Create a map of flag details for easy lookup
	flagDetails := make(map[string]RcloneCommandFlag)
	for _, f := range allFlags {
		flagDetails[f.Name] = f // Assuming Name includes '--' prefix
	}

	// Check each provided flag
	for name, value := range flags {
		// Check if the flag exists for this command
		flag, exists := flagDetails[name]
		if !exists {
			// Allow unknown flags for now, but could add an error here if needed
			// errorsMap[name] = "Invalid flag for command " + commandName
			continue
		}

		// Validate the flag value based on data type
		switch flag.DataType {
		case "int":
			if _, err := strconv.Atoi(value); err != nil {
				errorsMap[name] = "Value must be an integer"
			}
		case "float":
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				errorsMap[name] = "Value must be a number"
			}
		case "bool":
			// For boolean flags passed in the map, the value should ideally be "true" or omitted
			// If present and not "true", it's likely an error or misuse.
			// Rclone CLI typically handles bool flags by presence/absence.
			// This validation might need refinement based on how flags are constructed before calling this.
			if value != "true" {
				// errorsMap[name] = "Boolean flag should have value 'true' or be omitted"
			}
		case "string":
			// Basic check: ensure value is not empty if flag requires a value
			// More complex validation (regex, length) could be added here
			if value == "" && flag.IsRequired { // Check if required string flags have values
				errorsMap[name] = "Value cannot be empty for required string flag"
			}
		}
	}

	// Check for missing required flags
	for _, flag := range allFlags {
		if flag.IsRequired {
			if _, provided := flags[flag.Name]; !provided {
				// Check if the short name was provided instead
				shortNameProvided := false
				if flag.ShortName != "" {
					_, shortNameProvided = flags[flag.ShortName]
				}
				if !shortNameProvided {
					errorsMap[flag.Name] = "This flag is required"
				}
			} else if flag.DataType != "bool" && flags[flag.Name] == "" {
				// Required non-bool flags must have a value
				errorsMap[flag.Name] = "Value cannot be empty for required flag"
			}
		}
	}

	return len(errorsMap) == 0, errorsMap
}

// GetRcloneCategories returns all unique categories of rclone commands
func (db *DB) GetRcloneCategories() ([]string, error) {
	var categories []string
	err := db.Model(&RcloneCommand{}).Distinct("category").Pluck("category", &categories).Error
	return categories, err
}

// GetRcloneCommandsByAdvanced returns commands filtered by their advanced status
func (db *DB) GetRcloneCommandsByAdvanced(isAdvanced bool) ([]RcloneCommand, error) {
	var commands []RcloneCommand
	err := db.Where("is_advanced = ?", isAdvanced).Find(&commands).Error
	return commands, err
}

// SearchRcloneCommands searches for commands by name or description
func (db *DB) SearchRcloneCommands(query string) ([]RcloneCommand, error) {
	var commands []RcloneCommand
	searchQuery := "%" + query + "%"
	err := db.Where("name LIKE ? OR description LIKE ?", searchQuery, searchQuery).Find(&commands).Error
	return commands, err
}

// GetRcloneCommandUsage returns a basic usage example for a command with its required flags
func (db *DB) GetRcloneCommandUsage(commandID uint) (string, error) {
	command, err := db.GetRcloneCommandWithFlags(commandID)
	if err != nil {
		return "", err
	}

	usage := fmt.Sprintf("rclone %s [flags] <source> <dest>", command.Name)

	// Add basic usage examples for required flags
	requiredFlags := []string{}
	for _, flag := range command.Flags {
		if flag.IsRequired {
			// Assuming GetUsageExample is defined on RcloneCommandFlag in rclone.go
			requiredFlags = append(requiredFlags, flag.GetUsageExample())
		}
	}

	if len(requiredFlags) > 0 {
		usage += "\n\nRequired flags:\n  " + strings.Join(requiredFlags, "\n  ")
	}

	return usage, nil
}

// RenderRcloneCommandHelp generates a help text for a command with its flags
func (db *DB) RenderRcloneCommandHelp(commandID uint) (string, error) {
	command, err := db.GetRcloneCommandWithFlags(commandID)
	if err != nil {
		return "", err
	}

	// Build the help text
	help := fmt.Sprintf("COMMAND: %s\n", command.Name)
	help += fmt.Sprintf("DESCRIPTION: %s\n\n", command.Description)
	help += "FLAGS:\n"

	// Group flags by required status
	var requiredFlags, optionalFlags []RcloneCommandFlag
	for _, flag := range command.Flags {
		if flag.IsRequired {
			requiredFlags = append(requiredFlags, flag)
		} else {
			optionalFlags = append(optionalFlags, flag)
		}
	}

	// Add required flags
	if len(requiredFlags) > 0 {
		help += "  Required:\n"
		for _, flag := range requiredFlags {
			shortName := ""
			if flag.ShortName != "" {
				shortName = fmt.Sprintf(" (-%s)", flag.ShortName)
			}
			help += fmt.Sprintf("    %s%s - %s\n", flag.Name, shortName, flag.Description)
			if flag.DataType != "bool" && flag.DefaultValue != "" {
				help += fmt.Sprintf("      Default: %s\n", flag.DefaultValue)
			}
		}
	}

	// Add optional flags
	if len(optionalFlags) > 0 {
		help += "\n  Optional:\n"
		for _, flag := range optionalFlags {
			shortName := ""
			if flag.ShortName != "" {
				shortName = fmt.Sprintf(" (-%s)", flag.ShortName)
			}
			help += fmt.Sprintf("    %s%s - %s\n", flag.Name, shortName, flag.Description)
			if flag.DataType != "bool" && flag.DefaultValue != "" {
				help += fmt.Sprintf("      Default: %s\n", flag.DefaultValue)
			}
		}
	}

	return help, nil
}

// GetRcloneCommandFlagsMap returns all flags for a specific command as a map keyed by flag ID
func (db *DB) GetRcloneCommandFlagsMap(commandID uint) (map[uint]RcloneCommandFlag, error) {
	flags, err := db.GetRcloneCommandFlags(commandID)
	if err != nil {
		return nil, err
	}

	flagsMap := make(map[uint]RcloneCommandFlag)
	for _, flag := range flags {
		flagsMap[flag.ID] = flag
	}

	return flagsMap, nil
}
