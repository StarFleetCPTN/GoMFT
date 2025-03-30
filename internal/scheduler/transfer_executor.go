package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
)

// TransferExecutor handles the rclone command execution and transfer logic.
type TransferExecutor struct {
	db              *db.DB
	logger          *Logger
	metadataHandler *MetadataHandler // Placeholder
	notifier        *Notifier        // Placeholder
}

// NewTransferExecutor creates a new TransferExecutor.
func NewTransferExecutor(
	database *db.DB,
	logger *Logger,
	metadata *MetadataHandler,
	notify *Notifier,
) *TransferExecutor {
	return &TransferExecutor{
		db:              database,
		logger:          logger,
		metadataHandler: metadata,
		notifier:        notify,
	}
}

// executeConfigTransfer performs the actual file transfer for a single configuration
func (te *TransferExecutor) executeConfigTransfer(job db.Job, config db.TransferConfig, history *db.JobHistory) {
	te.logger.LogDebug("Starting transfer for config %d with params: %+v", config.ID, config)

	// Track files already processed in this job execution to prevent duplicates
	processedFiles := make(map[string]bool)

	// Get rclone config path
	configPath := te.db.GetConfigRclonePath(&config)

	// Get the command to use for the transfer
	var rcloneCommand string = "copyto" // Default command
	if config.CommandID > 0 {
		// Get the command by ID
		command, err := te.db.GetRcloneCommand(config.CommandID)
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
			if err := te.db.UpdateJobHistory(history); err != nil {
				te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
			}
			// Send notification for failure
			te.notifier.SendNotifications(&job, history, &config) // Call via notifier

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
	listCmd := exec.Command(rclonePath, listArgs...)
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
		if err := te.db.UpdateJobHistory(history); err != nil {
			te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send notification for failure
		te.notifier.SendNotifications(&job, history, &config) // Call via notifier
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
		if err := te.db.UpdateJobHistory(history); err != nil {
			te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send notification for failure
		te.notifier.SendNotifications(&job, history, &config) // Call via notifier
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
		if err := te.db.UpdateJobHistory(history); err != nil {
			te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send notification for empty completion
		te.notifier.SendNotifications(&job, history, &config) // Call via notifier
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
			// Call via metadataHandler
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
		// Call via metadataHandler
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
			cmd := exec.Command(rclonePath, transferArgs...)
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
					archiveCmd := exec.Command(rclonePath, archiveArgs...)
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
					deleteCmd := exec.Command(rclonePath, deleteArgs...)
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

			if err := te.db.CreateFileMetadata(metadata); err != nil {
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

	if err := te.db.UpdateJobHistory(history); err != nil {
		te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
	}

	// Create job notification
	if err := te.notifier.createJobNotification(&job, history); err != nil {
		te.logger.LogError("Failed to create job notification", "jobID", job.ID, "error", err)
	}

	// Send notification for success or with errors
	te.notifier.SendNotifications(&job, history, &config)
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

	// Handle source path with bucket for S3-compatible storage
	if config.SourceType == "s3" || config.SourceType == "minio" || config.SourceType == "b2" {
		sourcePath = fmt.Sprintf("source_%d:%s", config.ID, config.SourceBucket)
		if config.SourcePath != "" && config.SourcePath != "/" {
			sourcePath = fmt.Sprintf("source_%d:%s/%s", config.ID, config.SourceBucket, config.SourcePath)
		}
	} else {
		sourcePath = fmt.Sprintf("source_%d:%s", config.ID, config.SourcePath)
	}

	// Handle destination path with bucket for S3-compatible storage
	if config.DestinationType == "s3" || config.DestinationType == "minio" || config.DestinationType == "b2" {
		destPath = fmt.Sprintf("dest_%d:%s", config.ID, config.DestBucket)
		if config.DestinationPath != "" && config.DestinationPath != "/" {
			destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, config.DestBucket, config.DestinationPath)
		}
	} else {
		destPath = fmt.Sprintf("dest_%d:%s", config.ID, config.DestinationPath)
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

	// Execute the command
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	te.logger.LogDebug("Full command: %s %v", rclonePath, args)
	cmd := exec.Command(rclonePath, args...)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start timer for operation
	startTime := time.Now()

	// Run the command
	err := cmd.Run()

	// Calculate duration
	duration := time.Since(startTime)

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
			// Try to extract transfer statistics from command output
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
	if err := te.db.UpdateJobHistory(history); err != nil {
		te.logger.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
	}

	// Send notification
	te.notifier.SendNotifications(&job, history, &config)
}

// prepareBaseArguments prepares the base arguments for a command
func (te *TransferExecutor) prepareBaseArguments(command string, config *db.TransferConfig, progressCallback func(string)) []string {
	args := []string{command}

	// Add rclone flags from the config
	if config.CommandFlags != "" {
		var flagIDs []uint
		if err := json.Unmarshal([]byte(config.CommandFlags), &flagIDs); err != nil {
			te.logger.LogError("Error parsing command flags: %v", err)
		} else {
			// Get all available flags for this command and their values
			flagsMap, err := te.db.GetRcloneCommandFlagsMap(config.CommandID)
			if err != nil {
				te.logger.LogError("Error getting flags map: %v", err)
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
	configPath := te.db.GetConfigRclonePath(config)
	args = append(args, "--config", configPath)

	// Add progress callback related flags if needed (progressCallback is currently nil)
	if progressCallback != nil {
		args = append(args, "--stats-one-line")
		// Potentially add --json if parsing progress
	} else {
		// Default behavior without callback
		args = append(args, "--stats-one-line") // Keep this for general stats output
	}

	// Consider adding --json only if specifically needed for parsing output later
	// args = append(args, "--json")

	return args
}
