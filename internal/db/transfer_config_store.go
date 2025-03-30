package db

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// --- TransferConfig Store Methods ---

// CreateTransferConfig creates a new transfer config record
func (db *DB) CreateTransferConfig(config *TransferConfig) error {
	return db.Create(config).Error
}

// GetTransferConfigs retrieves all transfer configs for a user
func (db *DB) GetTransferConfigs(userID uint) ([]TransferConfig, error) {
	var configs []TransferConfig
	err := db.Where("created_by = ?", userID).Find(&configs).Error
	return configs, err
}

// GetTransferConfig retrieves a single transfer config by ID
func (db *DB) GetTransferConfig(id uint) (*TransferConfig, error) {
	var config TransferConfig
	err := db.First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateTransferConfig updates an existing transfer config record
func (db *DB) UpdateTransferConfig(config *TransferConfig) error {
	return db.Save(config).Error
}

// DeleteTransferConfig deletes a transfer config record after checking dependencies
func (db *DB) DeleteTransferConfig(id uint) error {
	// First check if any jobs are using this config
	var count int64
	// Need to check both ConfigID and ConfigIDs list
	// This check might need refinement depending on how ConfigIDs is used reliably
	if err := db.Model(&Job{}).Where("config_id = ? OR config_ids LIKE ?", id, "%"+strconv.FormatUint(uint64(id), 10)+"%").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check for dependent jobs: %v", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete config: %d jobs are using this configuration", count)
	}

	// Delete the config
	return db.Delete(&TransferConfig{}, id).Error
}

// GetConfigRclonePath returns the path to the rclone config file for a given transfer config
func (db *DB) GetConfigRclonePath(config *TransferConfig) string {
	// Get data directory from environment or use default
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Store configs in the data directory
	return filepath.Join(dataDir, "configs", fmt.Sprintf("config_%d.conf", config.ID))
}

// GenerateRcloneConfig generates the rclone config file content based on TransferConfig
// This function now primarily focuses on generating the content string or calling rclone config create
func (db *DB) GenerateRcloneConfig(config *TransferConfig) error {
	configPath := db.GetConfigRclonePath(config)

	// Get the directory part of the path
	configDir := filepath.Dir(configPath)

	// Ensure configs directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create configs directory: %v", err)
	}

	// Get the rclone path from the environment variable or use the default path
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	sourceName := fmt.Sprintf("source_%d", config.ID)
	// Generate rclone config using rclone CLI for source
	switch config.SourceType {
	case "sftp":
		args := []string{
			"config", "create", sourceName, "sftp",
			"host", config.SourceHost,
			"user", config.SourceUser,
			"port", fmt.Sprintf("%d", config.SourcePort),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		if config.SourcePassword != "" {
			args = append(args, "pass", config.SourcePassword)
		}
		if config.SourceKeyFile != "" {
			args = append(args, "key_file", config.SourceKeyFile)
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (sftp): %v\nOutput: %s", err, output)
		}
	case "s3":
		args := []string{
			"config", "create", sourceName, "s3",
			"provider", "AWS", // Assuming AWS provider, adjust if needed
			"env_auth", "false",
			"access_key_id", config.SourceAccessKey,
			"secret_access_key", config.SourceSecretKey,
			"region", config.SourceRegion,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		if config.SourceEndpoint != "" {
			args = append(args, "endpoint", config.SourceEndpoint)
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (s3): %v\nOutput: %s", err, output)
		}
	case "minio":
		args := []string{
			"config", "create", sourceName, "s3",
			"provider", "Minio",
			"env_auth", "false",
			"access_key_id", config.SourceAccessKey,
			"secret_access_key", config.SourceSecretKey,
			"endpoint", config.SourceEndpoint,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		// Add region if specified
		if config.SourceRegion != "" {
			args = append(args, "region", config.SourceRegion)
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (minio): %v\nOutput: %s", err, output)
		}
	// ... (Add cases for other source types: b2, smb, ftp, webdav, nextcloud, onedrive, gdrive, gphotos) ...
	case "local":
		// For local source, ensure the section exists but might not need specific rclone config create
		content := fmt.Sprintf("[%s]\ntype = local\n\n", sourceName)
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write source config (local): %v", err)
		}
	default:
		// Handle unknown or unsupported source types if necessary
		return fmt.Errorf("unsupported source type for rclone config generation: %s", config.SourceType)

	}

	destName := fmt.Sprintf("dest_%d", config.ID)
	// Generate rclone config using rclone CLI for destination
	switch config.DestinationType {
	case "sftp":
		args := []string{
			"config", "create", destName, "sftp",
			"host", config.DestHost,
			"user", config.DestUser,
			"port", fmt.Sprintf("%d", config.DestPort),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		if config.DestPassword != "" {
			args = append(args, "pass", config.DestPassword)
		}
		if config.DestKeyFile != "" {
			args = append(args, "key_file", config.DestKeyFile)
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (sftp): %v\nOutput: %s", err, output)
		}
	case "s3":
		args := []string{
			"config", "create", destName, "s3",
			"provider", "AWS", // Assuming AWS provider
			"env_auth", "false",
			"access_key_id", config.DestAccessKey,
			"secret_access_key", config.DestSecretKey,
			"region", config.DestRegion,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		if config.DestEndpoint != "" {
			args = append(args, "endpoint", config.DestEndpoint)
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (s3): %v\nOutput: %s", err, output)
		}
	case "minio":
		args := []string{
			"config", "create", destName, "s3",
			"provider", "Minio",
			"env_auth", "false",
			"access_key_id", config.DestAccessKey,
			"secret_access_key", config.DestSecretKey,
			"endpoint", config.DestEndpoint,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		// Add region if specified
		if config.DestRegion != "" {
			args = append(args, "region", config.DestRegion)
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (minio): %v\nOutput: %s", err, output)
		}
	// ... (Add cases for other destination types: b2, smb, ftp, webdav, nextcloud, onedrive, gdrive, gphotos) ...
	case "local":
		// Append local config section
		content := fmt.Sprintf("\n[%s]\ntype = local\n", destName)
		f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to open config file for appending (local dest): %v", err)
		}
		defer f.Close()
		if _, err := f.WriteString(content); err != nil {
			return fmt.Errorf("failed to write destination config (local): %v", err)
		}
	default:
		// Handle unknown or unsupported destination types if necessary
		return fmt.Errorf("unsupported destination type for rclone config generation: %s", config.DestinationType)
	}

	return nil
}

// StoreGoogleDriveToken stores the Google Drive auth token for a config
func (db *DB) StoreGoogleDriveToken(configIDStr string, token string) error {
	configID, err := strconv.ParseUint(configIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid config ID: %v", err)
	}

	config, err := db.GetTransferConfig(uint(configID))
	if err != nil {
		return fmt.Errorf("failed to get config: %v", err)
	}

	authenticated := true
	config.GoogleDriveAuthenticated = &authenticated

	if err := db.UpdateTransferConfig(config); err != nil {
		return fmt.Errorf("failed to update config: %v", err)
	}

	configPath := db.GetConfigRclonePath(config)
	existingConfig := ""
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read existing config: %v", err)
		}
		existingConfig = string(data)
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	destName := fmt.Sprintf("dest_%d", config.ID)
	newConfig := fmt.Sprintf("[%s]\ntype = drive\ntoken = %s\n", destName, token)

	if config.DestClientID != "" && config.DestClientSecret != "" {
		newConfig += fmt.Sprintf("client_id = %s\nclient_secret = %s\n", config.DestClientID, config.DestClientSecret)
	}
	if config.DestDriveID != "" {
		newConfig += fmt.Sprintf("root_folder_id = %s\n", config.DestDriveID)
	}
	if config.DestTeamDrive != "" {
		newConfig += fmt.Sprintf("team_drive = %s\n", config.DestTeamDrive)
	}

	var content string
	sectionHeader := fmt.Sprintf("[%s]", destName)
	if strings.Contains(existingConfig, sectionHeader) {
		parts := strings.SplitN(existingConfig, sectionHeader, 2)
		nextSectionIdx := strings.Index(parts[1], "[")
		if nextSectionIdx != -1 {
			content = parts[0] + newConfig + parts[1][nextSectionIdx:]
		} else {
			content = parts[0] + newConfig
		}
	} else {
		content = existingConfig + "\n" + newConfig
	}

	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

// GenerateRcloneConfigWithToken generates a rclone config file for a transfer config with a provided token
// Note: This seems partially redundant with StoreGoogleDriveToken and GenerateRcloneConfig. Consolidate if possible.
func (db *DB) GenerateRcloneConfigWithToken(config *TransferConfig, token string) error {
	configPath := db.GetConfigRclonePath(config)
	if configPath == "" {
		return fmt.Errorf("failed to get config path")
	}

	token = strings.TrimSpace(token)
	token = strings.ReplaceAll(token, "\n", "")
	token = strings.ReplaceAll(token, "\r", "")

	var configType, section, clientID, clientSecret string
	var readOnly, includeArchived *bool
	var startYear int

	// Determine if source or destination needs token update
	if config.DestinationType == "gdrive" || config.DestinationType == "gphotos" {
		configType = config.DestinationType
		section = "dest"
		clientID = config.DestClientID
		clientSecret = config.DestClientSecret
		readOnly = config.DestReadOnly
		startYear = config.DestStartYear
		includeArchived = config.DestIncludeArchived
	} else if config.SourceType == "gdrive" || config.SourceType == "gphotos" {
		configType = config.SourceType
		section = "source"
		clientID = config.SourceClientID
		clientSecret = config.SourceClientSecret
		readOnly = config.SourceReadOnly
		startYear = config.SourceStartYear
		includeArchived = config.SourceIncludeArchived
	} else {
		return fmt.Errorf("config is not for Google Drive or Google Photos")
	}

	contentBytes, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) { // Allow file not existing yet
		return fmt.Errorf("failed to read config file: %v", err)
	}
	content := string(contentBytes)

	var sectionContent string
	sectionHeader := fmt.Sprintf("[%s_%d]", section, config.ID)

	if configType == "gdrive" {
		sectionContent = sectionHeader + "\ntype = drive\n"
		if clientID != "" {
			sectionContent += fmt.Sprintf("client_id = %s\n", clientID)
		}
		if clientSecret != "" {
			sectionContent += fmt.Sprintf("client_secret = %s\n", clientSecret)
		}
		sectionContent += fmt.Sprintf("token = %s\n", token)
		if section == "source" && config.SourceTeamDrive != "" {
			sectionContent += fmt.Sprintf("team_drive = %s\n", config.SourceTeamDrive)
		}
		if section == "dest" && config.DestTeamDrive != "" {
			sectionContent += fmt.Sprintf("team_drive = %s\n", config.DestTeamDrive)
		}
		if section == "dest" && config.DestDriveID != "" {
			sectionContent += fmt.Sprintf("root_folder_id = %s\n", config.DestDriveID)
		} // Use DestDriveID for root_folder_id
	} else if configType == "gphotos" {
		sectionContent = sectionHeader + "\ntype = google photos\n"
		if clientID != "" {
			sectionContent += fmt.Sprintf("client_id = %s\n", clientID)
		}
		if clientSecret != "" {
			sectionContent += fmt.Sprintf("client_secret = %s\n", clientSecret)
		}
		sectionContent += fmt.Sprintf("token = %s\n", token)
		if readOnly != nil && *readOnly {
			sectionContent += "read_only = true\n"
		}
		if startYear > 0 {
			sectionContent += fmt.Sprintf("start_year = %d\n", startYear)
		}
		if includeArchived != nil && *includeArchived {
			sectionContent += "include_archived = true\n"
		}
	}

	// Replace or append logic
	sectionPattern := regexp.MustCompile(fmt.Sprintf(`(?m)^%s[^\[]*`, regexp.QuoteMeta(sectionHeader))) // Match section start to next section or EOF
	if sectionPattern.MatchString(content) {
		content = sectionPattern.ReplaceAllString(content, sectionContent)
	} else {
		if content != "" && !strings.HasSuffix(content, "\n\n") { // Ensure separation
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += "\n"
		}
		content += sectionContent
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Write the updated config file
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil { // Use 0600 for sensitive files
		return fmt.Errorf("failed to write updated config file: %v", err)
	}

	// Update the authentication status in DB
	authenticated := true
	if config.DestinationType == "gdrive" || config.DestinationType == "gphotos" {
		config.SetGoogleAuthenticated(authenticated)
	} else if config.SourceType == "gdrive" || config.SourceType == "gphotos" {
		config.SetGoogleAuthenticated(authenticated)
	}
	// Persist the change (assuming UpdateTransferConfig saves the whole object)
	if err := db.UpdateTransferConfig(config); err != nil {
		return fmt.Errorf("failed to update config authentication status: %v", err)
	}

	return nil
}

// GetGDriveCredentialsFromConfig extracts Google Drive client ID and secret from an existing rclone config file
func (db *DB) GetGDriveCredentialsFromConfig(config *TransferConfig) (string, string) {
	configPath := db.GetConfigRclonePath(config)
	if configPath == "" {
		return "", ""
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", ""
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(content), "\n")
	sourceSectionName := fmt.Sprintf("[source_%d]", config.ID)
	destSectionName := fmt.Sprintf("[dest_%d]", config.ID)
	var inSourceSection, inDestSection bool
	var sourceClientID, sourceClientSecret, destClientID, destClientSecret string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSourceSection = line == sourceSectionName
			inDestSection = line == destSectionName
			continue
		}
		if inSourceSection {
			if strings.HasPrefix(line, "client_id") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					sourceClientID = strings.TrimSpace(parts[1])
				}
			} else if strings.HasPrefix(line, "client_secret") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					sourceClientSecret = strings.TrimSpace(parts[1])
				}
			}
		}
		if inDestSection {
			if strings.HasPrefix(line, "client_id") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					destClientID = strings.TrimSpace(parts[1])
				}
			} else if strings.HasPrefix(line, "client_secret") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					destClientSecret = strings.TrimSpace(parts[1])
				}
			}
		}
		if sourceClientID != "" && sourceClientSecret != "" && destClientID != "" && destClientSecret != "" {
			break
		}
	}

	if destClientID != "" && destClientSecret != "" {
		return destClientID, destClientSecret
	}
	if sourceClientID != "" && sourceClientSecret != "" {
		return sourceClientID, sourceClientSecret
	}
	return "", ""
}
