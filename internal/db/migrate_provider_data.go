package db

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/starfleetcptn/gomft/internal/encryption"
)

// ProviderConfig represents a unique provider configuration extracted from TransferConfigs
type ProviderConfig struct {
	// Common identification fields
	SourceOrDest string              // "source" or "destination"
	Type         StorageProviderType // The provider type

	// All possible provider fields
	Host            string
	Port            int
	Username        string
	Password        string
	KeyFile         string
	Bucket          string
	Region          string
	AccessKey       string
	SecretKey       string
	Endpoint        string
	Share           string
	Domain          string
	PassiveMode     *bool
	ClientID        string
	ClientSecret    string
	DriveID         string
	TeamDrive       string
	ReadOnly        *bool
	StartYear       int
	IncludeArchived *bool
	UseBuiltinAuth  *bool

	// Status
	Authenticated *bool

	// References
	ConfigIDs []uint // IDs of TransferConfigs using this provider config
	CreatedBy uint   // User ID who created the config

	// For mapping to created provider
	NewProviderID uint // ID of the created StorageProvider (used during migration)
}

// GetUniqueKey returns a string key that uniquely identifies this provider configuration
// This is used for deduplication
func (pc *ProviderConfig) GetUniqueKey() string {
	// Create a composite key based on the most important identifying fields
	// The combination of fields depends on the provider type
	switch pc.Type {
	case ProviderTypeSFTP, ProviderTypeHetzner, ProviderTypeFTP:
		return fmt.Sprintf("%s:%s:%d:%s:%s",
			pc.Type, pc.Host, pc.Port, pc.Username, pc.KeyFile)
	case ProviderTypeS3:
		return fmt.Sprintf("%s:%s:%s:%s",
			pc.Type, pc.Endpoint, pc.Region, pc.AccessKey)
	case ProviderTypeSMB:
		return fmt.Sprintf("%s:%s:%s:%s",
			pc.Type, pc.Host, pc.Share, pc.Username)
	case ProviderTypeOneDrive, ProviderTypeGoogleDrive, ProviderTypeGooglePhoto:
		return fmt.Sprintf("%s:%s:%s",
			pc.Type, pc.ClientID, pc.DriveID)
	case ProviderTypeLocal:
		return fmt.Sprintf("%s:%d", pc.Type, pc.CreatedBy)
	default:
		// Fallback for unknown types
		return fmt.Sprintf("%s:%s:%d:%s",
			pc.Type, pc.Host, pc.Port, pc.Username)
	}
}

// GenerateName generates a meaningful name for the provider
func (pc *ProviderConfig) GenerateName(configName string) string {
	if configName == "" {
		configName = "Unnamed Config"
	}

	basePrefix := ""
	if pc.SourceOrDest == "source" {
		basePrefix = "Source -"
	} else {
		basePrefix = "Destination -"
	}

	// Include identifiable information based on provider type
	switch pc.Type {
	case ProviderTypeSFTP, ProviderTypeHetzner, ProviderTypeFTP:
		return fmt.Sprintf("%s %s %s (%s@%s)", configName, basePrefix, pc.Type, pc.Username, pc.Host)
	case ProviderTypeS3:
		return fmt.Sprintf("%s %s %s (%s - %s)", configName, basePrefix, pc.Type, pc.Region, pc.Bucket)
	case ProviderTypeSMB:
		return fmt.Sprintf("%s %s %s (%s on %s)", configName, basePrefix, pc.Type, pc.Share, pc.Host)
	case ProviderTypeOneDrive:
		return fmt.Sprintf("%s %s OneDrive", configName, basePrefix)
	case ProviderTypeGoogleDrive:
		return fmt.Sprintf("%s %s Google Drive", configName, basePrefix)
	case ProviderTypeGooglePhoto:
		return fmt.Sprintf("%s %s Google Photos", configName, basePrefix)
	case ProviderTypeLocal:
		return fmt.Sprintf("%s %s Local", configName, basePrefix)
	default:
		return fmt.Sprintf("%s %s %s", configName, basePrefix, pc.Type)
	}
}

// MigrationStats holds statistics about the migration process
type MigrationStats struct {
	TotalConfigs               int
	UniqueSourceProviders      int
	UniqueDestinationProviders int
	NewProvidersCreated        int
	ConfigsUpdated             int
	Errors                     []string
	StartTime                  time.Time
	EndTime                    time.Time
}

// MigrationBackup holds backup data for rollback in case of migration failure
type MigrationBackup struct {
	Configs          []TransferConfig
	ProvidersCreated []uint
}

// MigrateProviderDataOptions contains options for the migration process
type MigrateProviderDataOptions struct {
	DryRun         bool   // If true, perform a simulation without actually modifying data
	ValidationOnly bool   // If true, only perform validation without migration
	Force          bool   // If true, ignore validation errors and proceed with migration
	BackupDir      string // Directory to store backups in
}

// ExtractUniqueProviderConfigs extracts all unique provider configurations from existing TransferConfig records
// It returns a map of provider keys to ProviderConfig objects and any error encountered
func (db *DB) ExtractUniqueProviderConfigs() (map[string]*ProviderConfig, error) {
	log.Println("Starting extraction of unique provider configurations...")

	// Get all transfer configs
	var configs []TransferConfig
	if err := db.Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve transfer configs: %v", err)
	}

	log.Printf("Found %d transfer configs", len(configs))

	// Map to store unique provider configurations
	uniqueProviders := make(map[string]*ProviderConfig)

	// Process each transfer config
	for _, config := range configs {
		// Skip if already using provider references
		if config.IsUsingProviderReferences() {
			log.Printf("Config ID %d already using provider references, skipping", config.ID)
			continue
		}

		// Process source provider if not already using a reference
		if !config.IsUsingSourceProviderReference() && config.SourceType != "" {
			sourceConfig := extractSourceProviderConfig(&config)
			key := sourceConfig.GetUniqueKey()

			if existing, exists := uniqueProviders[key]; exists {
				// Add this config ID to the existing provider's references
				existing.ConfigIDs = append(existing.ConfigIDs, config.ID)
				log.Printf("Added Config ID %d to existing source provider key %s", config.ID, key)
			} else {
				// Add this as a new unique provider
				uniqueProviders[key] = sourceConfig
				log.Printf("Added new unique source provider with key %s", key)
			}
		}

		// Process destination provider if not already using a reference
		if !config.IsUsingDestinationProviderReference() && config.DestinationType != "" {
			destConfig := extractDestinationProviderConfig(&config)
			key := destConfig.GetUniqueKey()

			if existing, exists := uniqueProviders[key]; exists {
				// Add this config ID to the existing provider's references
				existing.ConfigIDs = append(existing.ConfigIDs, config.ID)
				log.Printf("Added Config ID %d to existing destination provider key %s", config.ID, key)
			} else {
				// Add this as a new unique provider
				uniqueProviders[key] = destConfig
				log.Printf("Added new unique destination provider with key %s", key)
			}
		}
	}

	// Count the number of source and destination providers
	sourceCount := 0
	destCount := 0
	for _, provider := range uniqueProviders {
		if provider.SourceOrDest == "source" {
			sourceCount++
		} else {
			destCount++
		}
	}

	log.Printf("Extraction complete. Found %d unique provider configurations (%d source, %d destination)",
		len(uniqueProviders), sourceCount, destCount)

	return uniqueProviders, nil
}

// extractSourceProviderConfig extracts source provider details from a TransferConfig
func extractSourceProviderConfig(config *TransferConfig) *ProviderConfig {
	sourceConfig := &ProviderConfig{
		SourceOrDest: "source",
		Type:         StorageProviderType(config.SourceType),
		CreatedBy:    config.CreatedBy,
		ConfigIDs:    []uint{config.ID},

		// Copy all relevant source fields
		Host:           config.SourceHost,
		Port:           config.SourcePort,
		Username:       config.SourceUser,
		Password:       config.SourcePassword,
		KeyFile:        config.SourceKeyFile,
		Bucket:         config.SourceBucket,
		Region:         config.SourceRegion,
		AccessKey:      config.SourceAccessKey,
		SecretKey:      config.SourceSecretKey,
		Endpoint:       config.SourceEndpoint,
		Share:          config.SourceShare,
		Domain:         config.SourceDomain,
		PassiveMode:    config.SourcePassiveMode,
		ClientID:       config.SourceClientID,
		ClientSecret:   config.SourceClientSecret,
		DriveID:        config.SourceDriveID,
		TeamDrive:      config.SourceTeamDrive,
		UseBuiltinAuth: config.UseBuiltinAuthSource,
	}

	// Handle boolean pointers
	if config.SourceReadOnly != nil {
		sourceConfig.ReadOnly = config.SourceReadOnly
	}
	if config.SourceIncludeArchived != nil {
		sourceConfig.IncludeArchived = config.SourceIncludeArchived
	}

	// Special handling for OAuth authentication status
	if config.SourceType == "drive" || config.SourceType == "gphotos" {
		authenticated := config.GetGoogleAuthenticated()
		sourceConfig.Authenticated = &authenticated
	}

	sourceConfig.StartYear = config.SourceStartYear

	return sourceConfig
}

// extractDestinationProviderConfig extracts destination provider details from a TransferConfig
func extractDestinationProviderConfig(config *TransferConfig) *ProviderConfig {
	destConfig := &ProviderConfig{
		SourceOrDest: "destination",
		Type:         StorageProviderType(config.DestinationType),
		CreatedBy:    config.CreatedBy,
		ConfigIDs:    []uint{config.ID},

		// Copy all relevant destination fields
		Host:           config.DestHost,
		Port:           config.DestPort,
		Username:       config.DestUser,
		Password:       config.DestPassword,
		KeyFile:        config.DestKeyFile,
		Bucket:         config.DestBucket,
		Region:         config.DestRegion,
		AccessKey:      config.DestAccessKey,
		SecretKey:      config.DestSecretKey,
		Endpoint:       config.DestEndpoint,
		Share:          config.DestShare,
		Domain:         config.DestDomain,
		PassiveMode:    config.DestPassiveMode,
		ClientID:       config.DestClientID,
		ClientSecret:   config.DestClientSecret,
		DriveID:        config.DestDriveID,
		TeamDrive:      config.DestTeamDrive,
		UseBuiltinAuth: config.UseBuiltinAuthDest,
	}

	// Handle boolean pointers
	if config.DestReadOnly != nil {
		destConfig.ReadOnly = config.DestReadOnly
	}
	if config.DestIncludeArchived != nil {
		destConfig.IncludeArchived = config.DestIncludeArchived
	}

	// Special handling for OAuth authentication status
	if config.DestinationType == "drive" || config.DestinationType == "gphotos" {
		authenticated := config.GetGoogleAuthenticated()
		destConfig.Authenticated = &authenticated
	}

	destConfig.StartYear = config.DestStartYear

	return destConfig
}

// CreateStorageProviderRecords creates new StorageProvider records from unique provider configurations
// It returns a map of provider keys to new StorageProvider IDs and any error encountered
func (db *DB) CreateStorageProviderRecords(uniqueConfigs map[string]*ProviderConfig) (map[string]uint, error) {
	log.Println("Starting creation of StorageProvider records...")

	// Get the credential encryptor
	credentialEncryptor, err := encryption.GetGlobalCredentialEncryptor()
	if err != nil {
		return nil, fmt.Errorf("failed to get credential encryptor: %v", err)
	}

	// Map to store provider keys to their IDs
	providerIDMap := make(map[string]uint)

	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	// Create a function to handle rollback in case of error
	rollback := func(err error) (map[string]uint, error) {
		tx.Rollback()
		return nil, err
	}

	// Process each unique provider config
	for key, providerConfig := range uniqueConfigs {
		log.Printf("Creating StorageProvider for key %s...", key)

		// Get the first config ID for naming
		var configName string
		if len(providerConfig.ConfigIDs) > 0 {
			firstConfigID := providerConfig.ConfigIDs[0]
			var config TransferConfig
			if err := tx.First(&config, firstConfigID).Error; err == nil {
				configName = config.Name
			}
		}

		// Create new StorageProvider record
		provider := &StorageProvider{
			Name:            providerConfig.GenerateName(configName),
			Type:            providerConfig.Type,
			Host:            providerConfig.Host,
			Port:            providerConfig.Port,
			Username:        providerConfig.Username,
			KeyFile:         providerConfig.KeyFile,
			Bucket:          providerConfig.Bucket,
			Region:          providerConfig.Region,
			AccessKey:       providerConfig.AccessKey,
			Endpoint:        providerConfig.Endpoint,
			Share:           providerConfig.Share,
			Domain:          providerConfig.Domain,
			PassiveMode:     providerConfig.PassiveMode,
			ClientID:        providerConfig.ClientID,
			DriveID:         providerConfig.DriveID,
			TeamDrive:       providerConfig.TeamDrive,
			ReadOnly:        providerConfig.ReadOnly,
			StartYear:       providerConfig.StartYear,
			IncludeArchived: providerConfig.IncludeArchived,
			UseBuiltinAuth:  providerConfig.UseBuiltinAuth,
			Authenticated:   providerConfig.Authenticated,
			CreatedBy:       providerConfig.CreatedBy,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		// Encrypt sensitive fields
		if providerConfig.Password != "" {
			encryptedPwd, err := credentialEncryptor.EncryptPassword(providerConfig.Password)
			if err != nil {
				return rollback(fmt.Errorf("failed to encrypt password: %v", err))
			}
			provider.EncryptedPassword = encryptedPwd
		}

		if providerConfig.SecretKey != "" {
			encryptedSecret, err := credentialEncryptor.EncryptSecretKey(providerConfig.SecretKey)
			if err != nil {
				return rollback(fmt.Errorf("failed to encrypt secret key: %v", err))
			}
			provider.EncryptedSecretKey = encryptedSecret
		}

		if providerConfig.ClientSecret != "" {
			encryptedClientSecret, err := credentialEncryptor.EncryptField(providerConfig.ClientSecret, encryption.TypeGeneric)
			if err != nil {
				return rollback(fmt.Errorf("failed to encrypt client secret: %v", err))
			}
			provider.EncryptedClientSecret = encryptedClientSecret
		}

		// Create the provider record
		if err := tx.Create(provider).Error; err != nil {
			sanitizedErrMsg := sanitizeErrorMessage(err.Error())
			return rollback(fmt.Errorf("failed to create provider record: %s", sanitizedErrMsg))
		}

		// Store the provider ID in the map
		providerIDMap[key] = provider.ID

		// Update the provider config with the new ID
		providerConfig.NewProviderID = provider.ID

		log.Printf("Created StorageProvider ID %d for key %s", provider.ID, key)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Successfully created %d StorageProvider records", len(providerIDMap))

	return providerIDMap, nil
}

// sanitizeErrorMessage removes any potential sensitive information from error messages
func sanitizeErrorMessage(errMsg string) string {
	// List of sensitive keywords to check for
	sensitiveKeywords := []string{
		"password", "secret", "token", "key", "credential", "auth",
	}

	// Check if the error message contains sensitive information
	lowercaseMsg := strings.ToLower(errMsg)
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowercaseMsg, keyword) {
			// If it contains sensitive info, return a generic message
			return "database error (details omitted for security)"
		}
	}

	return errMsg
}

// UpdateTransferConfigReferences updates TransferConfig records to reference the newly created StorageProvider entities
func (db *DB) UpdateTransferConfigReferences(uniqueConfigs map[string]*ProviderConfig) error {
	log.Println("Starting update of TransferConfig references...")

	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	// Create a function to handle rollback in case of error
	rollback := func(err error) error {
		tx.Rollback()
		return err
	}

	// Create a map of config IDs to their updates
	// This helps batch config updates by config ID
	configUpdates := make(map[uint]struct {
		SourceProviderID      *uint
		DestinationProviderID *uint
	})

	// Process each unique provider config
	for _, providerConfig := range uniqueConfigs {
		// Skip if no provider ID was assigned (shouldn't happen)
		if providerConfig.NewProviderID == 0 {
			log.Printf("Warning: Provider config %s has no ID assigned, skipping", providerConfig.GetUniqueKey())
			continue
		}

		// For each config ID that uses this provider
		for _, configID := range providerConfig.ConfigIDs {
			// Get or initialize the update record
			update, exists := configUpdates[configID]
			if !exists {
				update = struct {
					SourceProviderID      *uint
					DestinationProviderID *uint
				}{nil, nil}
			}

			// Update the appropriate provider ID
			if providerConfig.SourceOrDest == "source" {
				newID := providerConfig.NewProviderID
				update.SourceProviderID = &newID
			} else {
				newID := providerConfig.NewProviderID
				update.DestinationProviderID = &newID
			}

			// Store the update
			configUpdates[configID] = update
		}
	}

	// Apply the updates
	totalUpdated := 0
	for configID, update := range configUpdates {
		// Retrieve the config
		var config TransferConfig
		if err := tx.First(&config, configID).Error; err != nil {
			return rollback(fmt.Errorf("failed to retrieve config ID %d: %v", configID, err))
		}

		// Update source provider reference if needed
		if update.SourceProviderID != nil {
			config.SourceProviderID = update.SourceProviderID
		}

		// Update destination provider reference if needed
		if update.DestinationProviderID != nil {
			config.DestinationProviderID = update.DestinationProviderID
		}

		// Save the updated config
		if err := tx.Save(&config).Error; err != nil {
			return rollback(fmt.Errorf("failed to update config ID %d: %v", configID, err))
		}

		log.Printf("Updated TransferConfig ID %d with provider references", configID)
		totalUpdated++
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Successfully updated %d TransferConfig records with provider references", totalUpdated)

	return nil
}

// ValidationResult represents the result of a migration validation
type ValidationResult struct {
	Success           bool
	TotalConfigs      int
	ValidConfigs      int
	InvalidConfigs    int
	MissingProviders  int
	ValidationErrors  []string
	ConfigsWithErrors []uint
}

// ValidateMigrationIntegrity validates the integrity of the migration
func (db *DB) ValidateMigrationIntegrity() (*ValidationResult, error) {
	log.Println("Starting validation of migration integrity...")

	result := &ValidationResult{
		Success:           true,
		ValidationErrors:  []string{},
		ConfigsWithErrors: []uint{},
	}

	// Get all transfer configs
	var configs []TransferConfig
	if err := db.Preload("SourceProvider").Preload("DestinationProvider").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve transfer configs: %v", err)
	}

	result.TotalConfigs = len(configs)
	log.Printf("Found %d transfer configs for validation", result.TotalConfigs)

	// Get the credential encryptor for testing decryption
	credentialEncryptor, err := encryption.GetGlobalCredentialEncryptor()
	if err != nil {
		return nil, fmt.Errorf("failed to get credential encryptor: %v", err)
	}

	// Validate each config
	for _, config := range configs {
		configValid := true

		// Check if this config should be using provider references
		shouldUseProviders := !strings.HasPrefix(config.SourceType, "local") || !strings.HasPrefix(config.DestinationType, "local")

		// If it should be using provider references but isn't, mark as invalid
		if shouldUseProviders && !config.IsUsingProviderReferences() {
			result.ValidationErrors = append(result.ValidationErrors,
				fmt.Sprintf("Config ID %d is not using provider references", config.ID))
			result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
			configValid = false
		}

		// Check source provider reference if needed
		if !strings.HasPrefix(config.SourceType, "local") && !config.IsUsingSourceProviderReference() {
			result.ValidationErrors = append(result.ValidationErrors,
				fmt.Sprintf("Config ID %d is missing source provider reference", config.ID))
			result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
			configValid = false
		}

		// Check destination provider reference if needed
		if !strings.HasPrefix(config.DestinationType, "local") && !config.IsUsingDestinationProviderReference() {
			result.ValidationErrors = append(result.ValidationErrors,
				fmt.Sprintf("Config ID %d is missing destination provider reference", config.ID))
			result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
			configValid = false
		}

		// If using source provider reference, validate the provider
		if config.IsUsingSourceProviderReference() {
			if config.SourceProvider == nil {
				result.ValidationErrors = append(result.ValidationErrors,
					fmt.Sprintf("Config ID %d has source provider reference but provider is nil", config.ID))
				result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
				configValid = false
				result.MissingProviders++
			} else if config.SourceProvider.Type != StorageProviderType(config.SourceType) {
				result.ValidationErrors = append(result.ValidationErrors,
					fmt.Sprintf("Config ID %d source provider type mismatch: config=%s, provider=%s",
						config.ID, config.SourceType, config.SourceProvider.Type))
				result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
				configValid = false
			}
		}

		// If using destination provider reference, validate the provider
		if config.IsUsingDestinationProviderReference() {
			if config.DestinationProvider == nil {
				result.ValidationErrors = append(result.ValidationErrors,
					fmt.Sprintf("Config ID %d has destination provider reference but provider is nil", config.ID))
				result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
				configValid = false
				result.MissingProviders++
			} else if config.DestinationProvider.Type != StorageProviderType(config.DestinationType) {
				result.ValidationErrors = append(result.ValidationErrors,
					fmt.Sprintf("Config ID %d destination provider type mismatch: config=%s, provider=%s",
						config.ID, config.DestinationType, config.DestinationProvider.Type))
				result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
				configValid = false
			}
		}

		// Verify that source credentials can be retrieved
		if !strings.HasPrefix(config.SourceType, "local") {
			sourceCreds, err := config.GetSourceCredentials(db)
			if err != nil {
				result.ValidationErrors = append(result.ValidationErrors,
					fmt.Sprintf("Config ID %d failed to get source credentials: %v", config.ID, err))
				result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
				configValid = false
			} else {
				// Check if credentials are properly encrypted
				if encPwd, ok := sourceCreds["encrypted_password"].(string); ok && encPwd != "" {
					if !credentialEncryptor.IsEncrypted(encPwd) {
						result.ValidationErrors = append(result.ValidationErrors,
							fmt.Sprintf("Config ID %d source password is not properly encrypted", config.ID))
						result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
						configValid = false
					}
				}

				if encSecret, ok := sourceCreds["encrypted_secret_key"].(string); ok && encSecret != "" {
					if !credentialEncryptor.IsEncrypted(encSecret) {
						result.ValidationErrors = append(result.ValidationErrors,
							fmt.Sprintf("Config ID %d source secret key is not properly encrypted", config.ID))
						result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
						configValid = false
					}
				}
			}
		}

		// Verify that destination credentials can be retrieved
		if !strings.HasPrefix(config.DestinationType, "local") {
			destCreds, err := config.GetDestinationCredentials(db)
			if err != nil {
				result.ValidationErrors = append(result.ValidationErrors,
					fmt.Sprintf("Config ID %d failed to get destination credentials: %v", config.ID, err))
				result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
				configValid = false
			} else {
				// Check if credentials are properly encrypted
				if encPwd, ok := destCreds["encrypted_password"].(string); ok && encPwd != "" {
					if !credentialEncryptor.IsEncrypted(encPwd) {
						result.ValidationErrors = append(result.ValidationErrors,
							fmt.Sprintf("Config ID %d destination password is not properly encrypted", config.ID))
						result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
						configValid = false
					}
				}

				if encSecret, ok := destCreds["encrypted_secret_key"].(string); ok && encSecret != "" {
					if !credentialEncryptor.IsEncrypted(encSecret) {
						result.ValidationErrors = append(result.ValidationErrors,
							fmt.Sprintf("Config ID %d destination secret key is not properly encrypted", config.ID))
						result.ConfigsWithErrors = append(result.ConfigsWithErrors, config.ID)
						configValid = false
					}
				}
			}
		}

		if configValid {
			result.ValidConfigs++
		} else {
			result.InvalidConfigs++
			result.Success = false
		}
	}

	// Log validation summary
	if result.Success {
		log.Printf("Validation successful. All %d configs are valid.", result.ValidConfigs)
	} else {
		log.Printf("Validation failed. %d valid configs, %d invalid configs, %d missing providers.",
			result.ValidConfigs, result.InvalidConfigs, result.MissingProviders)
	}

	return result, nil
}

// MigrateProviderData is the main function that performs the complete migration process
func (db *DB) MigrateProviderData(options MigrateProviderDataOptions) (*MigrationStats, error) {
	// Initialize migration stats
	stats := &MigrationStats{
		StartTime: time.Now(),
		Errors:    []string{},
	}

	log.Println("Starting provider data migration...")

	// Create backup if not in dry run mode
	var backup *MigrationBackup
	var err error
	if !options.DryRun {
		backup, err = db.createMigrationBackup(options.BackupDir)
		if err != nil {
			return stats, fmt.Errorf("failed to create backup: %v", err)
		}
		log.Println("Created migration backup")
	}

	// Extract unique provider configs
	uniqueConfigs, err := db.ExtractUniqueProviderConfigs()
	if err != nil {
		return stats, fmt.Errorf("failed to extract unique provider configurations: %v", err)
	}

	// Count the number of source and destination providers
	sourceCount := 0
	destCount := 0
	for _, provider := range uniqueConfigs {
		if provider.SourceOrDest == "source" {
			sourceCount++
		} else {
			destCount++
		}
	}

	stats.TotalConfigs = len(uniqueConfigs)
	stats.UniqueSourceProviders = sourceCount
	stats.UniqueDestinationProviders = destCount

	// Return if validation only mode
	if options.ValidationOnly {
		log.Println("Validation-only mode: Migration stopped after extraction")
		stats.EndTime = time.Now()
		return stats, nil
	}

	// Return if dry run mode
	if options.DryRun {
		log.Println("Dry run mode: Migration stopped after extraction")
		stats.EndTime = time.Now()
		return stats, nil
	}

	// Create provider records
	providerIDMap, err := db.CreateStorageProviderRecords(uniqueConfigs)
	if err != nil {
		// Attempt rollback
		if rollbackErr := db.rollbackMigration(backup); rollbackErr != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("failed to rollback after provider creation error: %v", rollbackErr))
		}
		return stats, fmt.Errorf("failed to create provider records: %v", err)
	}

	stats.NewProvidersCreated = len(providerIDMap)

	// Update config references
	if err := db.UpdateTransferConfigReferences(uniqueConfigs); err != nil {
		// Attempt rollback
		if rollbackErr := db.rollbackMigration(backup); rollbackErr != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("failed to rollback after reference update error: %v", rollbackErr))
		}
		return stats, fmt.Errorf("failed to update config references: %v", err)
	}

	// Validate the migration
	validationResult, err := db.ValidateMigrationIntegrity()
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("validation error: %v", err))
		// Don't rollback here since the migration might be fine even if validation had errors
	}

	if validationResult != nil {
		stats.ConfigsUpdated = validationResult.ValidConfigs

		// If validation failed and not in force mode, rollback
		if !validationResult.Success && !options.Force {
			log.Println("Validation failed and not in force mode, rolling back...")
			if rollbackErr := db.rollbackMigration(backup); rollbackErr != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("failed to rollback after validation failure: %v", rollbackErr))
			}
			stats.Errors = append(stats.Errors, validationResult.ValidationErrors...)
			return stats, fmt.Errorf("migration validation failed")
		}

		// If validation failed but in force mode, log warnings
		if !validationResult.Success && options.Force {
			log.Println("Validation failed but running in force mode, proceeding anyway...")
			stats.Errors = append(stats.Errors, "Migration had validation errors but continued due to force mode")
			stats.Errors = append(stats.Errors, validationResult.ValidationErrors...)
		}
	}

	stats.EndTime = time.Now()
	log.Printf("Migration completed in %v", stats.EndTime.Sub(stats.StartTime))

	return stats, nil
}

// createMigrationBackup creates a backup of the current state for rollback
func (db *DB) createMigrationBackup(backupDir string) (*MigrationBackup, error) {
	backup := &MigrationBackup{
		Configs:          []TransferConfig{},
		ProvidersCreated: []uint{},
	}

	// Get all transfer configs
	if err := db.Find(&backup.Configs).Error; err != nil {
		return nil, fmt.Errorf("failed to backup transfer configs: %v", err)
	}

	log.Printf("Backed up %d transfer config records", len(backup.Configs))

	return backup, nil
}

// rollbackMigration restores the system to its pre-migration state
func (db *DB) rollbackMigration(backup *MigrationBackup) error {
	if backup == nil {
		return fmt.Errorf("cannot rollback: no backup provided")
	}

	log.Println("Starting migration rollback...")

	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start rollback transaction: %v", tx.Error)
	}

	// First, delete any provider records created during the migration
	if len(backup.ProvidersCreated) > 0 {
		if err := tx.Where("id IN ?", backup.ProvidersCreated).Delete(&StorageProvider{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete created providers: %v", err)
		}
		log.Printf("Deleted %d provider records created during migration", len(backup.ProvidersCreated))
	}

	// Then restore original config records
	for _, config := range backup.Configs {
		if err := tx.Save(&config).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to restore config ID %d: %v", config.ID, err)
		}
	}

	log.Printf("Restored %d transfer config records", len(backup.Configs))

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %v", err)
	}

	log.Println("Rollback completed successfully")

	return nil
}

// FormatMigrationReport generates a human-readable report of the migration results
func FormatMigrationReport(stats *MigrationStats) string {
	if stats == nil {
		return "No migration statistics available"
	}

	duration := stats.EndTime.Sub(stats.StartTime)

	report := strings.Builder{}
	report.WriteString("=== Provider Data Migration Report ===\n\n")
	report.WriteString(fmt.Sprintf("Started:             %s\n", stats.StartTime.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Completed:           %s\n", stats.EndTime.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Duration:            %s\n", duration))
	report.WriteString(fmt.Sprintf("Total Configs:       %d\n", stats.TotalConfigs))
	report.WriteString(fmt.Sprintf("Source Providers:    %d\n", stats.UniqueSourceProviders))
	report.WriteString(fmt.Sprintf("Destination Providers: %d\n", stats.UniqueDestinationProviders))
	report.WriteString(fmt.Sprintf("Providers Created:   %d\n", stats.NewProvidersCreated))
	report.WriteString(fmt.Sprintf("Configs Updated:     %d\n", stats.ConfigsUpdated))

	if len(stats.Errors) > 0 {
		report.WriteString("\nErrors/Warnings:\n")
		for i, err := range stats.Errors {
			report.WriteString(fmt.Sprintf("%d. %s\n", i+1, err))
		}
	} else {
		report.WriteString("\nNo errors or warnings reported.\n")
	}

	report.WriteString("\n=== End of Report ===\n")

	return report.String()
}
