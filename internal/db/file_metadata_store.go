package db

// --- FileMetadata Store Methods ---

// CreateFileMetadata creates a new file metadata record
func (db *DB) CreateFileMetadata(metadata *FileMetadata) error {
	return db.Create(metadata).Error
}

// GetFileMetadataByJobAndName retrieves file metadata by job ID and filename
func (db *DB) GetFileMetadataByJobAndName(jobID uint, fileName string) (*FileMetadata, error) {
	var metadata FileMetadata
	err := db.Where("job_id = ? AND file_name = ?", jobID, fileName).First(&metadata).Error
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

// GetFileMetadataByHash retrieves file metadata by file hash
func (db *DB) GetFileMetadataByHash(fileHash string) (*FileMetadata, error) {
	var metadata FileMetadata
	err := db.Where("file_hash = ?", fileHash).First(&metadata).Error
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

// DeleteFileMetadata deletes file metadata by ID
func (db *DB) DeleteFileMetadata(id uint) error {
	return db.Delete(&FileMetadata{}, id).Error
}
