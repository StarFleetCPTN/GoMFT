package db

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
)

// ValidateStorageProvider validates a storage provider based on its type
func (sp *StorageProvider) Validate() error {
	// Special case - if we have an empty struct or just an ID (could happen during GORM operations like foreign key checks)
	if (sp.ID > 0 && sp.Name == "" && sp.Type == "") || (sp.ID == 0 && sp.Name == "" && sp.Type == "") {
		log.Printf("Skipping validation for StorageProvider: ID=%d without other data (likely a reference check)", sp.ID)
		return nil
	}

	// Common validations
	if strings.TrimSpace(sp.Name) == "" {
		return errors.New("provider name cannot be empty")
	}

	// Type-specific validations
	switch sp.Type {
	case ProviderTypeSFTP, ProviderTypeHetzner:
		return sp.validateSFTP()
	case ProviderTypeS3:
		return sp.validateS3()
	case ProviderTypeFTP:
		return sp.validateFTP()
	case ProviderTypeWebDAV, ProviderTypeNextcloud:
		return sp.validateWebDAV()
	case ProviderTypeSMB:
		return sp.validateSMB()
	case ProviderTypeOneDrive:
		return sp.validateOneDrive()
	case ProviderTypeGoogleDrive:
		return sp.validateGoogleDrive()
	case ProviderTypeGooglePhoto:
		return sp.validateGooglePhoto()
	case ProviderTypeLocal:
		return sp.validateLocal()
	default:
		return fmt.Errorf("unsupported provider type: %s", sp.Type)
	}
}

// validateSFTP validates SFTP-specific fields
func (sp *StorageProvider) validateSFTP() error {
	if strings.TrimSpace(sp.Host) == "" {
		return errors.New("host is required for SFTP provider")
	}

	if sp.Port <= 0 {
		return errors.New("invalid port for SFTP provider")
	}

	if strings.TrimSpace(sp.Username) == "" {
		return errors.New("username is required for SFTP provider")
	}

	// Either password or key file must be provided
	if strings.TrimSpace(sp.Password) == "" && strings.TrimSpace(sp.EncryptedPassword) == "" && strings.TrimSpace(sp.KeyFile) == "" {
		return errors.New("either password or key file is required for SFTP provider")
	}

	return nil
}

// validateWebDAV validates WebDAV-specific fields
func (sp *StorageProvider) validateWebDAV() error {
	if strings.TrimSpace(sp.Host) == "" {
		return errors.New("host is required for WebDAV provider")
	}

	// Host must include the protocol
	if !strings.HasPrefix(sp.Host, "http://") && !strings.HasPrefix(sp.Host, "https://") {
		return errors.New("host must include the protocol (http:// or https://)")
	}

	if strings.TrimSpace(sp.Username) == "" {
		return errors.New("username is required for WebDAV provider")
	}

	// Either password or encrypted password must be provided
	if strings.TrimSpace(sp.Password) == "" && strings.TrimSpace(sp.EncryptedPassword) == "" {
		return errors.New("password is required for WebDAV provider")
	}

	return nil
}

// validateS3 validates S3-specific fields
func (sp *StorageProvider) validateS3() error {
	// For S3, either AccessKey or Username is used
	if strings.TrimSpace(sp.AccessKey) == "" && strings.TrimSpace(sp.Username) == "" {
		return errors.New("access key is required for S3 provider")
	}

	// Either SecretKey or EncryptedSecretKey must be provided
	if strings.TrimSpace(sp.SecretKey) == "" && strings.TrimSpace(sp.EncryptedSecretKey) == "" {
		return errors.New("secret key is required for S3 provider")
	}

	// Region is required for most S3 providers
	if strings.TrimSpace(sp.Region) == "" {
		return errors.New("region is required for S3 provider")
	}

	return nil
}

// validateFTP validates FTP-specific fields
func (sp *StorageProvider) validateFTP() error {
	if strings.TrimSpace(sp.Host) == "" {
		return errors.New("host is required for FTP provider")
	}

	if sp.Port <= 0 {
		return errors.New("invalid port for FTP provider")
	}

	if strings.TrimSpace(sp.Username) == "" {
		return errors.New("username is required for FTP provider")
	}

	// Either password or encrypted password must be provided
	if strings.TrimSpace(sp.Password) == "" && strings.TrimSpace(sp.EncryptedPassword) == "" {
		return errors.New("password is required for FTP provider")
	}

	return nil
}

// validateSMB validates SMB-specific fields
func (sp *StorageProvider) validateSMB() error {
	if strings.TrimSpace(sp.Host) == "" {
		return errors.New("host is required for SMB provider")
	}

	if strings.TrimSpace(sp.Username) == "" {
		return errors.New("username is required for SMB provider")
	}

	// Either password or encrypted password must be provided
	if strings.TrimSpace(sp.Password) == "" && strings.TrimSpace(sp.EncryptedPassword) == "" {
		return errors.New("password is required for SMB provider")
	}

	return nil
}

// validateOneDrive validates OneDrive-specific fields
func (sp *StorageProvider) validateOneDrive() error {
	if strings.TrimSpace(sp.ClientID) == "" {
		return errors.New("client ID is required for OneDrive provider")
	}

	// Either ClientSecret or EncryptedClientSecret must be provided
	if strings.TrimSpace(sp.ClientSecret) == "" && strings.TrimSpace(sp.EncryptedClientSecret) == "" {
		return errors.New("client secret is required for OneDrive provider")
	}

	// For authenticated providers, RefreshToken must be set
	if sp.GetAuthenticated() && strings.TrimSpace(sp.EncryptedRefreshToken) == "" && strings.TrimSpace(sp.RefreshToken) == "" {
		return errors.New("refresh token is required for authenticated OneDrive provider")
	}

	return nil
}

// validateGoogleDrive validates Google Drive-specific fields
func (sp *StorageProvider) validateGoogleDrive() error {
	// If not using builtin auth, ClientID and ClientSecret are required
	if !sp.GetUseBuiltinAuth() {
		if strings.TrimSpace(sp.ClientID) == "" {
			return errors.New("client ID is required for Google Drive provider when not using builtin auth")
		}

		// Either ClientSecret or EncryptedClientSecret must be provided
		if strings.TrimSpace(sp.ClientSecret) == "" && strings.TrimSpace(sp.EncryptedClientSecret) == "" {
			return errors.New("client secret is required for Google Drive provider when not using builtin auth")
		}
	}

	// For authenticated providers, RefreshToken must be set
	if sp.GetAuthenticated() && strings.TrimSpace(sp.EncryptedRefreshToken) == "" && strings.TrimSpace(sp.RefreshToken) == "" {
		return errors.New("refresh token is required for authenticated Google Drive provider")
	}

	return nil
}

// validateGooglePhoto validates Google Photos-specific fields
func (sp *StorageProvider) validateGooglePhoto() error {
	// Similar to Google Drive
	return sp.validateGoogleDrive()
}

// validateLocal validates Local-specific fields
func (sp *StorageProvider) validateLocal() error {
	// Local providers don't need additional validation
	return nil
}

// BeforeSave is a GORM hook that runs before saving the provider
func (sp *StorageProvider) BeforeSave(tx *gorm.DB) error {
	// Check if this is a reference check by examining the GORM operation
	if tx.Statement.SQL.String() == "" {
		// No explicit SQL means this might be part of a preload or association check

		// Case 1: Empty struct (as you already have)
		if sp.ID == 0 && sp.Name == "" && sp.Type == "" {
			log.Printf("BeforeSave: Skipping validation for empty StorageProvider")
			return nil
		}

		// Case 2: ID-only struct (foreign key reference check)
		if sp.ID > 0 && sp.Name == "" && sp.Type == "" {
			log.Printf("BeforeSave: Skipping validation for StorageProvider ID=%d (reference check)", sp.ID)
			return nil
		}

		// Case 3: Minimal data loaded from database for relationship check
		// Check if only a few fields are populated (typically ID and maybe a couple others)
		populatedFields := 0
		if sp.ID > 0 {
			populatedFields++
		}
		if sp.Name != "" {
			populatedFields++
		}
		if string(sp.Type) != "" {
			populatedFields++
		}
		if sp.Host != "" {
			populatedFields++
		}
		if sp.Username != "" {
			populatedFields++
		}

		// If we have just a few populated fields, it's likely a reference check
		if populatedFields <= 3 {
			log.Printf("BeforeSave: Skipping validation for partially loaded StorageProvider ID=%d (likely reference check)", sp.ID)
			return nil
		}
	}

	// Check if this is called from a foreign key operation on another model
	stmt := tx.Statement
	if stmt.Schema != nil && stmt.Schema.Table != "storage_providers" {
		log.Printf("BeforeSave: Skipping validation for StorageProvider ID=%d (called from %s table operation)",
			sp.ID, stmt.Schema.Table)
		return nil
	}

	log.Printf("BeforeSave: Validating StorageProvider: ID=%d, Name=%s, Type=%s", sp.ID, sp.Name, sp.Type)
	return sp.Validate()
}
