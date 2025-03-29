package db

import (
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
	SourcePassiveMode *bool `gorm:"default:true" form:"source_passive_mode"`
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
