package db

import (
	"fmt"
)

// --- StorageProvider Store Methods ---

// CreateStorageProvider creates a new storage provider record
func (db *DB) CreateStorageProvider(provider *StorageProvider) error {
	return db.Create(provider).Error
}

// GetStorageProviders retrieves all storage providers for a user
func (db *DB) GetStorageProviders(userID uint) ([]StorageProvider, error) {
	var providers []StorageProvider
	err := db.Where("created_by = ?", userID).Find(&providers).Error
	return providers, err
}

// GetStorageProvidersByType retrieves all storage providers of a specific type for a user
func (db *DB) GetStorageProvidersByType(userID uint, providerType StorageProviderType) ([]StorageProvider, error) {
	var providers []StorageProvider
	err := db.Where("created_by = ? AND type = ?", userID, providerType).Find(&providers).Error
	return providers, err
}

// GetStorageProvider retrieves a single storage provider by ID
func (db *DB) GetStorageProvider(id uint) (*StorageProvider, error) {
	var provider StorageProvider
	err := db.First(&provider, id).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetStorageProviderType retrieves the type of a storage provider by ID
func (db *DB) GetStorageProviderType(id uint) (StorageProviderType, error) {
	var provider StorageProvider
	err := db.First(&provider, id).Error
	return provider.Type, err
}

// GetStorageProviderWithOwnerCheck retrieves a single storage provider by ID with owner check
func (db *DB) GetStorageProviderWithOwnerCheck(id uint, userID uint) (*StorageProvider, error) {
	var provider StorageProvider
	err := db.Where("id = ? AND created_by = ?", id, userID).First(&provider).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// UpdateStorageProvider updates an existing storage provider record
func (db *DB) UpdateStorageProvider(provider *StorageProvider) error {
	return db.Save(provider).Error
}

// DeleteStorageProvider deletes a storage provider record after checking dependencies
func (db *DB) DeleteStorageProvider(id uint) error {
	// First check if any transfer configs are using this provider
	var count int64
	if err := db.Model(&TransferConfig{}).
		Where("source_provider_id = ? OR destination_provider_id = ?", id, id).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check for dependent transfer configs: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("cannot delete provider: %d transfer configurations are using this provider", count)
	}

	// Delete the provider
	return db.Delete(&StorageProvider{}, id).Error
}

// CountStorageProviders counts the number of storage providers for a user
func (db *DB) CountStorageProviders(userID uint) (int64, error) {
	var count int64
	err := db.Model(&StorageProvider{}).Where("created_by = ?", userID).Count(&count).Error
	return count, err
}
