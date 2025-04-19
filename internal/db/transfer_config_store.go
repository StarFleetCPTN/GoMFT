package db

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/starfleetcptn/gomft/internal/encryption"
)

// --- TransferConfig Store Methods ---

// CreateTransferConfig creates a new transfer config record
func (db *DB) CreateTransferConfig(config *TransferConfig) error {
	return db.Create(config).Error
}

// GetTransferConfigs retrieves all transfer configs for a user
func (db *DB) GetTransferConfigs(userID uint) ([]TransferConfig, error) {
	var configs []TransferConfig
	err := db.Preload("SourceProvider").Preload("DestinationProvider").Where("created_by = ?", userID).Find(&configs).Error
	return configs, err
}

// GetTransferConfig retrieves a single transfer config by ID
func (db *DB) GetTransferConfig(id uint) (*TransferConfig, error) {
	var config TransferConfig
	err := db.Preload("SourceProvider").Preload("DestinationProvider").First(&config, id).Error
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

	// Ensure providers are loaded if using references
	if config.IsUsingSourceProviderReference() && config.SourceProvider == nil {
		provider, err := db.GetStorageProvider(*config.SourceProviderID)
		if err != nil {
			return fmt.Errorf("failed to load source provider (ID %d): %v", *config.SourceProviderID, err)
		}
		config.SetSourceProvider(provider)
	}

	if config.IsUsingDestinationProviderReference() && config.DestinationProvider == nil {
		provider, err := db.GetStorageProvider(*config.DestinationProviderID)
		if err != nil {
			return fmt.Errorf("failed to load destination provider (ID %d): %v", *config.DestinationProviderID, err)
		}
		config.SetDestinationProvider(provider)
	}

	// If we have a destination provider but no ID or zero ID, fix it
	if config.DestinationProvider != nil && (config.DestinationProviderID == nil || *config.DestinationProviderID == 0) {
		config.SetDestinationProvider(config.DestinationProvider)
	}

	// If we have an ID but no provider, load it
	if config.DestinationProviderID != nil && *config.DestinationProviderID > 0 && config.DestinationProvider == nil {
		provider, err := db.GetStorageProvider(*config.DestinationProviderID)
		if err != nil {
			return fmt.Errorf("failed to load destination provider (ID %d): %v", *config.DestinationProviderID, err)
		}
		config.SetDestinationProvider(provider)
	}

	// Double check that everything is synchronized
	if config.IsUsingDestinationProviderReference() {
		if config.DestinationProvider == nil {
			return fmt.Errorf("destination provider reference is set (ID %d) but provider is nil", *config.DestinationProviderID)
		}
		if config.DestinationProviderID == nil || *config.DestinationProviderID != config.DestinationProvider.ID {
			config.SetDestinationProvider(config.DestinationProvider) // Re-sync the ID
		}
	}

	// Get source credentials, either from provider or directly from config
	sourceCredentials, err := config.GetSourceCredentials(db)
	if err != nil {
		return fmt.Errorf("failed to get source credentials: %v", err)
	}

	// Get source type either from provider or directly from config
	sourceType := config.SourceType
	if sourceTypeFromCreds, ok := sourceCredentials["type"].(StorageProviderType); ok {
		sourceType = string(sourceTypeFromCreds)
	} else if sourceTypeFromCreds, ok := sourceCredentials["type"].(string); ok {
		sourceType = sourceTypeFromCreds
	}

	sourceName := fmt.Sprintf("source_%d", config.ID)
	fmt.Printf("Generated source name: %s\n", sourceName)
	fmt.Printf("Final source type being used: %s\n", sourceType)

	// Generate rclone config using rclone CLI for source
	switch sourceType {
	case "sftp", "hetzner":
		args := []string{
			"config", "create", sourceName, "sftp",
			"host", getStringValue(sourceCredentials, "host", config.SourceHost),
			"user", getStringValue(sourceCredentials, "username", config.SourceUser),
			"port", fmt.Sprintf("%d", getIntValue(sourceCredentials, "port", config.SourcePort)),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// First try to get password from direct form input (transient)
		password := ""
		if config.SourcePassword != "" {
			password = config.SourcePassword
		} else if encryptedPwd, ok := sourceCredentials["encrypted_password"].(string); ok && encryptedPwd != "" {
			// For provider references, get the decrypted password
			decryptedPwd, err := db.DecryptCredential(encryptedPwd)
			if err != nil {
				return fmt.Errorf("failed to decrypt source password: %v", err)
			}
			password = decryptedPwd
		} else if pwVal, ok := sourceCredentials["password"].(string); ok && pwVal != "" {
			// For backward compatibility
			password = pwVal
		}

		if password != "" {
			args = append(args, "pass", password)
		}

		keyFile := getStringValue(sourceCredentials, "key_file", config.SourceKeyFile)
		if keyFile != "" {
			args = append(args, "key_file", keyFile)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (sftp): %v\nOutput: %s", err, output)
		}
	case "smb":
		args := []string{
			"config", "create", sourceName, "smb",
			"host", getStringValue(sourceCredentials, "host", config.SourceHost),
			"user", getStringValue(sourceCredentials, "username", config.SourceUser),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Get domain if provided
		domain := getStringValue(sourceCredentials, "domain", config.SourceDomain)
		if domain != "" {
			args = append(args, "domain", domain)
		}

		// Get port if specified (default is 445)
		port := getIntValue(sourceCredentials, "port", config.SourcePort)
		if port > 0 && port != 445 {
			args = append(args, "port", fmt.Sprintf("%d", port))
		}

		// Get share if provided
		share := getStringValue(sourceCredentials, "share", config.SourceShare)
		if share != "" {
			args = append(args, "share", share)
		}

		// Handle password
		password := ""
		if config.SourcePassword != "" {
			password = config.SourcePassword
		} else if encryptedPwd, ok := sourceCredentials["encrypted_password"].(string); ok && encryptedPwd != "" {
			decryptedPwd, err := db.DecryptCredential(encryptedPwd)
			if err != nil {
				return fmt.Errorf("failed to decrypt source password: %v", err)
			}
			password = decryptedPwd
		} else if pwVal, ok := sourceCredentials["password"].(string); ok && pwVal != "" {
			password = pwVal
		}

		if password != "" {
			args = append(args, "pass", password)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (smb): %v\nOutput: %s", err, output)
		}
	case "s3":
		args := []string{
			"config", "create", sourceName, "s3",
			"provider", "AWS", // Assuming AWS provider, adjust if needed
			"env_auth", "false",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Handle access key
		accessKey := getStringValue(sourceCredentials, "access_key", config.SourceAccessKey)
		if accessKey != "" {
			args = append(args, "access_key_id", accessKey)
		}

		// Handle secret key with proper decryption if from provider
		secretKey := ""
		if config.SourceSecretKey != "" {
			// Direct input from form (transient)
			secretKey = config.SourceSecretKey
		} else if encryptedSecret, ok := sourceCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt source secret key: %v", err)
			}
			secretKey = decryptedSecret
		} else if secretVal, ok := sourceCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			secretKey = secretVal
		}

		if secretKey != "" {
			args = append(args, "secret_access_key", secretKey)
		}

		// Add region
		region := getStringValue(sourceCredentials, "region", config.SourceRegion)
		if region != "" {
			args = append(args, "region", region)
		}

		endpoint := getStringValue(sourceCredentials, "endpoint", config.SourceEndpoint)
		if endpoint != "" {
			args = append(args, "endpoint", endpoint)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (s3): %v\nOutput: %s", err, output)
		}
	case "wasabi":
		args := []string{
			"config", "create", sourceName, "s3",
			"provider", "Wasabi",
			"env_auth", "false",
			"access_key_id", getStringValue(sourceCredentials, "access_key", config.SourceAccessKey),
			"secret_access_key", getStringOrDefault(sourceCredentials, "secret_key", config.SourceSecretKey),
			"region", getStringValue(sourceCredentials, "region", config.SourceRegion),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		endpoint := getStringValue(sourceCredentials, "endpoint", config.SourceEndpoint)
		if endpoint == "" {
			endpoint = "s3.wasabisys.com"
		}

		args = append(args, "endpoint", endpoint)
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (wasabi): %v\nOutput: %s", err, output)
		}
	case "b2":
		args := []string{
			"config", "create", sourceName, "b2",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		// Handle account ID (access key)
		accountID := getStringValue(sourceCredentials, "access_key", config.SourceAccessKey)
		if accountID != "" {
			args = append(args, "account", accountID)
		}

		// Handle secret key with proper decryption if from provider
		secretKey := ""
		if config.SourceSecretKey != "" {
			// Direct input from form (transient)
			secretKey = config.SourceSecretKey
		} else if encryptedSecret, ok := sourceCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt source secret key: %v", err)
			}
			secretKey = decryptedSecret
		} else if secretVal, ok := sourceCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			secretKey = secretVal
		}

		if secretKey != "" {
			args = append(args, "key", secretKey)
		}
		if config.SourceEndpoint != "" {
			args = append(args, "endpoint", config.SourceEndpoint)
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (b2): %v\nOutput: %s", err, output)
		}
	case "minio":
		args := []string{
			"config", "create", sourceName, "s3",
			"provider", "Minio",
			"env_auth", "false",
			"access_key_id", getStringValue(sourceCredentials, "access_key", config.SourceAccessKey),
			"endpoint", getStringValue(sourceCredentials, "endpoint", config.SourceEndpoint),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		// Handle secret key with proper decryption if from provider
		secretKey := ""
		if config.SourceSecretKey != "" {
			// Direct input from form (transient)
			secretKey = config.SourceSecretKey
		} else if encryptedSecret, ok := sourceCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt source secret key: %v", err)
			}
			secretKey = decryptedSecret
		} else if secretVal, ok := sourceCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			secretKey = secretVal
		}

		if secretKey != "" {
			args = append(args, "secret_access_key", secretKey)
		}

		// Add region if specified
		if getStringValue(sourceCredentials, "region", config.SourceRegion) != "" {
			args = append(args, "region", getStringValue(sourceCredentials, "region", config.SourceRegion))
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config (minio): %v\nOutput: %s", err, output)
		}
	case "webdav", "nextcloud": // Handle both webdav and nextcloud similarly
		// Construct the WebDAV URL
		// Parse the provided source URL, assuming it includes the scheme
		inputURL := getStringValue(sourceCredentials, "host", config.SourceHost)
		fmt.Printf("Input URL: %s\n", inputURL)
		parsedURL, err := url.Parse(inputURL)
		if err != nil {
			return fmt.Errorf("failed to parse source URL '%s': %v", inputURL, err)
		}
		// Validate that both scheme and host are present
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("invalid source URL '%s': must include scheme (http/https) and host", inputURL)
		}
		// Use the scheme and host from the parsed URL
		webdavURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

		// Determine vendor based on type
		vendor := "other" // Default vendor
		if config.SourceType == "nextcloud" {
			vendor = "nextcloud"

			// Construct the full Nextcloud path using the parsed base URL
			webdavURL = fmt.Sprintf("%s/remote.php/dav/files/%s/", webdavURL, config.SourceUser)
		}

		args := []string{
			"config", "create", sourceName, "webdav",
			"url", webdavURL,
			"vendor", vendor,
			"user", getStringValue(sourceCredentials, "username", config.SourceUser),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Handle password
		password := ""
		if config.SourcePassword != "" {
			password = config.SourcePassword
		} else if encryptedPwd, ok := sourceCredentials["encrypted_password"].(string); ok && encryptedPwd != "" {
			decryptedPwd, err := db.DecryptCredential(encryptedPwd)
			if err != nil {
				return fmt.Errorf("failed to decrypt source password: %v", err)
			}
			password = decryptedPwd
		} else if pwVal, ok := sourceCredentials["password"].(string); ok && pwVal != "" {
			password = pwVal
		}

		if password != "" {
			args = append(args, "pass", password)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			errorMsg := fmt.Sprintf("failed to create source config (%s): %v", config.SourceType, err)
			// Check if output contains useful info, especially for auth errors
			if len(output) > 0 {
				errorMsg += fmt.Sprintf("\nOutput: %s", output)
			}
			return fmt.Errorf("%v", errorMsg)
		}

	case "local":
		// For local source, ensure the section exists but might not need specific rclone config create
		content := fmt.Sprintf("[%s]\ntype = local\n\n", sourceName)
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write source config (local): %v", err)
		}
	case "gdrive":
		// For Google Drive, we need client ID and secret
		clientID := getStringValue(sourceCredentials, "client_id", config.SourceClientID)

		// Get client secret with proper decryption if from provider
		clientSecret := ""
		if config.SourceClientSecret != "" {
			// Direct input from form (transient)
			clientSecret = config.SourceClientSecret
		} else if encryptedSecret, ok := sourceCredentials["encrypted_client_secret"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt source client secret: %v", err)
			}
			clientSecret = decryptedSecret
		}

		// Get refresh token if available
		refreshToken := getStringOrDefault(sourceCredentials, "token", "")
		if refreshToken == "" {
			refreshToken = getStringOrDefault(sourceCredentials, "refresh_token", "")
		}

		if refreshToken == "" {
			if encryptedToken, ok := sourceCredentials["encrypted_refresh_token"].(string); ok && encryptedToken != "" {
				decryptedToken, err := db.DecryptCredential(encryptedToken)
				if err != nil {
					return fmt.Errorf("failed to decrypt source refresh token: %v", err)
				}
				refreshToken = decryptedToken
			}
		}

		// If not found in credentials, check if using a provider reference
		if refreshToken == "" && config.IsUsingSourceProviderReference() && config.SourceProvider != nil {
			refreshToken = config.SourceProvider.RefreshToken
		}

		// Clean up the token
		if refreshToken != "" {
			refreshToken = strings.TrimSpace(refreshToken)
			refreshToken = strings.ReplaceAll(refreshToken, "\n", "")
			refreshToken = strings.ReplaceAll(refreshToken, "\r", "")
			refreshToken = strings.Join(strings.Fields(refreshToken), "")
		}

		// Create rclone config for Google Drive
		args := []string{
			"config", "create", sourceName, "drive",
			"client_id", clientID,
			"client_secret", clientSecret,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Add team drive or drive ID if specified
		teamDrive := getStringValue(sourceCredentials, "team_drive", config.SourceTeamDrive)
		if teamDrive != "" {
			args = append(args, "team_drive", teamDrive)
		}

		driveID := getStringValue(sourceCredentials, "drive_id", config.SourceDriveID)
		if driveID != "" {
			args = append(args, "drive_id", driveID)
		}

		// If we have a refresh token, add it
		if refreshToken != "" {
			args = append(args, "token", fmt.Sprintf("%s", refreshToken))
		}

		cmd := exec.Command(rclonePath, args...)

		if output, err := cmd.CombinedOutput(); err != nil {
			errorMsg := fmt.Sprintf("failed to create source config (drive): %v", err)
			// Check if output contains useful info, especially for auth errors
			if len(output) > 0 {
				errorMsg += fmt.Sprintf("\nOutput: %s", output)
			}
			return fmt.Errorf("%v", errorMsg)
		}
	case "gphotos":
		// For Google Photos, we need client ID and secret
		clientID := getStringValue(sourceCredentials, "client_id", config.SourceClientID)

		// Get client secret with proper decryption if from provider
		clientSecret := ""
		if config.SourceClientSecret != "" {
			// Direct input from form (transient)
			clientSecret = config.SourceClientSecret
		} else if encryptedSecret, ok := sourceCredentials["encrypted_client_secret"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt source client secret: %v", err)
			}
			clientSecret = decryptedSecret
		}

		// Get refresh token if available
		refreshToken := getStringOrDefault(sourceCredentials, "token", "")
		if refreshToken == "" {
			refreshToken = getStringOrDefault(sourceCredentials, "refresh_token", "")
		}

		if refreshToken == "" {
			if encryptedToken, ok := sourceCredentials["encrypted_refresh_token"].(string); ok && encryptedToken != "" {
				decryptedToken, err := db.DecryptCredential(encryptedToken)
				if err != nil {
					return fmt.Errorf("failed to decrypt source refresh token: %v", err)
				}
				refreshToken = decryptedToken
			}
		}

		// Clean up the token
		if refreshToken != "" {
			refreshToken = strings.TrimSpace(refreshToken)
			refreshToken = strings.ReplaceAll(refreshToken, "\n", "")
			refreshToken = strings.ReplaceAll(refreshToken, "\r", "")
			refreshToken = strings.Join(strings.Fields(refreshToken), "")
		}

		// Create rclone config for Google Photos
		args := []string{
			"config", "create", sourceName, "gphotos",
			"client_id", clientID,
			"client_secret", clientSecret,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Add read-only flag if specified
		readOnly := false
		if readOnlyVal, ok := sourceCredentials["read_only"].(bool); ok {
			readOnly = readOnlyVal
		} else if config.SourceReadOnly != nil {
			readOnly = *config.SourceReadOnly
		}
		if readOnly {
			args = append(args, "read_only", "true")
		}

		// Add start year if specified
		startYear := getIntValue(sourceCredentials, "start_year", config.SourceStartYear)
		if startYear > 0 {
			args = append(args, "start_year", fmt.Sprintf("%d", startYear))
		}

		// Add include archived if specified
		includeArchived := false
		if includeArchivedVal, ok := sourceCredentials["include_archived"].(bool); ok {
			includeArchived = includeArchivedVal
		} else if config.SourceIncludeArchived != nil {
			includeArchived = *config.SourceIncludeArchived
		}
		if includeArchived {
			args = append(args, "include_archived", "true")
		}

		// If we have a refresh token, add it
		if refreshToken != "" {
			args = append(args, "token", fmt.Sprintf("%s", refreshToken))
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			errorMsg := fmt.Sprintf("failed to create source config (gphotos): %v", err)
			// Check if output contains useful info, especially for auth errors
			if len(output) > 0 {
				errorMsg += fmt.Sprintf("\nOutput: %s", output)
			}
			return fmt.Errorf("%v", errorMsg)
		}
	default:
		// Handle unknown or unsupported source types if necessary
		return fmt.Errorf("unsupported source type for rclone config generation: %s", config.SourceType)

	}

	// Get destination credentials, either from provider or directly from config
	destCredentials, err := config.GetDestinationCredentials(db)
	if err != nil {
		return fmt.Errorf("failed to get destination credentials: %v", err)
	}

	// Get destination type either from provider or directly from config
	destType := config.DestinationType
	if destTypeFromCreds, ok := destCredentials["type"].(StorageProviderType); ok {
		destType = string(destTypeFromCreds)
	} else if destTypeFromCreds, ok := destCredentials["type"].(string); ok {
		destType = destTypeFromCreds
	}

	destName := fmt.Sprintf("dest_%d", config.ID)
	// Generate rclone config using rclone CLI for destination
	switch destType {
	case "sftp", "hetzner":
		args := []string{
			"config", "create", destName, "sftp",
			"host", getStringValue(destCredentials, "host", config.DestHost),
			"user", getStringValue(destCredentials, "username", config.DestUser),
			"port", fmt.Sprintf("%d", getIntValue(destCredentials, "port", config.DestPort)),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		password := ""
		if config.DestPassword != "" {
			password = config.DestPassword
		} else if encryptedPwd, ok := destCredentials["encrypted_password"].(string); ok && encryptedPwd != "" {
			// For provider references, get the decrypted password
			decryptedPwd, err := db.DecryptCredential(encryptedPwd)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination password: %v", err)
			}
			password = decryptedPwd
		} else if pwVal, ok := destCredentials["password"].(string); ok && pwVal != "" {
			// For backward compatibility
			password = pwVal
		}

		if password != "" {
			args = append(args, "pass", password)
		}

		keyFile := getStringValue(destCredentials, "key_file", config.DestKeyFile)
		if keyFile != "" {
			args = append(args, "key_file", keyFile)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (sftp): %v\nOutput: %s", err, output)
		}
	case "smb":
		args := []string{
			"config", "create", destName, "smb",
			"host", getStringValue(destCredentials, "host", config.DestHost),
			"user", getStringValue(destCredentials, "username", config.DestUser),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Get domain if provided
		domain := getStringValue(destCredentials, "domain", config.DestDomain)
		if domain != "" {
			args = append(args, "domain", domain)
		}

		// Get port if specified (default is 445)
		port := getIntValue(destCredentials, "port", config.DestPort)
		if port > 0 && port != 445 {
			args = append(args, "port", fmt.Sprintf("%d", port))
		}

		// Get share if provided
		share := getStringValue(destCredentials, "share", config.DestShare)
		if share != "" {
			args = append(args, "share", share)
		}

		// Handle password
		password := ""
		if config.DestPassword != "" {
			password = config.DestPassword
		} else if encryptedPwd, ok := destCredentials["encrypted_password"].(string); ok && encryptedPwd != "" {
			decryptedPwd, err := db.DecryptCredential(encryptedPwd)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination password: %v", err)
			}
			password = decryptedPwd
		} else if pwVal, ok := destCredentials["password"].(string); ok && pwVal != "" {
			password = pwVal
		}

		if password != "" {
			args = append(args, "pass", password)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (smb): %v\nOutput: %s", err, output)
		}
	case "s3":
		args := []string{
			"config", "create", destName, "s3",
			"provider", "AWS", // Assuming AWS provider
			"env_auth", "false",
			"access_key_id", getStringValue(destCredentials, "access_key", config.DestAccessKey),
			"region", getStringValue(destCredentials, "region", config.DestRegion),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Handle secret key with proper decryption if from provider
		secretKey := ""
		if config.DestSecretKey != "" {
			// Direct input from form (transient)
			secretKey = config.DestSecretKey
		} else if encryptedSecret, ok := destCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination secret key: %v", err)
			}
			secretKey = decryptedSecret
		} else if secretVal, ok := destCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			secretKey = secretVal
		}

		if secretKey != "" {
			args = append(args, "secret_access_key", secretKey)
		}

		endpoint := getStringValue(destCredentials, "endpoint", config.DestEndpoint)
		if endpoint != "" {
			args = append(args, "endpoint", endpoint)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (s3): %v\nOutput: %s", err, output)
		}
	case "wasabi":
		args := []string{
			"config", "create", destName, "s3",
			"provider", "Wasabi",
			"env_auth", "false",
			"access_key_id", getStringValue(destCredentials, "access_key", config.DestAccessKey),
			"region", getStringValue(destCredentials, "region", config.DestRegion),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Handle secret key with proper decryption if from provider
		secretKey := ""
		if config.DestSecretKey != "" {
			// Direct input from form (transient)
			secretKey = config.DestSecretKey
		} else if encryptedSecret, ok := destCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination secret key: %v", err)
			}
			secretKey = decryptedSecret
		} else if secretVal, ok := destCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			secretKey = secretVal
		}

		if secretKey != "" {
			args = append(args, "secret_access_key", secretKey)
		}

		endpoint := getStringValue(destCredentials, "endpoint", config.DestEndpoint)
		if endpoint == "" {
			endpoint = "s3.wasabisys.com"
		}

		args = append(args, "endpoint", endpoint)
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (wasabi): %v\nOutput: %s", err, output)
		}
	case "b2":
		args := []string{
			"config", "create", destName, "b2",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Handle account ID (access key)
		accountID := getStringValue(destCredentials, "access_key", config.DestAccessKey)
		if accountID != "" {
			args = append(args, "account", accountID)
		}

		// Handle secret key with proper decryption if from provider
		secretKey := ""
		if config.DestSecretKey != "" {
			// Direct input from form (transient)
			secretKey = config.DestSecretKey
		} else if encryptedSecret, ok := destCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination secret key: %v", err)
			}
			secretKey = decryptedSecret
		} else if secretVal, ok := destCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			secretKey = secretVal
		}

		if secretKey != "" {
			args = append(args, "key", secretKey)
		}

		// Handle application key (secret key) with proper decryption if from provider
		appKey := ""
		if config.DestSecretKey != "" {
			// Direct input from form (transient)
			appKey = config.DestSecretKey
		} else if encryptedSecret, ok := destCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination secret key: %v", err)
			}
			appKey = decryptedSecret
		} else if secretVal, ok := destCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			appKey = secretVal
		}

		if appKey != "" {
			args = append(args, "key", appKey)
		}

		endpoint := getStringValue(destCredentials, "endpoint", config.DestEndpoint)
		if endpoint != "" {
			args = append(args, "endpoint", endpoint)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (b2): %v\nOutput: %s", err, output)
		}
	case "minio":
		args := []string{
			"config", "create", destName, "s3",
			"provider", "Minio",
			"env_auth", "false",
			"access_key_id", getStringValue(destCredentials, "access_key", config.DestAccessKey),
			"endpoint", getStringValue(destCredentials, "endpoint", config.DestEndpoint),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Handle secret key with proper decryption if from provider
		secretKey := ""
		if config.DestSecretKey != "" {
			// Direct input from form (transient)
			secretKey = config.DestSecretKey
		} else if encryptedSecret, ok := destCredentials["encrypted_secret_key"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination secret key: %v", err)
			}
			secretKey = decryptedSecret
		} else if secretVal, ok := destCredentials["secret_key"].(string); ok && secretVal != "" {
			// Backward compatibility
			secretKey = secretVal
		}

		if secretKey != "" {
			args = append(args, "secret_access_key", secretKey)
		}

		// Add region if specified
		if getStringValue(destCredentials, "region", config.DestRegion) != "" {
			args = append(args, "region", getStringValue(destCredentials, "region", config.DestRegion))
		}
		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config (minio): %v\nOutput: %s", err, output)
		}
	case "webdav", "nextcloud": // Combined case for WebDAV and Nextcloud
		// Parse and reconstruct the WebDAV URL robustly
		// Parse the provided destination URL, assuming it includes the scheme
		inputURL := getStringValue(destCredentials, "host", config.DestHost)
		parsedURL, err := url.Parse(inputURL)
		if err != nil {
			return fmt.Errorf("failed to parse destination URL '%s': %v", inputURL, err)
		}
		// Validate that both scheme and host are present
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("invalid destination URL '%s': must include scheme (http/https) and host", inputURL)
		}
		// Use the scheme and host from the parsed URL
		webdavURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

		// Determine vendor based on type
		vendor := "other" // Default vendor
		if config.DestinationType == "nextcloud" {
			vendor = "nextcloud"

			webdavURL = fmt.Sprintf("%s/remote.php/dav/files/%s/", webdavURL, getStringValue(destCredentials, "username", config.DestUser)) // Corrected variable
		}

		args := []string{
			"config", "create", destName, "webdav",
			"url", webdavURL, // Use the parsed and reconstructed URL
			"vendor", vendor,
			"user", getStringValue(destCredentials, "username", config.DestUser),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Handle password with proper decryption if from provider
		password := ""
		if config.DestPassword != "" {
			// Direct input from form (transient)
			password = config.DestPassword
		} else if encryptedPwd, ok := destCredentials["encrypted_password"].(string); ok && encryptedPwd != "" {
			// Provider reference with encrypted password
			decryptedPwd, err := db.DecryptCredential(encryptedPwd)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination password: %v", err)
			}
			password = decryptedPwd
		} else if pwVal, ok := destCredentials["password"].(string); ok && pwVal != "" {
			// Backward compatibility
			password = pwVal
		}

		if password != "" {
			args = append(args, "pass", password)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			errorMsg := fmt.Sprintf("failed to create destination config (%s): %v", config.DestinationType, err)
			if len(output) > 0 {
				errorMsg += fmt.Sprintf("\nOutput: %s", output)
			}
			return fmt.Errorf("%v", errorMsg)
		}
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
	case "gdrive":
		// For Google Drive, we need client ID and secret
		clientID := getStringValue(destCredentials, "client_id", config.DestClientID)

		// Get client secret with proper decryption if from provider
		clientSecret := ""
		if config.DestClientSecret != "" {
			// Direct input from form (transient)
			clientSecret = config.DestClientSecret
		} else if encryptedSecret, ok := destCredentials["encrypted_client_secret"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination client secret: %v", err)
			}
			clientSecret = decryptedSecret
		}

		// Get refresh token if available
		refreshToken := getStringOrDefault(destCredentials, "token", "")
		if refreshToken == "" {
			refreshToken = getStringOrDefault(destCredentials, "refresh_token", "")
		}

		if refreshToken == "" {
			if encryptedToken, ok := destCredentials["encrypted_refresh_token"].(string); ok && encryptedToken != "" {
				decryptedToken, err := db.DecryptCredential(encryptedToken)
				if err != nil {
					return fmt.Errorf("failed to decrypt source refresh token: %v", err)
				}
				refreshToken = decryptedToken
			}
		}

		// Clean up the token
		if refreshToken != "" {
			refreshToken = strings.TrimSpace(refreshToken)
			refreshToken = strings.ReplaceAll(refreshToken, "\n", "")
			refreshToken = strings.ReplaceAll(refreshToken, "\r", "")
			refreshToken = strings.Join(strings.Fields(refreshToken), "")
		}

		// Create rclone config for Google Drive
		args := []string{
			"config", "create", destName, "drive",
			"client_id", clientID,
			"client_secret", clientSecret,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Add team drive or drive ID if specified
		teamDrive := getStringValue(destCredentials, "team_drive", config.DestTeamDrive)
		if teamDrive != "" {
			args = append(args, "team_drive", teamDrive)
		}

		driveID := getStringValue(destCredentials, "drive_id", config.DestDriveID)
		if driveID != "" {
			args = append(args, "drive_id", driveID)
		}

		// If we have a refresh token, add it
		if refreshToken != "" {
			args = append(args, "token", fmt.Sprintf("%s", refreshToken))
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			errorMsg := fmt.Sprintf("failed to create destination config (drive): %v", err)
			// Check if output contains useful info, especially for auth errors
			if len(output) > 0 {
				errorMsg += fmt.Sprintf("\nOutput: %s", output)
			}
			return fmt.Errorf("%v", errorMsg)
		}
	case "gphotos":
		// For Google Photos, we need client ID and secret
		clientID := getStringValue(destCredentials, "client_id", config.DestClientID)

		// Get client secret with proper decryption if from provider
		clientSecret := ""
		if config.DestClientSecret != "" {
			// Direct input from form (transient)
			clientSecret = config.DestClientSecret
		} else if encryptedSecret, ok := destCredentials["encrypted_client_secret"].(string); ok && encryptedSecret != "" {
			// Provider reference with encrypted secret
			decryptedSecret, err := db.DecryptCredential(encryptedSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt destination client secret: %v", err)
			}
			clientSecret = decryptedSecret
		}

		// Get refresh token if available
		refreshToken := getStringOrDefault(destCredentials, "token", "")
		if refreshToken == "" {
			refreshToken = getStringOrDefault(destCredentials, "refresh_token", "")
		}

		if refreshToken == "" {
			if encryptedToken, ok := destCredentials["encrypted_refresh_token"].(string); ok && encryptedToken != "" {
				decryptedToken, err := db.DecryptCredential(encryptedToken)
				if err != nil {
					return fmt.Errorf("failed to decrypt source refresh token: %v", err)
				}
				refreshToken = decryptedToken
			}
		}

		// Clean up the token
		if refreshToken != "" {
			refreshToken = strings.TrimSpace(refreshToken)
			refreshToken = strings.ReplaceAll(refreshToken, "\n", "")
			refreshToken = strings.ReplaceAll(refreshToken, "\r", "")
		}

		// Create rclone config for Google Photos
		args := []string{
			"config", "create", destName, "gphotos",
			"client_id", clientID,
			"client_secret", clientSecret,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Add read-only flag if specified
		readOnly := false
		if readOnlyVal, ok := destCredentials["read_only"].(bool); ok {
			readOnly = readOnlyVal
		} else if config.DestReadOnly != nil {
			readOnly = *config.DestReadOnly
		}
		if readOnly {
			args = append(args, "read_only", "true")
		}

		// Add start year if specified
		startYear := getIntValue(destCredentials, "start_year", config.DestStartYear)
		if startYear > 0 {
			args = append(args, "start_year", fmt.Sprintf("%d", startYear))
		}

		// Add include archived if specified
		includeArchived := false
		if includeArchivedVal, ok := destCredentials["include_archived"].(bool); ok {
			includeArchived = includeArchivedVal
		} else if config.DestIncludeArchived != nil {
			includeArchived = *config.DestIncludeArchived
		}
		if includeArchived {
			args = append(args, "include_archived", "true")
		}

		// If we have a refresh token, add it
		if refreshToken != "" {
			args = append(args, "token", fmt.Sprintf("%s", refreshToken))
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			errorMsg := fmt.Sprintf("failed to create destination config (gphotos): %v", err)
			// Check if output contains useful info, especially for auth errors
			if len(output) > 0 {
				errorMsg += fmt.Sprintf("\nOutput: %s", output)
			}
			return fmt.Errorf("%v", errorMsg)
		}
	default:
		// Handle unknown or unsupported destination types if necessary
		return fmt.Errorf("unsupported destination type for rclone config generation: %s", config.DestinationType)
	}

	return nil
}

// Helper functions to get values from credentials map
func getStringValue(creds map[string]interface{}, key, defaultValue string) string {
	if val, ok := creds[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

func getStringOrDefault(creds map[string]interface{}, key, defaultValue string) string {
	if defaultValue != "" {
		return defaultValue // Prefer the value passed directly for sensitive fields
	}
	if val, ok := creds[key].(string); ok {
		return val
	}
	return ""
}

func getIntValue(creds map[string]interface{}, key string, defaultValue int) int {
	if val, ok := creds[key].(int); ok {
		return val
	}
	return defaultValue
}

// StoreGoogleDriveToken stores the Google Drive auth token for a config
func (db *DB) StoreGoogleDriveToken(configIDStr string, token string) error {
	// Remove all whitespace to ensure the token is a single line
	token = strings.Join(strings.Fields(token), "")
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

	// Check if we're using a provider reference and update the provider instead
	if config.IsUsingDestinationProviderReference() && config.DestinationProvider != nil &&
		(config.DestinationProvider.Type == "gdrive" || config.DestinationProvider.Type == "gphotos") {
		// Update the provider with the token
		provider := config.DestinationProvider
		provider.RefreshToken = token // Set the clear token temporarily
		provider.SetAuthenticated(true)

		// Update the provider in the database
		if err := db.UpdateStorageProvider(provider); err != nil {
			return fmt.Errorf("failed to update provider with token: %v", err)
		}

		// Continue with creating the rclone config file since this is still needed for transfers
	} else if config.IsUsingSourceProviderReference() && config.SourceProvider != nil &&
		(config.SourceProvider.Type == "gdrive" || config.SourceProvider.Type == "gphotos") {
		// Update the provider with the token
		provider := config.SourceProvider
		provider.RefreshToken = token // Set the clear token temporarily
		provider.SetAuthenticated(true)

		// Update the provider in the database
		if err := db.UpdateStorageProvider(provider); err != nil {
			return fmt.Errorf("failed to update provider with token: %v", err)
		}

		// Continue with creating the rclone config file since this is still needed for transfers
	}

	// Legacy fallback for direct token storage in config file
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

// ConvertToProviderReferences converts a TransferConfig that uses embedded credentials
// to one that uses StorageProvider references.
func (db *DB) ConvertToProviderReferences(config *TransferConfig) error {
	// Skip if already using both provider references
	if config.IsUsingProviderReferences() {
		return nil
	}

	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Convert source if needed
	if !config.IsUsingSourceProviderReference() && config.SourceType != "" {
		// Create new provider from source fields
		provider := &StorageProvider{
			Name:      fmt.Sprintf("%s Source - %s", config.Name, config.SourceType),
			Type:      StorageProviderType(config.SourceType),
			CreatedBy: config.CreatedBy,
			// Copy all relevant source fields to provider fields
			Host:            config.SourceHost,
			Port:            config.SourcePort,
			Username:        config.SourceUser,
			KeyFile:         config.SourceKeyFile,
			Bucket:          config.SourceBucket,
			Region:          config.SourceRegion,
			AccessKey:       config.SourceAccessKey,
			Share:           config.SourceShare,
			Domain:          config.SourceDomain,
			PassiveMode:     config.SourcePassiveMode,
			ClientID:        config.SourceClientID,
			DriveID:         config.SourceDriveID,
			TeamDrive:       config.SourceTeamDrive,
			ReadOnly:        config.SourceReadOnly,
			StartYear:       config.SourceStartYear,
			IncludeArchived: config.SourceIncludeArchived,
			UseBuiltinAuth:  config.UseBuiltinAuthSource,
		}

		// Handle fields that need encryption
		if config.SourcePassword != "" {
			encryptedPwd, err := db.EncryptCredential(config.SourcePassword)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to encrypt source password: %v", err)
			}
			provider.EncryptedPassword = encryptedPwd
		}

		if config.SourceSecretKey != "" {
			encryptedSecret, err := db.EncryptCredential(config.SourceSecretKey)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to encrypt source secret key: %v", err)
			}
			provider.EncryptedSecretKey = encryptedSecret
		}

		if config.SourceClientSecret != "" {
			encryptedClientSecret, err := db.EncryptCredential(config.SourceClientSecret)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to encrypt source client secret: %v", err)
			}
			provider.EncryptedClientSecret = encryptedClientSecret
		}

		// Save the new provider
		if err := tx.Create(provider).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create source provider: %v", err)
		}

		// Update the config to reference the new provider
		config.SetSourceProvider(provider)
	}

	// Convert destination if needed
	if !config.IsUsingDestinationProviderReference() && config.DestinationType != "" {
		// Create new provider from destination fields
		provider := &StorageProvider{
			Name:      fmt.Sprintf("%s Destination - %s", config.Name, config.DestinationType),
			Type:      StorageProviderType(config.DestinationType),
			CreatedBy: config.CreatedBy,
			// Copy all relevant destination fields to provider fields
			Host:            config.DestHost,
			Port:            config.DestPort,
			Username:        config.DestUser,
			KeyFile:         config.DestKeyFile,
			Bucket:          config.DestBucket,
			Region:          config.DestRegion,
			AccessKey:       config.DestAccessKey,
			Share:           config.DestShare,
			Domain:          config.DestDomain,
			PassiveMode:     config.DestPassiveMode,
			ClientID:        config.DestClientID,
			DriveID:         config.DestDriveID,
			TeamDrive:       config.DestTeamDrive,
			ReadOnly:        config.DestReadOnly,
			StartYear:       config.DestStartYear,
			IncludeArchived: config.DestIncludeArchived,
			UseBuiltinAuth:  config.UseBuiltinAuthDest,
		}

		// For Google Drive/Photos, carry over authentication status
		if config.DestinationType == "gdrive" || config.DestinationType == "gphotos" {
			provider.SetAuthenticated(config.GetGoogleAuthenticated())
		}

		// Handle fields that need encryption
		if config.DestPassword != "" {
			encryptedPwd, err := db.EncryptCredential(config.DestPassword)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to encrypt destination password: %v", err)
			}
			provider.EncryptedPassword = encryptedPwd
		}

		if config.DestSecretKey != "" {
			encryptedSecret, err := db.EncryptCredential(config.DestSecretKey)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to encrypt destination secret key: %v", err)
			}
			provider.EncryptedSecretKey = encryptedSecret
		}

		if config.DestClientSecret != "" {
			encryptedClientSecret, err := db.EncryptCredential(config.DestClientSecret)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to encrypt destination client secret: %v", err)
			}
			provider.EncryptedClientSecret = encryptedClientSecret
		}

		// Save the new provider
		if err := tx.Create(provider).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create destination provider: %v", err)
		}

		// Update the config to reference the new provider
		config.SetDestinationProvider(provider)
	}

	// Save the updated config
	if err := tx.Save(config).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update config with provider references: %v", err)
	}

	return tx.Commit().Error
}

// EncryptCredential encrypts a sensitive credential value
func (db *DB) EncryptCredential(value string) (string, error) {
	// Create a credential encryptor
	encryptor, err := encryption.GetGlobalCredentialEncryptor()
	if err != nil {
		return "", fmt.Errorf("failed to get credential encryptor: %w", err)
	}

	// Encrypt the value using the generic credential type
	encrypted, err := encryptor.Encrypt(value, encryption.TypeGeneric)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt credential: %w", err)
	}

	return encrypted, nil
}

// DecryptCredential decrypts a sensitive credential value
func (db *DB) DecryptCredential(encryptedValue string) (string, error) {
	// Create a credential encryptor
	encryptor, err := encryption.GetGlobalCredentialEncryptor()
	if err != nil {
		return "", fmt.Errorf("failed to get credential encryptor: %w", err)
	}

	// Check if value is already encrypted with our prefix
	if !encryptor.IsEncrypted(encryptedValue) {
		// Handle legacy format (temporary backward compatibility)
		if strings.HasPrefix(encryptedValue, "encrypted_") {
			return strings.TrimPrefix(encryptedValue, "encrypted_"), nil
		}
		// Not encrypted, return as-is
		return encryptedValue, nil
	}

	// Decrypt the value
	decrypted, err := encryptor.Decrypt(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt credential: %w", err)
	}

	return decrypted, nil
}

// UpdateStorageProvider updates an existing storage provider
