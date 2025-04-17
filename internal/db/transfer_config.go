package db

import (
	"fmt"
	"time"
)

// TransferConfig holds the configuration for a data transfer operation
type TransferConfig struct {
	ID             uint   `gorm:"primarykey"`
	Name           string `gorm:"not null" form:"name"`
	SourceType     string `gorm:"not null" form:"source_type"`
	SourcePath     string `gorm:"not null" form:"source_path"`
	SourceHost     string `form:"source_host"`
	SourcePort     int    `gorm:"default:22" form:"source_port"`
	SourceUser     string `form:"source_user"`
	SourcePassword string `form:"source_password" gorm:"-"` // Not stored in DB, only used for form
	SourceKeyFile  string `form:"source_key_file"`
	// Source provider reference
	SourceProviderID *uint            `form:"source_provider_id"`
	SourceProvider   *StorageProvider `gorm:"foreignKey:SourceProviderID" json:"-"`
	// S3 source fields
	SourceBucket    string `form:"source_bucket"`
	SourceRegion    string `form:"source_region"`
	SourceAccessKey string `form:"source_access_key"`
	SourceSecretKey string `form:"source_secret_key" gorm:"-"` // Not stored in DB, only used for form
	SourceEndpoint  string `form:"source_endpoint"`
	// SMB source fields
	SourceShare  string `form:"source_share"`
	SourceDomain string `form:"source_domain"`
	// FTP source fields
	SourcePassiveMode *bool `gorm:"default:true" form:"source_passive_mode"` // Already a pointer, no change needed here
	// OneDrive and Google Drive source fields
	SourceClientID     string `form:"source_client_id"`
	SourceClientSecret string `form:"source_client_secret" gorm:"-"` // Not stored in DB, only used for form
	SourceDriveID      string `form:"source_drive_id"`               // For OneDrive
	SourceTeamDrive    string `form:"source_team_drive"`             // For Google Drive
	// Google Photos source fields
	SourceReadOnly        *bool `form:"source_read_only"`        // For Google Photos
	SourceStartYear       int   `form:"source_start_year"`       // For Google Photos
	SourceIncludeArchived *bool `form:"source_include_archived"` // For Google Photos
	// General fields
	FilePattern     string `gorm:"default:'*'" form:"file_pattern"`
	OutputPattern   string `form:"output_pattern"` // Pattern for output filenames with date variables
	DestinationType string `gorm:"not null" form:"destination_type"`
	DestinationPath string `gorm:"not null" form:"destination_path"`
	DestHost        string `form:"dest_host"`
	DestPort        int    `gorm:"default:22" form:"dest_port"`
	DestUser        string `form:"dest_user"`
	DestPassword    string `form:"dest_password" gorm:"-"` // Not stored in DB, only used for form
	DestKeyFile     string `form:"dest_key_file"`
	// Destination provider reference
	DestinationProviderID *uint            `form:"destination_provider_id"`
	DestinationProvider   *StorageProvider `gorm:"foreignKey:DestinationProviderID" json:"-"`
	// S3 destination fields
	DestBucket    string `form:"dest_bucket"`
	DestRegion    string `form:"dest_region"`
	DestAccessKey string `form:"dest_access_key"`
	DestSecretKey string `form:"dest_secret_key" gorm:"-"` // Not stored in DB, only used for form
	DestEndpoint  string `form:"dest_endpoint"`
	// SMB destination fields
	DestShare  string `form:"dest_share"`
	DestDomain string `form:"dest_domain"`
	// FTP destination fields
	DestPassiveMode *bool `gorm:"default:true" form:"dest_passive_mode"`
	// OneDrive and Google Drive destination fields
	DestClientID     string `form:"dest_client_id"`
	DestClientSecret string `form:"dest_client_secret" gorm:"-"` // Not stored in DB, only used for form
	DestDriveID      string `form:"dest_drive_id"`               // For OneDrive
	DestTeamDrive    string `form:"dest_team_drive"`             // For Google Drive
	// Google Photos destination fields
	DestReadOnly        *bool `form:"dest_read_only"`        // For Google Photos
	DestStartYear       int   `form:"dest_start_year"`       // For Google Photos
	DestIncludeArchived *bool `form:"dest_include_archived"` // For Google Photos
	// Security fields
	UseBuiltinAuthSource     *bool `form:"use_builtin_auth_source"` // For Google and other OAuth services
	UseBuiltinAuthDest       *bool `form:"use_builtin_auth_dest"`   // For Google and other OAuth services
	GoogleDriveAuthenticated *bool // Whether Google Drive auth is completed
	// General fields
	ArchivePath    string `form:"archive_path"`
	ArchiveEnabled *bool  `gorm:"default:false" form:"archive_enabled"`
	RcloneFlags    string `form:"rclone_flags"`
	// Rclone command fields
	CommandID              uint   `gorm:"default:1" form:"command_id"` // Default to 'copy' command ID (1)
	CommandFlags           string `form:"command_flags"`               // JSON string of selected flags
	CommandFlagValues      string `form:"command_flag_values"`         // JSON string of flag values by ID
	DeleteAfterTransfer    *bool  `gorm:"default:false" form:"delete_after_transfer"`
	SkipProcessedFiles     *bool  `gorm:"default:true" form:"skip_processed_files"`
	MaxConcurrentTransfers int    `gorm:"default:4" form:"max_concurrent_transfers"` // Number of concurrent file transfers
	CreatedBy              uint
	User                   User `gorm:"foreignkey:CreatedBy"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// --- TransferConfig Helper Methods ---

// GetSourcePassiveMode returns the value of SourcePassiveMode with a default if nil
func (tc *TransferConfig) GetSourcePassiveMode() bool {
	if tc.SourcePassiveMode == nil {
		return true // Default to true if not set
	}
	return *tc.SourcePassiveMode
}

// SetSourcePassiveMode sets the SourcePassiveMode field
func (tc *TransferConfig) SetSourcePassiveMode(value bool) {
	tc.SourcePassiveMode = &value
}

// GetDestPassiveMode returns the value of DestPassiveMode with a default if nil
func (tc *TransferConfig) GetDestPassiveMode() bool {
	if tc.DestPassiveMode == nil {
		return true // Default to true if not set
	}
	return *tc.DestPassiveMode
}

// SetDestPassiveMode sets the DestPassiveMode field
func (tc *TransferConfig) SetDestPassiveMode(value bool) {
	tc.DestPassiveMode = &value
}

// GetGoogleDriveAuthenticated returns whether the transfer config has been authenticated with Google Drive
func (tc *TransferConfig) GetGoogleDriveAuthenticated() bool {
	return tc.GoogleDriveAuthenticated != nil && *tc.GoogleDriveAuthenticated
}

// SetGoogleDriveAuthenticated sets the Google Drive authentication status
func (tc *TransferConfig) SetGoogleDriveAuthenticated(value bool) {
	tc.GoogleDriveAuthenticated = &value
}

// GetGoogleAuthenticated is an alias for GetGoogleDriveAuthenticated for better semantics when working with Google Photos
func (tc *TransferConfig) GetGoogleAuthenticated() bool {
	return tc.GetGoogleDriveAuthenticated()
}

// SetGoogleAuthenticated is an alias for SetGoogleDriveAuthenticated for better semantics when working with Google Photos
func (tc *TransferConfig) SetGoogleAuthenticated(value bool) {
	tc.SetGoogleDriveAuthenticated(value)
}

// GetArchiveEnabled returns the value of ArchiveEnabled with a default if nil
func (tc *TransferConfig) GetArchiveEnabled() bool {
	if tc.ArchiveEnabled == nil {
		return false // Default to false if not set
	}
	return *tc.ArchiveEnabled
}

// SetArchiveEnabled sets the ArchiveEnabled field
func (tc *TransferConfig) SetArchiveEnabled(value bool) {
	tc.ArchiveEnabled = &value
}

// GetDeleteAfterTransfer returns the value of DeleteAfterTransfer with a default if nil
func (tc *TransferConfig) GetDeleteAfterTransfer() bool {
	if tc.DeleteAfterTransfer == nil {
		return false // Default to false if not set
	}
	return *tc.DeleteAfterTransfer
}

// SetDeleteAfterTransfer sets the DeleteAfterTransfer field
func (tc *TransferConfig) SetDeleteAfterTransfer(value bool) {
	tc.DeleteAfterTransfer = &value
}

// GetSkipProcessedFiles returns the value of SkipProcessedFiles with a default if nil
func (tc *TransferConfig) GetSkipProcessedFiles() bool {
	if tc.SkipProcessedFiles == nil {
		return true // Default to true if not set
	}
	return *tc.SkipProcessedFiles
}

// SetSkipProcessedFiles sets the SkipProcessedFiles field
func (tc *TransferConfig) SetSkipProcessedFiles(value bool) {
	tc.SkipProcessedFiles = &value
}

// GetUseBuiltinAuthSource returns the value of UseBuiltinAuthSource with a default if nil
func (tc *TransferConfig) GetUseBuiltinAuthSource() bool {
	if tc.UseBuiltinAuthSource == nil {
		return true // Default to true if not set
	}
	return *tc.UseBuiltinAuthSource
}

// SetUseBuiltinAuthSource sets the UseBuiltinAuthSource field
func (tc *TransferConfig) SetUseBuiltinAuthSource(value bool) {
	tc.UseBuiltinAuthSource = &value
}

// GetUseBuiltinAuthDest returns the value of UseBuiltinAuthDest with a default if nil
func (tc *TransferConfig) GetUseBuiltinAuthDest() bool {
	if tc.UseBuiltinAuthDest == nil {
		return true // Default to true if not set
	}
	return *tc.UseBuiltinAuthDest
}

// SetUseBuiltinAuthDest sets the UseBuiltinAuthDest field
func (tc *TransferConfig) SetUseBuiltinAuthDest(value bool) {
	tc.UseBuiltinAuthDest = &value
}

// --- Provider Reference Methods ---

// IsUsingSourceProviderReference returns true if this config is using a source provider reference
func (tc *TransferConfig) IsUsingSourceProviderReference() bool {
	return tc.SourceProviderID != nil && *tc.SourceProviderID > 0
}

// IsUsingDestinationProviderReference returns true if this config is using a destination provider reference
func (tc *TransferConfig) IsUsingDestinationProviderReference() bool {
	return tc.DestinationProviderID != nil && *tc.DestinationProviderID > 0
}

// IsUsingProviderReferences returns true if this config is using provider references for both source and destination
func (tc *TransferConfig) IsUsingProviderReferences() bool {
	return tc.IsUsingSourceProviderReference() && tc.IsUsingDestinationProviderReference()
}

// SetSourceProvider sets the source provider and ID fields
func (tc *TransferConfig) SetSourceProvider(provider *StorageProvider) {
	if provider == nil || provider.ID == 0 {
		tc.SourceProviderID = nil
		tc.SourceProvider = nil
		return
	}

	// Create a new uint pointer to avoid shared memory issues
	newID := provider.ID
	tc.SourceProviderID = &newID
	tc.SourceProvider = provider

	// Set the source type to match the provider type if not already set
	if provider.Type != "" {
		tc.SourceType = string(provider.Type)
	}
}

// SetDestinationProvider sets the destination provider and ID fields
func (tc *TransferConfig) SetDestinationProvider(provider *StorageProvider) {
	if provider == nil || provider.ID == 0 {
		tc.DestinationProviderID = nil
		tc.DestinationProvider = nil
		return
	}

	// Create a new uint pointer to avoid shared memory issues
	newID := provider.ID
	tc.DestinationProviderID = &newID
	tc.DestinationProvider = provider

	// Set the destination type to match the provider type if not already set
	if provider.Type != "" {
		tc.DestinationType = string(provider.Type)
	}
}

// EnsureProvidersLoaded ensures that both source and destination providers are loaded if references are used
func (tc *TransferConfig) EnsureProvidersLoaded(db interface{}) error {
	if db == nil {
		return fmt.Errorf("database interface is required to load providers")
	}

	// Try to load source provider if needed
	if tc.IsUsingSourceProviderReference() && tc.SourceProvider == nil {
		switch dbImpl := db.(type) {
		case *DB:
			provider, err := dbImpl.GetStorageProvider(*tc.SourceProviderID)
			if err != nil {
				return fmt.Errorf("failed to load source provider (ID %d): %w", *tc.SourceProviderID, err)
			}
			tc.SetSourceProvider(provider)
		default:
			return fmt.Errorf("invalid database interface for loading source provider")
		}
	}

	// Try to load destination provider if needed
	if tc.IsUsingDestinationProviderReference() && tc.DestinationProvider == nil {
		switch dbImpl := db.(type) {
		case *DB:
			provider, err := dbImpl.GetStorageProvider(*tc.DestinationProviderID)
			if err != nil {
				return fmt.Errorf("failed to load destination provider (ID %d): %w", *tc.DestinationProviderID, err)
			}
			tc.SetDestinationProvider(provider)
		default:
			return fmt.Errorf("invalid database interface for loading destination provider")
		}
	}

	return nil
}

// ValidateProviderConfiguration validates that the provider configuration is consistent
func (tc *TransferConfig) ValidateProviderConfiguration() error {
	// Validate source provider configuration
	if tc.IsUsingSourceProviderReference() {
		if tc.SourceProvider == nil {
			return fmt.Errorf("source provider reference set but provider is nil")
		}
		if tc.SourceProviderID == nil || *tc.SourceProviderID != tc.SourceProvider.ID {
			return fmt.Errorf("source provider ID mismatch")
		}
		if tc.SourceType != string(tc.SourceProvider.Type) {
			return fmt.Errorf("source type mismatch: config has %s but provider has %s", tc.SourceType, tc.SourceProvider.Type)
		}
	}

	// Validate destination provider configuration
	if tc.IsUsingDestinationProviderReference() {
		if tc.DestinationProvider == nil {
			return fmt.Errorf("destination provider reference set but provider is nil")
		}
		if tc.DestinationProviderID == nil || *tc.DestinationProviderID != tc.DestinationProvider.ID {
			return fmt.Errorf("destination provider ID mismatch")
		}
		if tc.DestinationType != string(tc.DestinationProvider.Type) {
			return fmt.Errorf("destination type mismatch: config has %s but provider has %s", tc.DestinationType, tc.DestinationProvider.Type)
		}
	}

	return nil
}

// GetSourceCredentials returns credential information for the source, either directly or from the provider
// If db is provided, it will try to load the provider from the database if needed
func (tc *TransferConfig) GetSourceCredentials(db interface{}) (map[string]interface{}, error) {
	creds := make(map[string]interface{})

	fmt.Printf("DEBUG GetSourceCreds Start: ProviderID=%v, HasProvider=%v\n",
		tc.SourceProviderID,
		tc.SourceProvider != nil)

	// If using provider reference and provider is loaded
	if tc.IsUsingSourceProviderReference() {
		// Try to load provider from database if we have a valid ID but no provider
		if tc.SourceProvider == nil && db != nil {
			// Try different types of DB interfaces to load the provider
			switch dbImpl := db.(type) {
			case *DB:
				provider, err := dbImpl.GetStorageProvider(*tc.SourceProviderID)
				if err != nil {
					return nil, fmt.Errorf("failed to load source provider (ID %d): %w", *tc.SourceProviderID, err)
				}
				tc.SourceProvider = provider
			case interface {
				GetStorageProvider(id uint) (*StorageProvider, error)
			}:
				provider, err := dbImpl.GetStorageProvider(*tc.SourceProviderID)
				if err != nil {
					return nil, fmt.Errorf("failed to load source provider (ID %d): %w", *tc.SourceProviderID, err)
				}
				tc.SourceProvider = provider
			default:
				return nil, fmt.Errorf("source provider not loaded and db interface cannot load providers")
			}
		}

		// If we still don't have a provider or it has no ID, return error
		if tc.SourceProvider == nil || tc.SourceProvider.ID == 0 {
			return nil, fmt.Errorf("failed to load valid source provider (ID %d)", *tc.SourceProviderID)
		}

		if tc.SourceProvider != nil {
			fmt.Printf("DEBUG Provider Details:\n"+
				"  ID: %v\n"+
				"  Type: %v\n"+
				"  Host: %v\n"+
				"  Port: %v\n"+
				"  Username: %v\n"+
				"  HasEncryptedPassword: %v\n"+
				"  HasKeyFile: %v\n"+
				"  HasSecretKey: %v\n"+
				"  HasClientSecret: %v\n"+
				"  HasRefreshToken: %v\n",
				tc.SourceProvider.ID,
				tc.SourceProvider.Type,
				tc.SourceProvider.Host,
				tc.SourceProvider.Port,
				tc.SourceProvider.Username,
				tc.SourceProvider.EncryptedPassword != "",
				tc.SourceProvider.KeyFile != "",
				tc.SourceProvider.EncryptedSecretKey != "",
				tc.SourceProvider.EncryptedClientSecret != "",
				tc.SourceProvider.EncryptedRefreshToken != "")
		}

		// Copy credentials from provider
		creds["type"] = tc.SourceProvider.Type
		creds["host"] = tc.SourceProvider.Host
		creds["port"] = tc.SourceProvider.Port
		creds["username"] = tc.SourceProvider.Username
		creds["encrypted_password"] = tc.SourceProvider.EncryptedPassword
		creds["key_file"] = tc.SourceProvider.KeyFile

		// Handle S3 fields
		creds["bucket"] = tc.SourceProvider.Bucket
		creds["region"] = tc.SourceProvider.Region
		creds["access_key"] = tc.SourceProvider.AccessKey
		creds["encrypted_secret_key"] = tc.SourceProvider.EncryptedSecretKey
		creds["endpoint"] = tc.SourceProvider.Endpoint

		// Handle SMB fields
		creds["share"] = tc.SourceProvider.Share
		creds["domain"] = tc.SourceProvider.Domain

		// Handle FTP fields
		if tc.SourceProvider.PassiveMode != nil {
			creds["passive_mode"] = *tc.SourceProvider.PassiveMode
		}

		// Handle OAuth fields
		creds["client_id"] = tc.SourceProvider.ClientID
		creds["encrypted_client_secret"] = tc.SourceProvider.EncryptedClientSecret
		creds["encrypted_refresh_token"] = tc.SourceProvider.EncryptedRefreshToken
		creds["drive_id"] = tc.SourceProvider.DriveID
		creds["team_drive"] = tc.SourceProvider.TeamDrive

		if tc.SourceProvider.ReadOnly != nil {
			creds["read_only"] = *tc.SourceProvider.ReadOnly
		}
		creds["start_year"] = tc.SourceProvider.StartYear
		if tc.SourceProvider.IncludeArchived != nil {
			creds["include_archived"] = *tc.SourceProvider.IncludeArchived
		}

		if tc.SourceProvider.UseBuiltinAuth != nil {
			creds["use_builtin_auth"] = *tc.SourceProvider.UseBuiltinAuth
		}

		if tc.SourceProvider.Authenticated != nil {
			creds["authenticated"] = *tc.SourceProvider.Authenticated
		}

		fmt.Printf("DEBUG Final Provider Creds:\n"+
			"  type: %v\n"+
			"  host: %v\n"+
			"  port: %v\n"+
			"  username: %v\n"+
			"  has_encrypted_password: %v\n"+
			"  has_key_file: %v\n"+
			"  has_encrypted_secret_key: %v\n"+
			"  has_encrypted_client_secret: %v\n",
			creds["type"],
			creds["host"],
			creds["port"],
			creds["username"],
			creds["encrypted_password"] != "",
			creds["key_file"] != "",
			creds["encrypted_secret_key"] != "",
			creds["encrypted_client_secret"] != "")

		return creds, nil
	}

	// Use legacy fields directly
	creds["type"] = tc.SourceType
	creds["host"] = tc.SourceHost
	creds["port"] = tc.SourcePort
	creds["username"] = tc.SourceUser
	creds["key_file"] = tc.SourceKeyFile

	// Handle S3 fields
	creds["bucket"] = tc.SourceBucket
	creds["region"] = tc.SourceRegion
	creds["access_key"] = tc.SourceAccessKey
	creds["endpoint"] = tc.SourceEndpoint

	// Handle SMB fields
	creds["share"] = tc.SourceShare
	creds["domain"] = tc.SourceDomain

	// Handle FTP fields
	if tc.SourcePassiveMode != nil {
		creds["passive_mode"] = *tc.SourcePassiveMode
	}

	// Handle OAuth fields
	creds["client_id"] = tc.SourceClientID
	creds["drive_id"] = tc.SourceDriveID
	creds["team_drive"] = tc.SourceTeamDrive

	if tc.SourceReadOnly != nil {
		creds["read_only"] = *tc.SourceReadOnly
	}
	creds["start_year"] = tc.SourceStartYear
	if tc.SourceIncludeArchived != nil {
		creds["include_archived"] = *tc.SourceIncludeArchived
	}

	if tc.UseBuiltinAuthSource != nil {
		creds["use_builtin_auth"] = *tc.UseBuiltinAuthSource
	}

	// Handle temporary form fields and their encrypted counterparts
	if tc.SourcePassword != "" {
		creds["password"] = tc.SourcePassword
	}
	if tc.SourceSecretKey != "" {
		creds["secret_key"] = tc.SourceSecretKey
	}
	if tc.SourceClientSecret != "" {
		creds["client_secret"] = tc.SourceClientSecret
	}

	// If we have a db interface, try to encrypt any sensitive fields
	if db != nil {
		switch dbImpl := db.(type) {
		case *DB:
			// Handle encrypted fields if they exist in the database
			if tc.SourcePassword != "" {
				if encrypted, err := dbImpl.EncryptCredential(tc.SourcePassword); err == nil {
					creds["encrypted_password"] = encrypted
				}
			}
			if tc.SourceSecretKey != "" {
				if encrypted, err := dbImpl.EncryptCredential(tc.SourceSecretKey); err == nil {
					creds["encrypted_secret_key"] = encrypted
				}
			}
			if tc.SourceClientSecret != "" {
				if encrypted, err := dbImpl.EncryptCredential(tc.SourceClientSecret); err == nil {
					creds["encrypted_client_secret"] = encrypted
				}
			}
		}
	}

	return creds, nil
}

// GetDestinationCredentials returns credential information for the destination, either directly or from the provider
// If db is provided, it will try to load the provider from the database if needed
func (tc *TransferConfig) GetDestinationCredentials(db interface{}) (map[string]interface{}, error) {
	creds := make(map[string]interface{})

	fmt.Printf("DEBUG GetDestCreds Start: ProviderID=%v, HasProvider=%v\n",
		tc.DestinationProviderID,
		tc.DestinationProvider != nil)

	// If using provider reference and provider is loaded
	if tc.IsUsingDestinationProviderReference() {
		// Try to load provider from database if we have a valid ID but no provider
		if tc.DestinationProvider == nil && db != nil {
			// Try different types of DB interfaces to load the provider
			switch dbImpl := db.(type) {
			case *DB:
				provider, err := dbImpl.GetStorageProvider(*tc.DestinationProviderID)
				if err != nil {
					return nil, fmt.Errorf("failed to load destination provider (ID %d): %w", *tc.DestinationProviderID, err)
				}
				tc.DestinationProvider = provider
			case interface {
				GetStorageProvider(id uint) (*StorageProvider, error)
			}:
				provider, err := dbImpl.GetStorageProvider(*tc.DestinationProviderID)
				if err != nil {
					return nil, fmt.Errorf("failed to load destination provider (ID %d): %w", *tc.DestinationProviderID, err)
				}
				tc.DestinationProvider = provider
			default:
				return nil, fmt.Errorf("destination provider not loaded and db interface cannot load providers")
			}
		}

		// If we still don't have a provider or it has no ID, return error
		if tc.DestinationProvider == nil || tc.DestinationProvider.ID == 0 {
			return nil, fmt.Errorf("failed to load valid destination provider (ID %d)", *tc.DestinationProviderID)
		}

		if tc.DestinationProvider != nil {
			fmt.Printf("DEBUG Provider Details:\n"+
				"  ID: %v\n"+
				"  Type: %v\n"+
				"  Host: %v\n"+
				"  Port: %v\n"+
				"  Username: %v\n"+
				"  HasEncryptedPassword: %v\n"+
				"  HasKeyFile: %v\n"+
				"  HasSecretKey: %v\n"+
				"  HasClientSecret: %v\n"+
				"  HasRefreshToken: %v\n",
				tc.DestinationProvider.ID,
				tc.DestinationProvider.Type,
				tc.DestinationProvider.Host,
				tc.DestinationProvider.Port,
				tc.DestinationProvider.Username,
				tc.DestinationProvider.EncryptedPassword != "",
				tc.DestinationProvider.KeyFile != "",
				tc.DestinationProvider.EncryptedSecretKey != "",
				tc.DestinationProvider.EncryptedClientSecret != "",
				tc.DestinationProvider.EncryptedRefreshToken != "")
		}

		// Copy credentials from provider
		creds["type"] = tc.DestinationProvider.Type
		creds["host"] = tc.DestinationProvider.Host
		creds["port"] = tc.DestinationProvider.Port
		creds["username"] = tc.DestinationProvider.Username
		creds["encrypted_password"] = tc.DestinationProvider.EncryptedPassword
		creds["key_file"] = tc.DestinationProvider.KeyFile

		// Handle S3 fields
		creds["bucket"] = tc.DestinationProvider.Bucket
		creds["region"] = tc.DestinationProvider.Region
		creds["access_key"] = tc.DestinationProvider.AccessKey
		creds["encrypted_secret_key"] = tc.DestinationProvider.EncryptedSecretKey
		creds["endpoint"] = tc.DestinationProvider.Endpoint

		// Handle SMB fields
		creds["share"] = tc.DestinationProvider.Share
		creds["domain"] = tc.DestinationProvider.Domain

		// Handle FTP fields
		if tc.DestinationProvider.PassiveMode != nil {
			creds["passive_mode"] = *tc.DestinationProvider.PassiveMode
		}

		// Handle OAuth fields
		creds["client_id"] = tc.DestinationProvider.ClientID
		creds["encrypted_client_secret"] = tc.DestinationProvider.EncryptedClientSecret
		creds["encrypted_refresh_token"] = tc.DestinationProvider.EncryptedRefreshToken
		creds["drive_id"] = tc.DestinationProvider.DriveID
		creds["team_drive"] = tc.DestinationProvider.TeamDrive

		if tc.DestinationProvider.ReadOnly != nil {
			creds["read_only"] = *tc.DestinationProvider.ReadOnly
		}
		creds["start_year"] = tc.DestinationProvider.StartYear
		if tc.DestinationProvider.IncludeArchived != nil {
			creds["include_archived"] = *tc.DestinationProvider.IncludeArchived
		}

		if tc.DestinationProvider.UseBuiltinAuth != nil {
			creds["use_builtin_auth"] = *tc.DestinationProvider.UseBuiltinAuth
		}

		if tc.DestinationProvider.Authenticated != nil {
			creds["authenticated"] = *tc.DestinationProvider.Authenticated
		}

		fmt.Printf("DEBUG Final Provider Creds:\n"+
			"  type: %v\n"+
			"  host: %v\n"+
			"  port: %v\n"+
			"  username: %v\n"+
			"  has_encrypted_password: %v\n"+
			"  has_key_file: %v\n"+
			"  has_encrypted_secret_key: %v\n"+
			"  has_encrypted_client_secret: %v\n",
			creds["type"],
			creds["host"],
			creds["port"],
			creds["username"],
			creds["encrypted_password"] != "",
			creds["key_file"] != "",
			creds["encrypted_secret_key"] != "",
			creds["encrypted_client_secret"] != "")

		return creds, nil
	}

	// Use legacy fields directly
	creds["type"] = tc.DestinationType
	creds["host"] = tc.DestHost
	creds["port"] = tc.DestPort
	creds["username"] = tc.DestUser
	creds["key_file"] = tc.DestKeyFile

	// Handle S3 fields
	creds["bucket"] = tc.DestBucket
	creds["region"] = tc.DestRegion
	creds["access_key"] = tc.DestAccessKey
	creds["endpoint"] = tc.DestEndpoint

	// Handle SMB fields
	creds["share"] = tc.DestShare
	creds["domain"] = tc.DestDomain

	// Handle FTP fields
	if tc.DestPassiveMode != nil {
		creds["passive_mode"] = *tc.DestPassiveMode
	}

	// Handle OAuth fields
	creds["client_id"] = tc.DestClientID
	creds["drive_id"] = tc.DestDriveID
	creds["team_drive"] = tc.DestTeamDrive

	if tc.DestReadOnly != nil {
		creds["read_only"] = *tc.DestReadOnly
	}
	creds["start_year"] = tc.DestStartYear
	if tc.DestIncludeArchived != nil {
		creds["include_archived"] = *tc.DestIncludeArchived
	}

	if tc.UseBuiltinAuthDest != nil {
		creds["use_builtin_auth"] = *tc.UseBuiltinAuthDest
	}

	// Handle temporary form fields and their encrypted counterparts
	if tc.DestPassword != "" {
		creds["password"] = tc.DestPassword
	}
	if tc.DestSecretKey != "" {
		creds["secret_key"] = tc.DestSecretKey
	}
	if tc.DestClientSecret != "" {
		creds["client_secret"] = tc.DestClientSecret
	}

	// If we have a db interface, try to encrypt any sensitive fields
	if db != nil {
		switch dbImpl := db.(type) {
		case *DB:
			// Handle encrypted fields if they exist in the database
			if tc.DestPassword != "" {
				if encrypted, err := dbImpl.EncryptCredential(tc.DestPassword); err == nil {
					creds["encrypted_password"] = encrypted
				}
			}
			if tc.DestSecretKey != "" {
				if encrypted, err := dbImpl.EncryptCredential(tc.DestSecretKey); err == nil {
					creds["encrypted_secret_key"] = encrypted
				}
			}
			if tc.DestClientSecret != "" {
				if encrypted, err := dbImpl.EncryptCredential(tc.DestClientSecret); err == nil {
					creds["encrypted_client_secret"] = encrypted
				}
			}
		}
	}

	return creds, nil
}
