package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec" // Keep this for the variable type definition
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/encryption"
)

// --- Interfaces for Dependencies ---

// TransferDB defines the database methods needed by TransferExecutor.
type TransferDB interface {
	GetConfigRclonePath(config *db.TransferConfig) string
	GetRcloneCommand(id uint) (*db.RcloneCommand, error)
	UpdateJobHistory(history *db.JobHistory) error
	CreateFileMetadata(metadata *db.FileMetadata) error
	GetRcloneCommandFlagsMap(commandID uint) (map[uint]db.RcloneCommandFlag, error)
	GetStorageProvider(id uint) (*db.StorageProvider, error)
}

// TransferNotifier defines the notification methods needed by TransferExecutor.
type TransferNotifier interface {
	SendNotifications(job *db.Job, history *db.JobHistory, config *db.TransferConfig)
	createJobNotification(job *db.Job, history *db.JobHistory) error
}

// TransferMetadataHandler defines the metadata methods needed by TransferExecutor.
type TransferMetadataHandler interface {
	hasFileBeenProcessed(jobID uint, fileHash string) (bool, *db.FileMetadata, error)
	checkFileProcessingHistory(jobID uint, fileName string) (*db.FileMetadata, error)
}

// --- Mockable exec Command ---

// execCommandContext allows mocking exec.CommandContext during tests.
// It's initialized to the real exec.CommandContext function.
var execCommandContext = exec.CommandContext

// --- TransferExecutor Implementation ---

// TransferExecutor handles the rclone command execution and transfer logic.
type TransferExecutor struct {
	db              TransferDB              // Use interface
	logger          *Logger                 // Logger remains concrete
	metadataHandler TransferMetadataHandler // Use interface
	notifier        TransferNotifier        // Use interface
}

// NewTransferExecutor creates a new TransferExecutor.
func NewTransferExecutor(
	database TransferDB, // Accept interface
	logger *Logger,
	metadata TransferMetadataHandler, // Accept interface
	notify TransferNotifier, // Accept interface
) *TransferExecutor {
	return &TransferExecutor{
		db:              database,
		logger:          logger,
		metadataHandler: metadata,
		notifier:        notify,
	}
}

// decryptCredentials securely decrypts credentials for use during transfer operations
// This ensures that sensitive data is only decrypted when needed and never logged
func (te *TransferExecutor) decryptCredentials(config *db.TransferConfig) error {
	var errors []string

	// Get credential encryptor
	credentialEncryptor, err := encryption.GetGlobalCredentialEncryptor()
	if err != nil {
		return fmt.Errorf("failed to get credential encryptor: %v", err)
	}

	// Decrypt source provider credentials if using provider references
	if config.IsUsingSourceProviderReference() && config.SourceProvider != nil {
		provider := config.SourceProvider

		// Decrypt password if present
		if provider.EncryptedPassword != "" {
			password, err := credentialEncryptor.DecryptField(provider.EncryptedPassword)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt source password: %v", err))
			} else {
				// Store decrypted password in memory-only field
				provider.Password = password
			}
		}

		// Decrypt secret key if present (for S3)
		if provider.EncryptedSecretKey != "" {
			secretKey, err := credentialEncryptor.DecryptField(provider.EncryptedSecretKey)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt source secret key: %v", err))
			} else {
				// Store decrypted secret key in memory-only field
				provider.SecretKey = secretKey
			}
		}

		// Decrypt client secret if present (for OAuth)
		if provider.EncryptedClientSecret != "" {
			clientSecret, err := credentialEncryptor.DecryptField(provider.EncryptedClientSecret)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt source client secret: %v", err))
			} else {
				// Store decrypted client secret in memory-only field
				provider.ClientSecret = clientSecret
			}
		}

		// Decrypt refresh token if present (for OAuth)
		if provider.EncryptedRefreshToken != "" {
			refreshToken, err := credentialEncryptor.DecryptField(provider.EncryptedRefreshToken)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt source refresh token: %v", err))
			} else {
				// Store decrypted refresh token in memory-only field
				provider.RefreshToken = refreshToken
			}
		}
	}

	// Decrypt destination provider credentials if using provider references
	if config.IsUsingDestinationProviderReference() && config.DestinationProvider != nil {
		provider := config.DestinationProvider

		// Decrypt password if present
		if provider.EncryptedPassword != "" {
			password, err := credentialEncryptor.DecryptField(provider.EncryptedPassword)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt destination password: %v", err))
			} else {
				// Store decrypted password in memory-only field
				provider.Password = password
			}
		}

		// Decrypt secret key if present (for S3)
		if provider.EncryptedSecretKey != "" {
			secretKey, err := credentialEncryptor.DecryptField(provider.EncryptedSecretKey)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt destination secret key: %v", err))
			} else {
				// Store decrypted secret key in memory-only field
				provider.SecretKey = secretKey
			}
		}

		// Decrypt client secret if present (for OAuth)
		if provider.EncryptedClientSecret != "" {
			clientSecret, err := credentialEncryptor.DecryptField(provider.EncryptedClientSecret)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt destination client secret: %v", err))
			} else {
				// Store decrypted client secret in memory-only field
				provider.ClientSecret = clientSecret
			}
		}

		// Decrypt refresh token if present (for OAuth)
		if provider.EncryptedRefreshToken != "" {
			refreshToken, err := credentialEncryptor.DecryptField(provider.EncryptedRefreshToken)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to decrypt destination refresh token: %v", err))
			} else {
				// Store decrypted refresh token in memory-only field
				provider.RefreshToken = refreshToken
			}
		}
	}

	// If there were any decryption errors, return them combined
	if len(errors) > 0 {
		return fmt.Errorf("credential decryption errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// executeConfigTransfer performs the actual file transfer for a single configuration
func (te *TransferExecutor) executeConfigTransfer(job db.Job, config db.TransferConfig, history *db.JobHistory) {
	te.logger.LogDebug("Starting transfer for config %d with params: %+v", config.ID, config)

	// Track files already processed in this job execution to prevent duplicates
	processedFiles := make(map[string]bool)

	// Get rclone config path
	configPath := te.db.GetConfigRclonePath(&config) // Calls interface method

	// Decrypt credentials if using provider references
	if config.IsUsingSourceProviderReference() || config.IsUsingDestinationProviderReference() {
		if err := te.decryptCredentials(&config); err != nil {
			te.logger.LogError("Failed to decrypt credentials for transfer config %d: %v", config.ID, err)
			history.Status = "failed"
			history.ErrorMessage = fmt.Sprintf("Credential decryption failed: %v", err)
			endTime := time.Now()
			history.EndTime = &endTime
			if updateErr := te.db.UpdateJobHistory(history); updateErr != nil {
				te.logger.LogError("Error updating job history after credential error for job %d, config %d: %v", job.ID, config.ID, updateErr)
			}
			te.notifier.SendNotifications(&job, history, &config)
			return
		}
		te.logger.LogDebug("Successfully decrypted credentials for transfer config %d", config.ID)
	}

	// Get the command to use for the transfer
	var rcloneCommand string = "copyto" // Default command
	if config.CommandID > 0 {
		// Get the command by ID
		command, err := te.db.GetRcloneCommand(config.CommandID) // Calls interface method
		if err == nil && command != nil {
			rcloneCommand = command.Name
			te.logger.LogDebug("Using rclone command %s for job %d, config %d", rcloneCommand, job.ID, config.ID)
		} else {
			te.logger.LogError("Failed to get rclone command with ID %d: %v", config.CommandID, err)
		}
	}

	// Determine command type to handle execution appropriately
	commandType := determineCommandType(rcloneCommand) // Package-level call
	te.logger.LogDebug("Command %s is of type: %s", rcloneCommand, commandType)

	// For non-file-by-file transfer commands, use the simple execution approach
	if commandType != "transfer" || isDirectoryBasedTransfer(rcloneCommand) { // Package-level call
		te.executeSimpleCommand(rcloneCommand, commandType, job, config, history, configPath)
		return
	}

	// The rest of the function handles file-by-file transfer commands (copyto, moveto)
	// Use lsjson to get file list and metadata in one operation instead of separate size and ls commands
	listArgs := []string{
		"--config", configPath,
		"lsjson",
		"--hash",
		"--recursive",
	}

	// Add file pattern filter if specified
	if config.FilePattern != "" && config.FilePattern != "*" {
		// Create a temporary filter file for complex patterns
		filterFile, err := createRcloneFilterFile(config.FilePattern) // Package-level call from utils.go
		if err != nil {
			te.logger.LogError("Error creating filter file for job %d, config %d: %v", job.ID, config.ID, err)
			history.Status = "failed"
			history.ErrorMessage = fmt.Sprintf("Filter Creation Error: %v", err)
			endTime := time.Now()
			history.EndTime = &endTime
			if err := te.db.UpdateJobHistory(history); err != nil { // Calls interface method
				te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
			}
			// Send notification for failure
			te.notifier.SendNotifications(&job, history, &config) // Calls interface method

			return
		}
		defer os.Remove(filterFile)
		listArgs = append(listArgs, "--filter-from", filterFile)
	}

	// Add source path with bucket for S3-compatible storage
	var sourceListPath string
	if config.SourceType == "s3" || config.SourceType == "minio" || config.SourceType == "b2" {
		sourceListPath = fmt.Sprintf("source_%d:%s", config.ID, config.SourceBucket)
		if config.SourcePath != "" && config.SourcePath != "/" {
			sourceListPath = fmt.Sprintf("source_%d:%s/%s", config.ID, config.SourceBucket, config.SourcePath)
		}
	} else {
		sourceListPath = fmt.Sprintf("source_%d:%s", config.ID, config.SourcePath)
	}

	listArgs = append(listArgs, sourceListPath)

	// Execute lsjson command
	te.logger.LogDebug("Full lsjson command: %s %v", os.Getenv("RCLONE_PATH"), listArgs)
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}
	// Use the mockable execCommandContext
	listCmd := execCommandContext(context.Background(), rclonePath, listArgs...)
	listOutput, listErr := listCmd.CombinedOutput()

	// Add debug logging of raw output
	if listErr == nil {
		te.logger.LogDebug("Raw lsjson output for job %d config %d:\n%s",
			job.ID,
			config.ID,
			string(listOutput))
	} else {
		te.logger.LogDebug("Raw lsjson output (error case) for job %d config %d:\n%s",
			job.ID,
			config.ID,
			string(listOutput))
	}

	if listErr != nil {
		te.logger.LogError("Error listing files for job %d, config %d: %v", job.ID, config.ID, listErr)
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("File Listing Error: %v\nOutput: %s", listErr, string(listOutput))
		endTime := time.Now()
		history.EndTime = &endTime
		if err := te.db.UpdateJobHistory(history); err != nil { // Calls interface method
			te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send notification for failure
		te.notifier.SendNotifications(&job, history, &config) // Calls interface method
		return
	}

	// Parse JSON output to get file information
	var fileEntries []map[string]interface{}
	if err := json.Unmarshal(listOutput, &fileEntries); err != nil {
		te.logger.LogError("Error parsing file list JSON for job %d, config %d: %v", job.ID, config.ID, err)
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("JSON Parsing Error: %v", err)
		endTime := time.Now()
		history.EndTime = &endTime
		if err := te.db.UpdateJobHistory(history); err != nil { // Calls interface method
			te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send notification for failure
		te.notifier.SendNotifications(&job, history, &config) // Calls interface method
		return
	}

	// Calculate total size and filter out directories
	var files []map[string]interface{}
	var totalSize int64
	for _, entry := range fileEntries {
		// Process directories
		if isDir, ok := entry["IsDir"].(bool); ok && isDir {
			continue
		}

		// Add to files list
		files = append(files, entry)

		// Add to total size
		if size, ok := entry["Size"].(float64); ok {
			totalSize += int64(size)
		}
	}

	te.logger.LogInfo("Found %d files totaling %d bytes to transfer for job %d, config %d", len(files), totalSize, job.ID, config.ID)

	// Update history with size information
	history.BytesTransferred = totalSize

	if len(files) == 0 {
		te.logger.LogInfo("No files to transfer for job %d, config %d", job.ID, config.ID)
		history.Status = "completed"
		history.ErrorMessage = ""
		history.FilesTransferred = 0
		endTime := time.Now()
		history.EndTime = &endTime
		if err := te.db.UpdateJobHistory(history); err != nil { // Calls interface method
			te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send notification for empty completion
		te.notifier.SendNotifications(&job, history, &config) // Calls interface method
		return
	}

	var transferErrors []string
	filesTransferred := 0

	// Use mutex for thread-safe access to shared variables
	var mutex sync.Mutex

	// Determine number of concurrent transfers
	maxConcurrent := config.MaxConcurrentTransfers
	if maxConcurrent < 1 {
		maxConcurrent = 1 // Default to 1 if not set
	}

	// Limit Google Photos to 1 concurrent transfers
	if config.SourceType == "gphotos" || config.DestinationType == "gphotos" {
		maxConcurrent = 1
	}

	te.logger.LogInfo("Using %d concurrent transfers for job %d, config %d", maxConcurrent, job.ID, config.ID)

	// Create wait group for concurrent processing
	var wg sync.WaitGroup

	// Create channel to limit concurrency
	concurrencySemaphore := make(chan struct{}, maxConcurrent)

	// Process each file individually
	for i, fileEntry := range files {
		fileName, ok := fileEntry["Path"].(string)
		if !ok || fileName == "" {
			continue
		}

		// Skip files that have already been processed in this execution
		if processedFiles[fileName] {
			te.logger.LogDebug("Skipping duplicate file entry: %s (already processed in this execution)", fileName)
			continue
		}

		// Extract hash from the file entry
		fileHash := ""
		if hashes, ok := fileEntry["Hashes"].(map[string]interface{}); ok {
			// Try several hash algorithms in order of preference
			for _, hashType := range []string{"SHA-1", "sha1", "MD5", "md5", "sha256", "crc32"} {
				if hashValue, found := hashes[hashType]; found {
					if hashStr, ok := hashValue.(string); ok && hashStr != "" {
						te.logger.LogDebug("Found hash %s: %s for file %s", hashType, hashStr, fileName)
						fileHash = hashStr
						break
					}
				}
			}
		}

		// Log if no hash was found
		if fileHash == "" {
			te.logger.LogDebug("No hash found for file %s. Available fields: %v", fileName, fileEntry)
		}

		// Extract size from the file entry
		fileSize := int64(0)
		if size, ok := fileEntry["Size"].(float64); ok {
			fileSize = int64(size)
		}

		// Skip files that have already been processed based on hash
		skipFiles := config.GetSkipProcessedFiles()

		if skipFiles && fileHash != "" {
			// Call via metadataHandler interface
			alreadyProcessed, prevMetadata, err := te.metadataHandler.hasFileBeenProcessed(job.ID, fileHash)
			if err == nil && alreadyProcessed {
				te.logger.LogDebug("File %s with hash %s was previously processed on %s with status: %s",
					fileName, fileHash, prevMetadata.ProcessedTime.Format(time.RFC3339), prevMetadata.Status)

				// Determine if we should skip this file based on status
				shouldSkip := false
				if prevMetadata.Status == "processed" ||
					prevMetadata.Status == "archived" ||
					prevMetadata.Status == "deleted" ||
					prevMetadata.Status == "archived_and_deleted" {
					shouldSkip = true
				}

				if shouldSkip {
					te.logger.LogInfo("Skipping unchanged file %s (hash matches previous processing)", fileName)
					continue
				} else {
					te.logger.LogInfo("Re-processing file %s despite previous processing (skipProcessedFiles=%v)", fileName, skipFiles)
				}
			}
		}

		// Also check the processing history for this specific file name
		// Call via metadataHandler interface
		prevMetadata, histErr := te.metadataHandler.checkFileProcessingHistory(job.ID, fileName)
		if histErr == nil {
			te.logger.LogDebug("File %s was previously processed on %s with status: %s",
				fileName, prevMetadata.ProcessedTime.Format(time.RFC3339), prevMetadata.Status)

			// Determine if we should skip this file based on name+hash match
			shouldSkip := false
			if skipFiles && fileHash != "" && fileHash == prevMetadata.FileHash {
				if prevMetadata.Status == "processed" ||
					prevMetadata.Status == "archived" ||
					prevMetadata.Status == "deleted" ||
					prevMetadata.Status == "archived_and_deleted" {
					shouldSkip = true
				}
			}

			if shouldSkip {
				te.logger.LogInfo("Skipping unchanged file %s (hash matches previous processing)", fileName)
				// Skip this file and continue to the next one
				continue
			} else if fileHash != "" && fileHash == prevMetadata.FileHash {
				te.logger.LogInfo("Re-processing file %s despite matching hash (skipProcessedFiles=%v)", fileName, skipFiles)
			}
		}

		// Mark this file as processed for this execution before launching goroutine
		// to prevent duplicate processing
		processedFiles[fileName] = true

		// Add to wait group before starting goroutine
		wg.Add(1)

		// Get creation time and mod time for the file metadata
		createTime := time.Now()
		modTime := time.Now()
		if creationTimeStr, ok := fileEntry["ModTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, creationTimeStr); err == nil {
				modTime = t
				createTime = t
			}
		}

		// Capture current file information for goroutine
		currentFileName := fileName
		currentFileHash := fileHash
		currentFileSize := fileSize
		currentCreateTime := createTime
		currentModTime := modTime

		// Log the file information that will be processed
		te.logger.LogDebug("Processing file %d/%d: %s (Size: %d, Hash: %s)",
			i+1, len(files), currentFileName, currentFileSize, currentFileHash)

		// Start goroutine for concurrent processing
		go func() {
			// Acquire semaphore
			concurrencySemaphore <- struct{}{}
			defer func() {
				// Release semaphore and mark work as done
				<-concurrencySemaphore
				wg.Done()
			}()

			// Prepare rclone command
			transferArgs := te.prepareBaseArguments(rcloneCommand, &config, nil) // Use method call

			// Source and destination paths
			var sourcePath, destPath string

			// For S3, MinIO, and B2, include the bucket in the path
			if config.SourceType == "s3" || config.SourceType == "minio" || config.SourceType == "b2" {
				sourcePath = fmt.Sprintf("source_%d:%s/%s", config.ID, config.SourceBucket, currentFileName)
				if config.SourcePath != "" && config.SourcePath != "/" {
					sourcePath = fmt.Sprintf("source_%d:%s/%s/%s", config.ID, config.SourceBucket, config.SourcePath, currentFileName)
				}
			} else {
				sourcePath = fmt.Sprintf("source_%d:%s/%s", config.ID, config.SourcePath, currentFileName)
			}

			var destFile string = currentFileName

			if config.DestinationType == "s3" || config.DestinationType == "minio" || config.DestinationType == "b2" {
				destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, config.DestBucket, currentFileName)
				if config.DestinationPath != "" && config.DestinationPath != "/" {
					destPath = fmt.Sprintf("dest_%d:%s/%s/%s", config.ID, config.DestBucket, config.DestinationPath, currentFileName)
				}
			} else {
				destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, config.DestinationPath, currentFileName)
			}

			// Add output filename pattern if specified
			if config.OutputPattern != "" {
				// Process the output pattern for this specific file
				destFile = ProcessOutputPattern(config.OutputPattern, currentFileName) // Package-level call from utils.go

				if config.DestinationType == "s3" || config.DestinationType == "minio" || config.DestinationType == "b2" {
					destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, config.DestBucket, destFile)
					if config.DestinationPath != "" && config.DestinationPath != "/" {
						destPath = fmt.Sprintf("dest_%d:%s/%s/%s", config.ID, config.DestBucket, config.DestinationPath, destFile)
					}
				} else {
					destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, config.DestinationPath, destFile)
				}

				te.logger.LogDebug("Renaming file from %s to %s for job %d, config %d", currentFileName, destFile, job.ID, config.ID)
			}

			// Add source and destination to the command (already added in prepareBaseArguments for some commands, check logic)
			// This part needs careful review based on how prepareBaseArguments is structured
			// For file-by-file (copyto, moveto), we need source and dest here.
			transferArgs = append(transferArgs, sourcePath, destPath)

			// Execute transfer for this file
			te.logger.LogDebug("Full transfer command: %s %v", rclonePath, transferArgs)
			te.logger.LogDebug("Environment: RCLONE_PATH=%s", os.Getenv("RCLONE_PATH"))
			// Use the mockable execCommandContext
			cmd := execCommandContext(context.Background(), rclonePath, transferArgs...)
			fileOutput, fileErr := cmd.CombinedOutput()

			// Print the output
			te.logger.LogDebug("Output for file %s: %s", currentFileName, string(fileOutput))

			// Create file metadata record
			fileStatus := "processed"
			var fileErrorMsg string
			var destPathForDB string

			// Check if file was successfully transferred
			if fileErr != nil {
				te.logger.LogError("Error transferring file %s for job %d, config %d: %v", currentFileName, job.ID, config.ID, fileErr)
				mutex.Lock()
				transferErrors = append(transferErrors, fmt.Sprintf("File %s: %v", currentFileName, fileErr))
				mutex.Unlock()
				fileStatus = "error"
				fileErrorMsg = fileErr.Error()
			} else {
				mutex.Lock()
				filesTransferred++
				mutex.Unlock()
				te.logger.LogInfo("Successfully transferred file %s for job %d, config %d", currentFileName, job.ID, config.ID)

				// Extract the actual destination path (without rclone remote prefix)
				if config.DestinationType == "local" {
					destPathForDB = filepath.Join(config.DestinationPath, destFile)
				} else {
					// For remote destinations, store the path format
					if config.DestinationType == "s3" || config.DestinationType == "minio" || config.DestinationType == "b2" {
						if config.DestinationPath != "" && config.DestinationPath != "/" {
							destPathForDB = fmt.Sprintf("%s/%s/%s", config.DestBucket, config.DestinationPath, destFile)
						} else {
							destPathForDB = fmt.Sprintf("%s/%s", config.DestBucket, destFile)
						}
					} else {
						destPathForDB = fmt.Sprintf("%s/%s", config.DestinationPath, destFile)
					}
				}

				// If archiving is enabled and transfer was successful, move files to archive
				if config.GetArchiveEnabled() && config.ArchivePath != "" {
					te.logger.LogInfo("Archiving file %s for job %d, config %d", currentFileName, job.ID, config.ID)

					// We don't need to move the file since we used moveto, but we can copy it to archive
					archiveArgs := []string{
						"--config", configPath,
						"copyto",
						sourcePath,
					}

					// Construct archive path with bucket if needed
					var archiveDest string
					if config.SourceType == "s3" || config.SourceType == "minio" || config.SourceType == "b2" {
						archiveDest = fmt.Sprintf("source_%d:%s/%s/%s", config.ID, config.SourceBucket, config.ArchivePath, currentFileName)
					} else {
						archiveDest = fmt.Sprintf("source_%d:%s/%s", config.ID, config.ArchivePath, currentFileName)
					}

					archiveArgs = append(archiveArgs, archiveDest)

					te.logger.LogInfo("Executing rclone archive command for job %d, config %d, file %s: rclone %s",
						job.ID, config.ID, currentFileName, strings.Join(archiveArgs, " "))
					// Get the rclone path from the environment variable or use the default path
					rclonePath := os.Getenv("RCLONE_PATH")
					if rclonePath == "" {
						rclonePath = "rclone"
					}
					// Use the mockable execCommandContext
					archiveCmd := execCommandContext(context.Background(), rclonePath, archiveArgs...)
					archiveOutput, archiveErr := archiveCmd.CombinedOutput()

					// Print the output
					te.logger.LogDebug("Output for file %s: %s", currentFileName, string(archiveOutput))

					// Check if file was successfully transferred
					if archiveErr != nil {
						te.logger.LogError("Warning: Error archiving file %s for job %d, config %d: %v", currentFileName, job.ID, config.ID, archiveErr)
						mutex.Lock()
						transferErrors = append(transferErrors,
							fmt.Sprintf("Archive error for file %s: %v", currentFileName, archiveErr))
						mutex.Unlock()
					} else {
						fileStatus = "archived"
					}
				}

				if config.GetDeleteAfterTransfer() {
					te.logger.LogInfo("Deleting file %s for job %d, config %d", currentFileName, job.ID, config.ID)
					deleteArgs := []string{
						"--config", configPath,
						"deletefile",
						sourcePath}
					// Use the mockable execCommandContext
					deleteCmd := execCommandContext(context.Background(), rclonePath, deleteArgs...)
					deleteOutput, deleteErr := deleteCmd.CombinedOutput()
					te.logger.LogDebug("Output for file %s: %s", currentFileName, string(deleteOutput))
					if deleteErr != nil {
						te.logger.LogError("Error deleting file %s for job %d, config %d: %v", currentFileName, job.ID, config.ID, deleteErr)
						mutex.Lock()
						transferErrors = append(transferErrors,
							fmt.Sprintf("Delete error for file %s: %v", currentFileName, deleteErr))
						mutex.Unlock()
					} else {
						if fileStatus == "archived" {
							fileStatus = "archived_and_deleted"
						} else {
							fileStatus = "deleted"
						}
					}
				}
			}

			// Create and save file metadata
			metadata := &db.FileMetadata{
				JobID:           job.ID,
				ConfigID:        config.ID,
				FileName:        currentFileName,
				OriginalPath:    config.SourcePath,
				FileSize:        currentFileSize,
				FileHash:        currentFileHash,
				CreationTime:    currentCreateTime,
				ModTime:         currentModTime,
				ProcessedTime:   time.Now(),
				DestinationPath: destPathForDB,
				Status:          fileStatus,
				ErrorMessage:    fileErrorMsg,
			}

			if err := te.db.CreateFileMetadata(metadata); err != nil { // Calls interface method
				te.logger.LogError("Error creating file metadata for %s: %v", currentFileName, err)
			} else {
				te.logger.LogDebug("Created file metadata record for %s (ID: %d) with hash: %s", currentFileName, metadata.ID, currentFileHash)
			}
		}()
	}

	// Wait for all transfers to complete
	wg.Wait()

	// Clean up concurrency semaphore
	close(concurrencySemaphore)

	// Update job history with transfer results
	history.FilesTransferred = filesTransferred

	if len(transferErrors) > 0 {
		history.Status = "completed_with_errors"
		history.ErrorMessage = fmt.Sprintf("Transfer completed with %d errors:\n%s",
			len(transferErrors), strings.Join(transferErrors, "\n"))
	} else {
		history.Status = "completed"
	}

	// Update job history with completion status and end time
	endTime := time.Now()
	history.EndTime = &endTime

	if err := te.db.UpdateJobHistory(history); err != nil { // Calls interface method
		te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
	}

	// Create job notification
	if err := te.notifier.createJobNotification(&job, history); err != nil { // Calls interface method
		te.logger.LogError("Failed to create job notification: jobID=%d, error=%v", job.ID, err)
	}

	// Send notification for success or with errors
	te.notifier.SendNotifications(&job, history, &config) // Calls interface method
}

// isDirectoryBasedTransfer checks if a transfer command operates on directories rather than individual files
func isDirectoryBasedTransfer(commandName string) bool {
	// These commands operate on entire directories, not file-by-file
	dirBasedCommands := map[string]bool{
		"sync":   true,
		"bisync": true,
		"copy":   true,
		"move":   true,
	}

	return dirBasedCommands[commandName]
}

// determineCommandType categorizes rclone commands into types for execution
func determineCommandType(commandName string) string {
	// File transfer commands
	transferCommands := map[string]bool{
		"copy":   true,
		"copyto": true,
		"move":   true,
		"moveto": true,
		"sync":   true,
		"bisync": true,
	}

	// Listing commands
	listingCommands := map[string]bool{
		"ls":          true,
		"lsd":         true,
		"lsl":         true,
		"lsf":         true,
		"lsjson":      true,
		"listremotes": true,
	}

	// Information commands
	infoCommands := map[string]bool{
		"md5sum":  true,
		"sha1sum": true,
		"size":    true,
		"version": true,
	}

	// Directory operations
	dirCommands := map[string]bool{
		"mkdir":  true,
		"rmdir":  true,
		"rmdirs": true,
	}

	// Destructive commands
	destructiveCommands := map[string]bool{
		"delete": true,
		"purge":  true,
	}

	// Maintenance commands
	maintenanceCommands := map[string]bool{
		"cleanup": true,
		"dedupe":  true,
		"check":   true,
	}

	// Specialized commands
	specialCommands := map[string]bool{
		"obscure":    true,
		"cryptcheck": true,
	}

	// Determine the command type
	if transferCommands[commandName] {
		return "transfer"
	} else if listingCommands[commandName] {
		return "listing"
	} else if infoCommands[commandName] {
		return "info"
	} else if dirCommands[commandName] {
		return "directory"
	} else if destructiveCommands[commandName] {
		return "destructive"
	} else if maintenanceCommands[commandName] {
		return "maintenance"
	} else if specialCommands[commandName] {
		return "special"
	}

	// Default to transfer if unknown
	return "transfer"
}

// executeSimpleCommand executes a simple command (non file-by-file transfer)
func (te *TransferExecutor) executeSimpleCommand(cmdName string, cmdType string, job db.Job, config db.TransferConfig, history *db.JobHistory, configPath string) {
	te.logger.LogInfo("Executing simple command '%s' of type '%s' for job %d, config %d", cmdName, cmdType, job.ID, config.ID)

	// Prepare base arguments
	baseArgs := te.prepareBaseArguments(cmdName, &config, nil) // Use method call

	// Prepare source and destination paths
	var sourcePath, destPath string

	// Determine correct source bucket/path based on provider or direct configuration
	var sourceBucket, sourceBasePath string
	if config.IsUsingSourceProviderReference() && config.SourceProvider != nil {
		sourceBucket = config.SourceProvider.Bucket
		sourceBasePath = config.SourcePath // Still use config's path for the specific location
	} else {
		sourceBucket = config.SourceBucket
		sourceBasePath = config.SourcePath
	}

	// Get the effective source type (from provider if using reference, otherwise from config)
	sourceType := te.getEffectiveSourceType(&config)

	// Handle source path with bucket for S3-compatible storage
	if sourceType == "s3" || sourceType == "minio" || sourceType == "b2" {
		sourcePath = fmt.Sprintf("source_%d:%s", config.ID, sourceBucket)
		if sourceBasePath != "" && sourceBasePath != "/" {
			sourcePath = fmt.Sprintf("source_%d:%s/%s", config.ID, sourceBucket, sourceBasePath)
		}
	} else {
		sourcePath = fmt.Sprintf("source_%d:%s", config.ID, sourceBasePath)
	}

	// Determine correct destination bucket/path based on provider or direct configuration
	var destBucket, destBasePath string
	if config.IsUsingDestinationProviderReference() && config.DestinationProvider != nil {
		destBucket = config.DestinationProvider.Bucket
		destBasePath = config.DestinationPath // Still use config's path for the specific location
	} else {
		destBucket = config.DestBucket
		destBasePath = config.DestinationPath
	}

	// Get the effective destination type (from provider if using reference, otherwise from config)
	destType := te.getEffectiveDestinationType(&config)

	// Handle destination path with bucket for S3-compatible storage
	if destType == "s3" || destType == "minio" || destType == "b2" {
		destPath = fmt.Sprintf("dest_%d:%s", config.ID, destBucket)
		if destBasePath != "" && destBasePath != "/" {
			destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, destBucket, destBasePath)
		}
	} else {
		destPath = fmt.Sprintf("dest_%d:%s", config.ID, destBasePath)
	}

	// Add appropriate paths based on command type
	args := baseArgs // Start with base args prepared by prepareBaseArguments
	switch cmdType {
	case "transfer":
		// Directory-based transfers and file-specific transfers handled here
		args = append(args, sourcePath, destPath)
	case "maintenance":
		// Check command needs both source and destination, others may just need source
		if cmdName == "check" {
			args = append(args, sourcePath, destPath)
		} else {
			args = append(args, sourcePath)
		}
	case "listing":
		// Listing commands only need source path
		args = append(args, sourcePath)
	case "info":
		// Info commands typically need only source path
		args = append(args, sourcePath)
	case "directory":
		// Directory operations might need one or both paths depending on operation
		if cmdName == "rmdirs" && strings.Contains(config.RcloneFlags, "--dst") {
			// Special case: rmdirs with --dst flag needs both paths
			args = append(args, sourcePath, destPath)
		} else {
			// Default case: just source path
			args = append(args, sourcePath)
		}
	case "destructive":
		// Destructive commands only need source path
		args = append(args, sourcePath)
	case "special":
		// Special commands handled case by case
		if cmdName == "cryptcheck" {
			args = append(args, sourcePath, destPath)
		} else if cmdName == "obscure" || cmdName == "version" || cmdName == "listremotes" {
			// These commands don't need paths at all
		} else {
			args = append(args, sourcePath)
		}
	default:
		// Default to source path only
		args = append(args, sourcePath)
	}

	// Create a temporary file for rclone logs
	tempLogFile, err := os.CreateTemp("", "rclone-log-*.txt")
	if err != nil {
		te.logger.LogError("Error creating temporary log file for job %d, config %d: %v", job.ID, config.ID, err)
		// Update history and return if log file creation fails
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("Log File Creation Error: %v", err)
		endTime := time.Now()
		history.EndTime = &endTime
		if updateErr := te.db.UpdateJobHistory(history); updateErr != nil {
			te.logger.LogError("Error updating job history after log file error for job %d, config %d: %v", job.ID, config.ID, updateErr)
		}
		te.notifier.SendNotifications(&job, history, &config)
		return
	}
	defer os.Remove(tempLogFile.Name()) // Ensure cleanup

	// Add logging flags to arguments
	args = append(args, "--log-file", tempLogFile.Name(), "--log-level", "DEBUG")

	// Execute the command
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	te.logger.LogDebug("Full command: %s %v", rclonePath, args)
	// Use the mockable execCommandContext
	cmd := execCommandContext(context.Background(), rclonePath, args...)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start timer for operation
	startTime := time.Now()

	// Run the command
	err = cmd.Run() // This will use the mocked command if execCommandContext is replaced

	// Calculate duration
	duration := time.Since(startTime)

	var filesProcessedFromLog int = 0 // Declare counter for files processed based on log

	// Read the rclone log file content
	logContent, logReadErr := os.ReadFile(tempLogFile.Name())
	if logReadErr != nil {
		te.logger.LogError("Error reading rclone log file %s for job %d, config %d: %v", tempLogFile.Name(), job.ID, config.ID, logReadErr)
		// Proceed without log content, but log the error
	} else {
		te.logger.LogDebug("Rclone log content for job %d, config %d:\n%s", job.ID, config.ID, string(logContent))

		// --- Start Log Parsing for FileMetadata ---
		if cmdType == "transfer" && logReadErr == nil { // Only parse for transfer commands if log was read
			logLines := strings.Split(string(logContent), "\n")

			// --- First Pass: Extract Hashes ---
			// Regex to find lines like: "DEBUG : filename.txt: md5 = hashvalue OK"
			// Captures filename (group 1) and hash value (group 2)
			hashLogRegex := regexp.MustCompile(`DEBUG\s*:\s*(.*?):\s*(?:md5|sha1)\s*=\s*([a-f0-9]+)\s*OK`)
			fileHashMap := make(map[string]string)
			for _, line := range logLines {
				matches := hashLogRegex.FindStringSubmatch(line)
				if len(matches) >= 3 {
					fileName := strings.TrimSpace(matches[1])
					hashValue := strings.TrimSpace(matches[2])
					if fileName != "" && hashValue != "" {
						fileHashMap[fileName] = hashValue
						te.logger.LogDebug("Extracted hash for %s: %s", fileName, hashValue)
					}
				}
			}
			// --- End Hash Extraction ---

			// --- Second Pass: Process Copied Files and Create Metadata ---
			// Regex to find lines like: "INFO : path/to/file.txt: Copied (new)" or "INFO : path/to/file.txt: Copied (replaced existing)"
			// It captures the filename (group 1)
			copyLogRegex := regexp.MustCompile(`INFO\s*:\s*(.*?):\s*Copied`)
			processedFilesInLog := make(map[string]bool) // Track files found in log to avoid duplicates

			for _, line := range logLines {
				matches := copyLogRegex.FindStringSubmatch(line)
				if len(matches) >= 2 {
					fileName := strings.TrimSpace(matches[1])
					if fileName == "" || processedFilesInLog[fileName] {
						continue // Skip empty or duplicate filenames within the log
					}
					processedFilesInLog[fileName] = true // Mark as processed in this log

					// Construct destination path for metadata
					var destPathForDB string
					destFile := fileName // Assume filename is the same unless output pattern is used (not handled here)
					if config.DestinationType == "local" {
						destPathForDB = filepath.Join(config.DestinationPath, destFile)
					} else if config.DestinationType == "s3" || config.DestinationType == "minio" || config.DestinationType == "b2" {
						if config.DestinationPath != "" && config.DestinationPath != "/" {
							destPathForDB = fmt.Sprintf("%s/%s/%s", config.DestBucket, config.DestinationPath, destFile)
						} else {
							destPathForDB = fmt.Sprintf("%s/%s", config.DestBucket, destFile)
						}
					} else {
						destPathForDB = fmt.Sprintf("%s/%s", config.DestinationPath, destFile)
					}

					// Get hash from the map
					fileHash := fileHashMap[fileName] // Will be empty string if not found

					// Create FileMetadata record
					now := time.Now()
					metadata := &db.FileMetadata{
						JobID:           job.ID,
						ConfigID:        config.ID,
						FileName:        fileName,
						OriginalPath:    config.SourcePath, // Base source path
						FileSize:        0,                 // Unknown from log
						FileHash:        fileHash,          // Use extracted hash
						CreationTime:    now,               // Approximation
						ModTime:         now,               // Approximation
						ProcessedTime:   now,
						DestinationPath: destPathForDB,
						Status:          "processed", // Assumed success based on log line
						ErrorMessage:    "",
					}

					if err := te.db.CreateFileMetadata(metadata); err != nil {
						te.logger.LogError("Error creating file metadata from log for %s: %v", fileName, err)
						// Don't stop processing other files
					} else {
						filesProcessedFromLog++
						te.logger.LogDebug("Created file metadata from log for %s (ID: %d, Hash: %s)", fileName, metadata.ID, fileHash)
					}
				}
			}
			te.logger.LogInfo("Processed %d files based on rclone log for job %d, config %d", filesProcessedFromLog, job.ID, config.ID)
			// --- End Metadata Creation ---
		}
		// --- End Log Parsing ---

	}

	// Update history with basic info
	history.EndTime = &time.Time{}
	*history.EndTime = startTime.Add(duration)

	// Check for pattern in stderr that indicates successful completion with warnings
	// Some commands like sync may complete successfully but with warnings
	successWithWarnings := strings.Contains(stderr.String(), "Transferred:") &&
		strings.Contains(stderr.String(), "Errors:") &&
		strings.Contains(stderr.String(), "Checks:")

	// Process results
	if err != nil && !successWithWarnings {
		te.logger.LogError("Error executing command '%s' for job %d, config %d: %v", cmdName, job.ID, config.ID, err)
		te.logger.LogError("Command stderr: %s", stderr.String())

		history.Status = "failed"
		// Use stderr directly from the buffer as the error from Run() might not contain it
		history.ErrorMessage = fmt.Sprintf("Command Error: %v\nStderr: %s", err, stderr.String())
	} else {
		te.logger.LogInfo("Successfully executed command '%s' for job %d, config %d (duration: %v)",
			cmdName, job.ID, config.ID, duration)

		// Handle different command output types
		if cmdType == "listing" {
			// For listing commands, count the number of lines in the output as "files processed"
			lines := strings.Count(stdout.String(), "\n")
			history.FilesTransferred = lines
			history.Status = "completed"
		} else if cmdType == "transfer" {
			// Try to extract transfer statistics from command output (stderr)
			history.Status = "completed"

			// Look for metrics in stderr which is where rclone puts stats
			// Extract bytes transferred if available
			bytesRegex := regexp.MustCompile(`Transferred:\s+(\d+)\s+/\s+(\d+)\s+Bytes`)
			if matches := bytesRegex.FindStringSubmatch(stderr.String()); len(matches) >= 3 {
				if bytesTransferred, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					history.BytesTransferred = bytesTransferred
				}
			}

			// Extract files transferred if available
			filesRegex := regexp.MustCompile(`Transferred:\s+(\d+)\s+/\s+(\d+)\s+Files`)
			if matches := filesRegex.FindStringSubmatch(stderr.String()); len(matches) >= 3 {
				if filesTransferred, err := strconv.Atoi(matches[1]); err == nil {
					history.FilesTransferred = filesTransferred
				}
			}

			// If stats weren't found in stderr OR the log parsing yielded a count, use the log count
			// This prioritizes the count derived from actual file processing logs.
			if filesProcessedFromLog > 0 {
				history.FilesTransferred = filesProcessedFromLog
				te.logger.LogDebug("Updated FilesTransferred count to %d based on log parsing", filesProcessedFromLog)
			} else if history.FilesTransferred == 0 && logReadErr == nil {
				// Fallback message if stats were 0 and log parsing also yielded 0 (or wasn't applicable)
				te.logger.LogDebug("Could not determine FilesTransferred from stderr stats or log parsing.")
			}

		} else {
			// For other commands, we don't have file counts, but the command completed
			history.Status = "completed"
		}

		// Store command output in the history for reference
		if cmdType == "listing" || cmdType == "info" {
			// For listing and info commands, the output is the result
			// Limit to first 1000 characters to avoid huge entries
			output := stdout.String()
			if len(output) > 1000 {
				output = output[:997] + "..."
			}
			history.ErrorMessage = fmt.Sprintf("Command Output:\n%s", output)
		}
	}

	// Update job history in the database
	if err := te.db.UpdateJobHistory(history); err != nil { // Calls interface method
		te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
	}

	// Send notification
	te.notifier.SendNotifications(&job, history, &config) // Calls interface method
}

// prepareBaseArguments prepares rclone command arguments
func (te *TransferExecutor) prepareBaseArguments(command string, config *db.TransferConfig, progressCallback interface{}) []string {
	args := []string{command}

	// Add rclone flags from the config
	if config.CommandFlags != "" {
		var flagIDs []uint
		if err := json.Unmarshal([]byte(config.CommandFlags), &flagIDs); err != nil {
			te.logger.LogError("Error parsing command flags JSON: %v", err) // Corrected format
		} else {
			// Get all available flags for this command and their values
			flagsMap, err := te.db.GetRcloneCommandFlagsMap(config.CommandID) // Calls interface method
			if err != nil {
				te.logger.LogError("Error getting flags map for command %d: %v", config.CommandID, err) // Added context
			} else {
				// Parse flag values if available
				var flagValues map[uint]string
				if config.CommandFlagValues != "" {
					if err := json.Unmarshal([]byte(config.CommandFlagValues), &flagValues); err != nil {
						te.logger.LogError("Error parsing flag values: %v", err)
					}
				}

				// Add each selected flag
				for _, flagID := range flagIDs {
					if flag, ok := flagsMap[flagID]; ok {
						if flag.DataType == "bool" {
							// Boolean flags don't have values
							args = append(args, "--"+flag.Name) // Prepend -- for rclone flags
						} else if flagValues != nil {
							// Check if we have a value for this flag
							if value, ok := flagValues[flagID]; ok && value != "" {
								args = append(args, "--"+flag.Name, value) // Prepend --
							} else {
								// If there's a default value, use it
								if flag.DefaultValue != "" {
									args = append(args, "--"+flag.Name, flag.DefaultValue) // Prepend --
								} else {
									// Skip flags without values
									te.logger.LogError("Skipping flag %s: no value provided", flag.Name)
								}
							}
						} else if flag.DefaultValue != "" { // Handle case where flagValues is nil but default exists
							args = append(args, "--"+flag.Name, flag.DefaultValue) // Prepend --
						}
					}
				}
			}
		}
	}

	// Add any additional rclone flags specified by the user
	if config.RcloneFlags != "" {
		additionalFlags := strings.Fields(config.RcloneFlags)
		args = append(args, additionalFlags...)
	}

	// Add common rclone options
	args = append(args, "--progress")
	args = append(args, "--stats", "1s")

	// Add config file location
	configPath := te.db.GetConfigRclonePath(config) // Calls interface method
	args = append(args, "--config", configPath)

	// Add progress callback related flags if needed (progressCallback is currently nil)
	if progressCallback != nil {
		args = append(args, "--stats-one-line")
		// Potentially add --json if parsing progress
	} else {
		// Default behavior without callback
		args = append(args, "--stats-one-line") // Keep this for general stats output
	}

	// Ensure providers are fully loaded if using references
	if config.IsUsingSourceProviderReference() && config.SourceProvider == nil {
		provider, err := te.db.GetStorageProvider(*config.SourceProviderID)
		if err != nil {
			te.logger.LogError("Failed to load source provider (ID %d): %v", *config.SourceProviderID, err)
		} else {
			config.SourceProvider = provider
			te.logger.LogDebug("Loaded source provider (ID %d): %s", provider.ID, provider.Name)
		}
	}

	if config.IsUsingDestinationProviderReference() && config.DestinationProvider == nil {
		provider, err := te.db.GetStorageProvider(*config.DestinationProviderID)
		if err != nil {
			te.logger.LogError("Failed to load destination provider (ID %d): %v", *config.DestinationProviderID, err)
		} else {
			config.DestinationProvider = provider
			te.logger.LogDebug("Loaded destination provider (ID %d): %s", provider.ID, provider.Name)
		}
	}

	return args
}

// getEffectiveSourceType returns the effective source type based on provider or direct configuration
func (te *TransferExecutor) getEffectiveSourceType(config *db.TransferConfig) string {
	if config.IsUsingSourceProviderReference() && config.SourceProvider != nil {
		return string(config.SourceProvider.Type)
	}
	return config.SourceType
}

// getEffectiveDestinationType returns the effective destination type based on provider or direct configuration
func (te *TransferExecutor) getEffectiveDestinationType(config *db.TransferConfig) string {
	if config.IsUsingDestinationProviderReference() && config.DestinationProvider != nil {
		return string(config.DestinationProvider.Type)
	}
	return config.DestinationType
}
