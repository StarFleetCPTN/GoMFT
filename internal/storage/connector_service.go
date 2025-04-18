package storage

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/starfleetcptn/gomft/internal/rclone_service"
)

// ConnectorService manages storage provider connection testing
type ConnectorService struct {
	dbInstance          *db.DB
	encryptionSvc       *encryption.EncryptionService
	credentialEncryptor *encryption.CredentialEncryptor
}

// NewConnectorService creates a new ConnectorService
func NewConnectorService(dbInstance *db.DB) (*ConnectorService, error) {
	// Get the global encryption service
	encryptionSvc, err := encryption.GetGlobalEncryptionService()
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption service: %w", err)
	}

	// Get the global credential encryptor
	credentialEncryptor, err := encryption.GetGlobalCredentialEncryptor()
	if err != nil {
		return nil, fmt.Errorf("failed to get credential encryptor: %w", err)
	}

	return &ConnectorService{
		dbInstance:          dbInstance,
		encryptionSvc:       encryptionSvc,
		credentialEncryptor: credentialEncryptor,
	}, nil
}

// TestConnection tests a connection to a storage provider using rclone
func (s *ConnectorService) TestConnection(ctx context.Context, providerID uint, userID uint) (*db.ConnectionResult, error) {
	// Get the provider from the database with owner check
	provider, err := s.dbInstance.GetStorageProviderWithOwnerCheck(providerID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage provider: %w", err)
	}

	// Decrypt sensitive fields
	if err := s.decryptProviderCredentials(provider); err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Create a temporary TransferConfig with just the source fields populated
	tempConfig := createTempTransferConfig(provider)

	// Use the rclone service to test the connection
	success, message, err := rclone_service.TestRcloneConnection(*tempConfig, "source", s.dbInstance)

	// Create the connection result
	result := &db.ConnectionResult{
		Success:   success,
		Message:   message,
		Timestamp: time.Now(),
	}

	// If there was an error, add it to the result
	if err != nil {
		errorCode := determineErrorCode(err.Error())
		result.Error = &db.ConnectorError{
			Code:    errorCode,
			Message: err.Error(),
			Err:     err,
		}
	}

	// Record the test result in logs (without sensitive info)
	s.logConnectionTest(provider, result)

	return result, nil
}

// createTempTransferConfig creates a temporary TransferConfig for connection testing
func createTempTransferConfig(provider *db.StorageProvider) *db.TransferConfig {
	config := &db.TransferConfig{
		SourceType: string(provider.Type),
		SourceHost: provider.Host,
		SourcePort: provider.Port,
	}

	// Set the right credential fields based on provider type
	switch provider.Type {
	case db.ProviderTypeSFTP, db.ProviderTypeFTP, db.ProviderTypeSMB, db.ProviderTypeHetzner:
		config.SourceUser = provider.Username
		config.SourcePassword = provider.Password
		config.SourceKeyFile = provider.KeyFile
		config.SourceDomain = provider.Domain

		// Set passive mode for FTP
		if provider.Type == db.ProviderTypeFTP && provider.PassiveMode != nil {
			passive := provider.GetPassiveMode()
			config.SetSourcePassiveMode(passive)
		}

	case db.ProviderTypeS3, db.ProviderTypeWasabi, db.ProviderTypeMinio:
		config.SourceAccessKey = provider.AccessKey
		config.SourceSecretKey = provider.SecretKey
		config.SourceBucket = provider.Bucket
		config.SourceRegion = provider.Region
		config.SourceEndpoint = provider.Endpoint

	case db.ProviderTypeB2:
		// B2 uses AccessKey as account and SecretKey as application key
		config.SourceAccessKey = provider.AccessKey
		config.SourceSecretKey = provider.SecretKey
		config.SourceBucket = provider.Bucket
		config.SourceRegion = provider.Region
		config.SourceEndpoint = provider.Endpoint

	case db.ProviderTypeOneDrive, db.ProviderTypeGoogleDrive, db.ProviderTypeGooglePhoto:
		config.SourceClientID = provider.ClientID
		config.SourceClientSecret = provider.ClientSecret
		config.SourceDriveID = provider.DriveID
		config.SourceTeamDrive = provider.TeamDrive

		// For Google Photos, we would set read-only mode if the method existed
		// Currently commented out as SetSourceReadOnly doesn't exist
		// if provider.Type == db.ProviderTypeGooglePhoto && provider.ReadOnly != nil {
		//     readonly := provider.GetReadOnly()
		//     config.SetSourceReadOnly(readonly)
		// }
	}

	return config
}

// determineErrorCode maps rclone error messages to our error code system
func determineErrorCode(errMsg string) string {
	switch {
	case strings.Contains(errMsg, "connection refused"), strings.Contains(errMsg, "dial tcp"):
		return db.ErrorCodeConnection
	case strings.Contains(errMsg, "no such host"), strings.Contains(errMsg, "network is unreachable"):
		return db.ErrorCodeNetwork
	case strings.Contains(errMsg, "timeout"), strings.Contains(errMsg, "timed out"):
		return db.ErrorCodeTimeout
	case strings.Contains(errMsg, "authentication failed"), strings.Contains(errMsg, "login incorrect"),
		strings.Contains(errMsg, "permission denied"), strings.Contains(errMsg, "invalid credentials"):
		return db.ErrorCodeAuthentication
	case strings.Contains(errMsg, "directory not found"), strings.Contains(errMsg, "no such file"):
		return db.ErrorCodeResourceNotFound
	case strings.Contains(errMsg, "invalid parameters"):
		return db.ErrorCodeInvalidParams
	default:
		return db.ErrorCodeUnknown
	}
}

// decryptProviderCredentials decrypts the provider's sensitive fields
func (s *ConnectorService) decryptProviderCredentials(provider *db.StorageProvider) error {
	// Handle different provider types
	switch provider.Type {
	case db.ProviderTypeSFTP, db.ProviderTypeFTP, db.ProviderTypeSMB, db.ProviderTypeHetzner:
		if provider.EncryptedPassword != "" {
			password, err := s.credentialEncryptor.Decrypt(provider.EncryptedPassword)
			if err != nil {
				return fmt.Errorf("failed to decrypt password: %w", err)
			}
			provider.Password = password
		}

	case db.ProviderTypeS3, db.ProviderTypeWasabi, db.ProviderTypeMinio, db.ProviderTypeB2:
		if provider.EncryptedSecretKey != "" {
			secretKey, err := s.credentialEncryptor.Decrypt(provider.EncryptedSecretKey)
			if err != nil {
				return fmt.Errorf("failed to decrypt secret key: %w", err)
			}
			provider.SecretKey = secretKey
			log.Printf("DEBUG: Decrypted secret key for %s provider (ID: %d, Type: %s) with length: %d",
				provider.Name, provider.ID, provider.Type, len(provider.SecretKey))
		} else {
			log.Printf("WARNING: No encrypted secret key found for %s provider (ID: %d, Type: %s)",
				provider.Name, provider.ID, provider.Type)
		}

	case db.ProviderTypeOneDrive, db.ProviderTypeGoogleDrive, db.ProviderTypeGooglePhoto:
		if provider.EncryptedClientSecret != "" {
			clientSecret, err := s.credentialEncryptor.Decrypt(provider.EncryptedClientSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt client secret: %w", err)
			}
			provider.ClientSecret = clientSecret
		}
		if provider.EncryptedRefreshToken != "" {
			refreshToken, err := s.credentialEncryptor.Decrypt(provider.EncryptedRefreshToken)
			if err != nil {
				return fmt.Errorf("failed to decrypt refresh token: %w", err)
			}
			provider.RefreshToken = refreshToken
		}
	}

	return nil
}

// logConnectionTest logs the connection test result without sensitive information
func (s *ConnectorService) logConnectionTest(provider *db.StorageProvider, result *db.ConnectionResult) {
	if result.Success {
		log.Printf("Connection test successful for provider %s (ID: %d, Type: %s)",
			provider.Name, provider.ID, provider.Type)
	} else {
		errorCode := "unknown"
		if result.Error != nil {
			errorCode = result.Error.Code
		}
		log.Printf("Connection test failed for provider %s (ID: %d, Type: %s): %s [%s]",
			provider.Name, provider.ID, provider.Type, result.Message, errorCode)
	}
}
