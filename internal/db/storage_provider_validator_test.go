package db

import (
	"testing"
)

// TestStorageProviderValidation tests the validation of storage providers
func TestStorageProviderValidation(t *testing.T) {
	tests := []struct {
		name      string
		provider  StorageProvider
		wantError bool
	}{
		{
			name: "Valid SFTP provider",
			provider: StorageProvider{
				Name:     "Test SFTP",
				Type:     ProviderTypeSFTP,
				Host:     "example.com",
				Port:     22,
				Username: "user",
				Password: "pass",
			},
			wantError: false,
		},
		{
			name: "Invalid SFTP provider - missing host",
			provider: StorageProvider{
				Name:     "Test SFTP",
				Type:     ProviderTypeSFTP,
				Port:     22,
				Username: "user",
				Password: "pass",
			},
			wantError: true,
		},
		{
			name: "Valid S3 provider",
			provider: StorageProvider{
				Name:      "Test S3",
				Type:      ProviderTypeS3,
				AccessKey: "accesskey",
				SecretKey: "secretkey",
				Region:    "us-west-1",
			},
			wantError: false,
		},
		{
			name: "Valid OneDrive provider",
			provider: StorageProvider{
				Name:         "Test OneDrive",
				Type:         ProviderTypeOneDrive,
				ClientID:     "clientid",
				ClientSecret: "clientsecret",
			},
			wantError: false,
		},
		// Testing all provider types to ensure they're correctly recognized in the switch statement
		{
			name: "Valid Hetzner provider",
			provider: StorageProvider{
				Name:     "Test Hetzner",
				Type:     ProviderTypeHetzner,
				Host:     "example.com",
				Port:     22,
				Username: "user",
				Password: "pass",
			},
			wantError: false,
		},
		{
			name: "Valid FTP provider",
			provider: StorageProvider{
				Name:     "Test FTP",
				Type:     ProviderTypeFTP,
				Host:     "example.com",
				Port:     21,
				Username: "user",
				Password: "pass",
			},
			wantError: false,
		},
		{
			name: "Valid SMB provider",
			provider: StorageProvider{
				Name:     "Test SMB",
				Type:     ProviderTypeSMB,
				Host:     "example.com",
				Share:    "share",
				Username: "user",
				Password: "pass",
			},
			wantError: false,
		},
		{
			name: "Valid Google Drive provider",
			provider: StorageProvider{
				Name:         "Test Google Drive",
				Type:         ProviderTypeGoogleDrive,
				ClientID:     "clientid",
				ClientSecret: "clientsecret",
			},
			wantError: false,
		},
		{
			name: "Valid Google Photo provider",
			provider: StorageProvider{
				Name:         "Test Google Photo",
				Type:         ProviderTypeGooglePhoto,
				ClientID:     "clientid",
				ClientSecret: "clientsecret",
			},
			wantError: false,
		},
		{
			name: "Valid Local provider",
			provider: StorageProvider{
				Name: "Test Local",
				Type: ProviderTypeLocal,
			},
			wantError: false,
		},
		{
			name: "Invalid provider type",
			provider: StorageProvider{
				Name: "Test Invalid",
				Type: "invalid_type",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
