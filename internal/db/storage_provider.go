package db

import (
	"time"
)

// StorageProviderType defines the type of storage provider
type StorageProviderType string

const (
	// Storage provider types
	ProviderTypeSFTP        StorageProviderType = "sftp"
	ProviderTypeS3          StorageProviderType = "s3"
	ProviderTypeOneDrive    StorageProviderType = "onedrive"
	ProviderTypeGoogleDrive StorageProviderType = "google_drive"
	ProviderTypeGooglePhoto StorageProviderType = "google_photo"
	ProviderTypeFTP         StorageProviderType = "ftp"
	ProviderTypeSMB         StorageProviderType = "smb"
	ProviderTypeHetzner     StorageProviderType = "hetzner"
	ProviderTypeLocal       StorageProviderType = "local"
	ProviderTypeWebDAV      StorageProviderType = "webdav"
	ProviderTypeNextcloud   StorageProviderType = "nextcloud"
	ProviderTypeB2          StorageProviderType = "b2"
	ProviderTypeWasabi      StorageProviderType = "wasabi"
	ProviderTypeMinio       StorageProviderType = "minio"
)

// StorageProvider represents a connection to a storage service
type StorageProvider struct {
	ID   uint                `gorm:"primarykey" json:"id"`
	Name string              `gorm:"not null;uniqueIndex:idx_storage_providers_name_created_by" json:"name" form:"name"`
	Type StorageProviderType `gorm:"not null" json:"type" form:"type"`

	// Common fields
	Host     string `json:"host" form:"host"`                   // For server-based providers (SFTP, FTP, SMB)
	Port     int    `gorm:"default:22" json:"port" form:"port"` // For server-based providers
	Username string `json:"username" form:"username"`           // Or AccessKey for S3

	// Password is not stored in the database, only used for form input
	Password string `gorm:"-" json:"-" form:"password"`

	// These fields will be encrypted before storage
	EncryptedPassword string `json:"-"` // Encrypted version of Password
	KeyFile           string `json:"key_file" form:"key_file"`

	// S3 specific fields
	Bucket    string `json:"bucket" form:"bucket"`
	Region    string `json:"region" form:"region"`
	AccessKey string `json:"access_key" form:"access_key"` // Alternative to Username for S3

	// SecretKey is not stored in the database, only used for form input
	SecretKey string `gorm:"-" json:"-" form:"secret_key"`

	// Encrypted version of SecretKey
	EncryptedSecretKey string `json:"-"`

	Endpoint string `json:"endpoint" form:"endpoint"`

	// SMB specific fields
	Share  string `json:"share" form:"share"`
	Domain string `json:"domain" form:"domain"`

	// FTP specific fields
	PassiveMode *bool `gorm:"default:true" json:"passive_mode" form:"passive_mode"`

	// OAuth-related fields for cloud providers (OneDrive, GoogleDrive, GooglePhoto)
	ClientID string `json:"client_id" form:"client_id"`

	// ClientSecret is not stored in the database, only used for form input
	ClientSecret string `gorm:"-" json:"-" form:"client_secret"`

	// Encrypted version of ClientSecret
	EncryptedClientSecret string `json:"-"`

	// RefreshToken is not stored in the database, only used for form input
	RefreshToken string `gorm:"-" json:"-" form:"refresh_token"`

	// Encrypted version of RefreshToken
	EncryptedRefreshToken string `json:"-"`

	// OAuth specific fields
	DriveID         string `json:"drive_id" form:"drive_id"`                 // For OneDrive
	TeamDrive       string `json:"team_drive" form:"team_drive"`             // For Google Drive
	ReadOnly        *bool  `json:"read_only" form:"read_only"`               // For Google Photos
	StartYear       int    `json:"start_year" form:"start_year"`             // For Google Photos
	IncludeArchived *bool  `json:"include_archived" form:"include_archived"` // For Google Photos

	// Security fields
	UseBuiltinAuth *bool `gorm:"default:true" json:"use_builtin_auth" form:"use_builtin_auth"` // For OAuth services

	// Status fields
	Authenticated *bool `json:"authenticated"` // Whether auth is completed (for OAuth providers)

	// Ownership and timestamps
	CreatedBy uint      `gorm:"not null;uniqueIndex:idx_storage_providers_name_created_by" json:"created_by"`
	User      User      `gorm:"foreignkey:CreatedBy" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// --- StorageProvider Helper Methods ---

// GetPassiveMode returns the value of PassiveMode with a default if nil
func (sp *StorageProvider) GetPassiveMode() bool {
	if sp.PassiveMode == nil {
		return true // Default to true if not set
	}
	return *sp.PassiveMode
}

// SetPassiveMode sets the PassiveMode field
func (sp *StorageProvider) SetPassiveMode(value bool) {
	sp.PassiveMode = &value
}

// GetReadOnly returns the value of ReadOnly with a default if nil
func (sp *StorageProvider) GetReadOnly() bool {
	if sp.ReadOnly == nil {
		return false // Default to false if not set
	}
	return *sp.ReadOnly
}

// SetReadOnly sets the ReadOnly field
func (sp *StorageProvider) SetReadOnly(value bool) {
	sp.ReadOnly = &value
}

// GetIncludeArchived returns the value of IncludeArchived with a default if nil
func (sp *StorageProvider) GetIncludeArchived() bool {
	if sp.IncludeArchived == nil {
		return false // Default to false if not set
	}
	return *sp.IncludeArchived
}

// SetIncludeArchived sets the IncludeArchived field
func (sp *StorageProvider) SetIncludeArchived(value bool) {
	sp.IncludeArchived = &value
}

// GetUseBuiltinAuth returns the value of UseBuiltinAuth with a default if nil
func (sp *StorageProvider) GetUseBuiltinAuth() bool {
	if sp.UseBuiltinAuth == nil {
		return true // Default to true if not set
	}
	return *sp.UseBuiltinAuth
}

// SetUseBuiltinAuth sets the UseBuiltinAuth field
func (sp *StorageProvider) SetUseBuiltinAuth(value bool) {
	sp.UseBuiltinAuth = &value
}

// GetAuthenticated returns the value of Authenticated with a default if nil
func (sp *StorageProvider) GetAuthenticated() bool {
	if sp.Authenticated == nil {
		return false // Default to false if not set
	}
	return *sp.Authenticated
}

// SetAuthenticated sets the Authenticated field
func (sp *StorageProvider) SetAuthenticated(value bool) {
	sp.Authenticated = &value
}

// IsOAuthProvider returns true if the provider type requires OAuth authentication
func (sp *StorageProvider) IsOAuthProvider() bool {
	return sp.Type == ProviderTypeOneDrive ||
		sp.Type == ProviderTypeGoogleDrive ||
		sp.Type == ProviderTypeGooglePhoto
}

// RequiresEncryption returns true if the provider has sensitive fields that need encryption
func (sp *StorageProvider) RequiresEncryption() bool {
	// All provider types have some form of sensitive authentication that needs encryption
	return true
}

// GetSensitiveFields returns a map of field names to values that need encryption
func (sp *StorageProvider) GetSensitiveFields() map[string]string {
	sensitiveFields := make(map[string]string)

	// Add fields based on provider type
	switch sp.Type {
	case ProviderTypeSFTP, ProviderTypeFTP, ProviderTypeSMB, ProviderTypeHetzner, ProviderTypeWebDAV, ProviderTypeNextcloud:
		if sp.Password != "" {
			sensitiveFields["Password"] = sp.Password
		}
	case ProviderTypeS3, ProviderTypeWasabi, ProviderTypeMinio, ProviderTypeB2:
		if sp.SecretKey != "" {
			sensitiveFields["SecretKey"] = sp.SecretKey
		}
	case ProviderTypeOneDrive, ProviderTypeGoogleDrive, ProviderTypeGooglePhoto:
		if sp.ClientSecret != "" {
			sensitiveFields["ClientSecret"] = sp.ClientSecret
		}
		if sp.RefreshToken != "" {
			sensitiveFields["RefreshToken"] = sp.RefreshToken
		}
	}

	return sensitiveFields
}

// GetEncryptedFieldName returns the corresponding encrypted field name for a given sensitive field
func (sp *StorageProvider) GetEncryptedFieldName(fieldName string) string {
	return "Encrypted" + fieldName
}
