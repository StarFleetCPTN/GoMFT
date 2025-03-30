package scheduler

import (
	"fmt"

	"github.com/starfleetcptn/gomft/internal/db"
)

// MetadataHandler handles checking file processing history.
type MetadataHandler struct {
	db     *db.DB
	logger *Logger // Added logger dependency
}

// NewMetadataHandler creates a new MetadataHandler.
func NewMetadataHandler(database *db.DB, logger *Logger) *MetadataHandler {
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
	metadata, err := mh.db.GetFileMetadataByHash(fileHash)
	if err == nil && metadata != nil {
		// Optional: Add logging here if needed
		mh.logger.LogDebug("Found existing metadata by hash for job %d, hash %s", jobID, fileHash)
		return true, metadata, nil
	}
	if err != nil {
		mh.logger.LogError("Error checking metadata by hash for job %d, hash %s: %v", jobID, fileHash, err)
	}

	return false, nil, err // Return the error if one occurred during DB lookup
}

// checkFileProcessingHistory checks processing history for a given file name within a specific job.
func (mh *MetadataHandler) checkFileProcessingHistory(jobID uint, fileName string) (*db.FileMetadata, error) {
	// Try to find by job and filename
	metadata, err := mh.db.GetFileMetadataByJobAndName(jobID, fileName)
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
