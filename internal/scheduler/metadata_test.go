package scheduler

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/starfleetcptn/gomft/internal/db"
	"gorm.io/gorm" // Keep for gorm.ErrRecordNotFound
)

// --- Mock DB Implementation ---

// Ensure mockMetadataDB implements the MetadataDB interface
var _ MetadataDB = (*mockMetadataDB)(nil)

type mockMetadataDB struct {
	GetFileMetadataByHashFunc       func(hash string) (*db.FileMetadata, error)
	GetFileMetadataByJobAndNameFunc func(jobID uint, fileName string) (*db.FileMetadata, error)
}

// Implement the MetadataDB interface methods
func (m *mockMetadataDB) GetFileMetadataByHash(hash string) (*db.FileMetadata, error) {
	if m.GetFileMetadataByHashFunc != nil {
		return m.GetFileMetadataByHashFunc(hash)
	}
	return nil, errors.New("mock GetFileMetadataByHashFunc not implemented")
}

func (m *mockMetadataDB) GetFileMetadataByJobAndName(jobID uint, fileName string) (*db.FileMetadata, error) {
	if m.GetFileMetadataByJobAndNameFunc != nil {
		return m.GetFileMetadataByJobAndNameFunc(jobID, fileName)
	}
	return nil, errors.New("mock GetFileMetadataByJobAndNameFunc not implemented")
}

// --- Tests ---

func TestHasFileBeenProcessed(t *testing.T) {
	testJobID := uint(1)
	testHash := "testhash123"
	testMetadata := &db.FileMetadata{ID: 1, JobID: testJobID, FileHash: testHash, Status: "processed"}
	dbErr := errors.New("database error")

	logger, _ := newTestLogger(LogLevelDebug) // Use helper from logger_test
	defer logger.Close()

	tests := []struct {
		name           string
		fileHash       string
		mockDBFunc     func(hash string) (*db.FileMetadata, error)
		wantProcessed  bool
		wantMetadata   *db.FileMetadata
		wantErr        error
		wantLogMessage string // Optional: check log output
	}{
		{
			name:          "Empty hash",
			fileHash:      "",
			mockDBFunc:    nil, // Not called
			wantProcessed: false,
			wantMetadata:  nil,
			wantErr:       nil,
		},
		{
			name:     "Hash found",
			fileHash: testHash,
			mockDBFunc: func(hash string) (*db.FileMetadata, error) {
				if hash == testHash {
					return testMetadata, nil
				}
				return nil, gorm.ErrRecordNotFound
			},
			wantProcessed:  true,
			wantMetadata:   testMetadata,
			wantErr:        nil,
			wantLogMessage: "Found existing metadata by hash",
		},
		{
			name:     "Hash not found",
			fileHash: testHash,
			mockDBFunc: func(hash string) (*db.FileMetadata, error) {
				return nil, gorm.ErrRecordNotFound
			},
			wantProcessed: false,
			wantMetadata:  nil,
			wantErr:       nil, // Not found is not an error for this function's return
		},
		{
			name:     "DB error",
			fileHash: testHash,
			mockDBFunc: func(hash string) (*db.FileMetadata, error) {
				return nil, dbErr
			},
			wantProcessed:  false,
			wantMetadata:   nil,
			wantErr:        dbErr, // The DB error should be returned
			wantLogMessage: "Error checking metadata by hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &mockMetadataDB{ // Instantiate the mock implementing the interface
				GetFileMetadataByHashFunc: tt.mockDBFunc,
			}
			// Recreate logger and buffer for each test run to isolate logs
			logger, logBuf := newTestLogger(LogLevelDebug)
			defer logger.Close()

			// Pass the mockDB which now satisfies the MetadataDB interface
			handler := NewMetadataHandler(mockDB, logger)

			processed, metadata, err := handler.hasFileBeenProcessed(testJobID, tt.fileHash)

			if processed != tt.wantProcessed {
				t.Errorf("hasFileBeenProcessed() processed = %v, want %v", processed, tt.wantProcessed)
			}
			if metadata != tt.wantMetadata {
				t.Errorf("hasFileBeenProcessed() metadata = %v, want %v", metadata, tt.wantMetadata)
			}
			if err != tt.wantErr {
				t.Errorf("hasFileBeenProcessed() error = %v, want %v", err, tt.wantErr)
			}

			logOutput := logBuf.String()
			if tt.wantLogMessage != "" && !strings.Contains(logOutput, tt.wantLogMessage) {
				t.Errorf("Expected log message containing %q, but got:\n%s", tt.wantLogMessage, logOutput)
			}
		})
	}
}

func TestCheckFileProcessingHistory(t *testing.T) {
	testJobID := uint(1)
	testFileName := "testfile.txt"
	testMetadata := &db.FileMetadata{ID: 2, JobID: testJobID, FileName: testFileName, Status: "processed"}
	dbErr := errors.New("database error")
	notFoundErr := fmt.Errorf("no history found for file %s in job %d", testFileName, testJobID)

	logger, _ := newTestLogger(LogLevelDebug) // Use helper from logger_test
	defer logger.Close()

	tests := []struct {
		name           string
		jobID          uint
		fileName       string
		mockDBFunc     func(jobID uint, fileName string) (*db.FileMetadata, error)
		wantMetadata   *db.FileMetadata
		wantErr        error  // Check for specific error type/message
		wantLogMessage string // Optional: check log output
	}{
		{
			name:     "History found",
			jobID:    testJobID,
			fileName: testFileName,
			mockDBFunc: func(jobID uint, fileName string) (*db.FileMetadata, error) {
				if jobID == testJobID && fileName == testFileName {
					return testMetadata, nil
				}
				return nil, gorm.ErrRecordNotFound
			},
			wantMetadata:   testMetadata,
			wantErr:        nil,
			wantLogMessage: "Found existing metadata by name",
		},
		{
			name:     "History not found",
			jobID:    testJobID,
			fileName: testFileName,
			mockDBFunc: func(jobID uint, fileName string) (*db.FileMetadata, error) {
				return nil, gorm.ErrRecordNotFound
			},
			wantMetadata: nil,
			wantErr:      notFoundErr, // Expect the specific "no history found" error
		},
		{
			name:     "DB error",
			jobID:    testJobID,
			fileName: testFileName,
			mockDBFunc: func(jobID uint, fileName string) (*db.FileMetadata, error) {
				return nil, dbErr
			},
			wantMetadata:   nil,
			wantErr:        notFoundErr, // Even with DB error, it returns "no history found"
			wantLogMessage: "Error checking metadata by name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &mockMetadataDB{ // Instantiate the mock implementing the interface
				GetFileMetadataByJobAndNameFunc: tt.mockDBFunc,
			}
			// Recreate logger and buffer for each test run
			logger, logBuf := newTestLogger(LogLevelDebug)
			defer logger.Close()

			// Pass the mockDB which now satisfies the MetadataDB interface
			handler := NewMetadataHandler(mockDB, logger)

			metadata, err := handler.checkFileProcessingHistory(tt.jobID, tt.fileName)

			if metadata != tt.wantMetadata {
				t.Errorf("checkFileProcessingHistory() metadata = %v, want %v", metadata, tt.wantMetadata)
			}

			// Check error message specifically for "not found" cases
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("checkFileProcessingHistory() error = nil, want error containing %q", tt.wantErr.Error())
				} else if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("checkFileProcessingHistory() error = %q, want error containing %q", err.Error(), tt.wantErr.Error())
				}
			} else if err != nil {
				t.Errorf("checkFileProcessingHistory() error = %v, want nil", err)
			}

			logOutput := logBuf.String()
			if tt.wantLogMessage != "" && !strings.Contains(logOutput, tt.wantLogMessage) {
				t.Errorf("Expected log message containing %q, but got:\n%s", tt.wantLogMessage, logOutput)
			}
		})
	}
}
