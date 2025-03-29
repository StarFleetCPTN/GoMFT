package db

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// GetAllAuthProviders returns all authentication providers
func (db *DB) GetAllAuthProviders(ctx context.Context) ([]AuthProvider, error) {
	var providers []AuthProvider
	tx := db.WithContext(ctx).Order("name asc").Find(&providers)
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to get auth providers: %w", tx.Error)
	}
	return providers, nil
}

// GetAuthProviderByID retrieves an authentication provider by ID
func (db *DB) GetAuthProviderByID(ctx context.Context, id uint) (*AuthProvider, error) {
	var provider AuthProvider
	tx := db.WithContext(ctx).First(&provider, id)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("auth provider not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get auth provider: %w", tx.Error)
	}
	return &provider, nil
}

// CreateAuthProvider creates a new authentication provider
func (db *DB) CreateAuthProvider(ctx context.Context, provider *AuthProvider) error {
	tx := db.WithContext(ctx).Create(provider)
	if tx.Error != nil {
		return fmt.Errorf("failed to create auth provider: %w", tx.Error)
	}
	return nil
}

// UpdateAuthProvider updates an existing authentication provider
func (db *DB) UpdateAuthProvider(ctx context.Context, provider *AuthProvider) error {
	tx := db.WithContext(ctx).Save(provider)
	if tx.Error != nil {
		return fmt.Errorf("failed to update auth provider: %w", tx.Error)
	}
	return nil
}

// DeleteAuthProvider deletes an authentication provider by ID
func (db *DB) DeleteAuthProvider(ctx context.Context, id uint) error {
	tx := db.WithContext(ctx).Delete(&AuthProvider{}, id)
	if tx.Error != nil {
		return fmt.Errorf("failed to delete auth provider: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("auth provider not found: %d", id)
	}
	return nil
}

// GetEnabledAuthProviders returns all enabled authentication providers
func (db *DB) GetEnabledAuthProviders(ctx context.Context) ([]AuthProvider, error) {
	var providers []AuthProvider
	tx := db.WithContext(ctx).Where("enabled = ?", true).Order("name asc").Find(&providers)
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to get enabled auth providers: %w", tx.Error)
	}
	return providers, nil
}

// GetAuthProviderByType returns authentication providers of a specific type
func (db *DB) GetAuthProviderByType(ctx context.Context, providerType ProviderType) ([]AuthProvider, error) {
	var providers []AuthProvider
	tx := db.WithContext(ctx).Where("type = ?", providerType).Order("name asc").Find(&providers)
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to get auth providers by type: %w", tx.Error)
	}
	return providers, nil
}

// GetExternalUserIdentitiesByProviderID returns all external user identities for a specific provider
func (db *DB) GetExternalUserIdentitiesByProviderID(ctx context.Context, providerID uint) ([]ExternalUserIdentity, error) {
	var identities []ExternalUserIdentity
	tx := db.WithContext(ctx).Where("provider_id = ?", providerID).Find(&identities)
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to get external user identities: %w", tx.Error)
	}
	return identities, nil
}

// CountExternalUserIdentitiesByProviderID counts the number of external user identities for a specific provider
func (db *DB) CountExternalUserIdentitiesByProviderID(ctx context.Context, providerID uint) (int64, error) {
	var count int64
	tx := db.WithContext(ctx).Model(&ExternalUserIdentity{}).Where("provider_id = ?", providerID).Count(&count)
	if tx.Error != nil {
		return 0, fmt.Errorf("failed to count external user identities: %w", tx.Error)
	}
	return count, nil
}

// GetExternalUserIdentity gets an external user identity by provider ID and external ID
func (db *DB) GetExternalUserIdentity(ctx context.Context, providerID uint, externalID string) (*ExternalUserIdentity, error) {
	var identity ExternalUserIdentity
	tx := db.WithContext(ctx).Where("provider_id = ? AND external_id = ?", providerID, externalID).First(&identity)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil // Not found, but not an error
		}
		return nil, fmt.Errorf("failed to get external user identity: %w", tx.Error)
	}
	return &identity, nil
}

// CreateExternalUserIdentity creates a new external user identity
func (db *DB) CreateExternalUserIdentity(ctx context.Context, identity *ExternalUserIdentity) error {
	tx := db.WithContext(ctx).Create(identity)
	if tx.Error != nil {
		return fmt.Errorf("failed to create external user identity: %w", tx.Error)
	}
	return nil
}

// UpdateExternalUserIdentity updates an existing external user identity
func (db *DB) UpdateExternalUserIdentity(ctx context.Context, identity *ExternalUserIdentity) error {
	tx := db.WithContext(ctx).Save(identity)
	if tx.Error != nil {
		return fmt.Errorf("failed to update external user identity: %w", tx.Error)
	}
	return nil
}

// DeleteExternalUserIdentity deletes an external user identity by ID
func (db *DB) DeleteExternalUserIdentity(ctx context.Context, id uint) error {
	tx := db.WithContext(ctx).Delete(&ExternalUserIdentity{}, id)
	if tx.Error != nil {
		return fmt.Errorf("failed to delete external user identity: %w", tx.Error)
	}
	return nil
}

// UpdateAuthProviderLastUsed updates the last used timestamp and increments the successful logins counter
func (db *DB) UpdateAuthProviderLastUsed(ctx context.Context, providerID uint) error {
	tx := db.WithContext(ctx).Model(&AuthProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"last_used":         gorm.Expr("NOW()"),
			"successful_logins": gorm.Expr("successful_logins + 1"),
		})
	if tx.Error != nil {
		return fmt.Errorf("failed to update auth provider last used: %w", tx.Error)
	}
	return nil
}
