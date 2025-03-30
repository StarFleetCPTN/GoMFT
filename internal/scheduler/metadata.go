package scheduler

import (
	"errors"
	"fmt"

	"github.com/starfleetcptn/gomft/internal/db"
	"gorm.io/gorm"
)

// MetadataDB defines the database methods needed by MetadataHandler.
// This allows for easier mocking during testing.
type MetadataDB interface {
	GetFileMetadataByHash(hash string) (*db.FileMetadata, error)
	GetFileMetadataByJobAndName(jobID uint, fileName string) (*db.FileMetadata, error)
}

// MetadataHandler handles checking file processing history.
type MetadataHandler struct {
	db     MetadataDB // Use the interface type
	logger *Logger    // Added logger dependency
}

// NewMetadataHandler creates a new MetadataHandler.
func NewMetadataHandler(database MetadataDB, logger *Logger) *MetadataHandler { // Accept the interface type
	return &MetadataHandler{
		db:     database,
		logger: logger,
	}
}

// hasFileBeenProcessed checks if a file with the same hash has been processed before.
func (mh *MetadataHandler) hasFileBeenProcessed(jobID uint, fileHash string) (bool, *db.FileMetadata, error) {
	if fileHash == "" {
		return false, nil, nil
	}

	// First try to find by hash (most reliable)
	metadata, err := mh.db.GetFileMetadataByHash(fileHash) // Calls the interface method
	if err == nil && metadata != nil {
		// Optional: Add logging here if needed
		mh.logger.LogDebug("Found existing metadata by hash for job %d, hash %s", jobID, fileHash)
		return true, metadata, nil
	}
	// Handle DB errors
	if err != nil {
		// If the error is specifically "record not found", it means not processed, which is not an error for this function.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, nil // Not found, no error to return
		}
		// For any other DB error, log it and return it.
		mh.logger.LogError("Error checking metadata by hash for job %d, hash %s: %v", jobID, fileHash, err)
		return false, nil, err // Return the actual DB error
	}

	// Should not be reached if err is nil and metadata is nil, but return false just in case.
	return false, nil, nil
}

// checkFileProcessingHistory checks processing history for a given file name within a specific job.
func (mh *MetadataHandler) checkFileProcessingHistory(jobID uint, fileName string) (*db.FileMetadata, error) {
	// Try to find by job and filename
	metadata, err := mh.db.GetFileMetadataByJobAndName(jobID, fileName) // Calls the interface method
	if err == nil && metadata != nil {
		mh.logger.LogDebug("Found existing metadata by name for job %d, file %s", jobID, fileName)
		return metadata, nil
	}

	if err != nil {
		mh.logger.LogError("Error checking metadata by name for job %d, file %s: %v", jobID, fileName, err)
		// Don't return error here, just indicate not found
	}

	return nil, fmt.Errorf("no history found for file %s in job %d", fileName, jobID)
}
