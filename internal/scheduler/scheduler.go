package scheduler

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/starfleetcptn/gomft/internal/db"
)

type Scheduler struct {
	cron     *cron.Cron
	db       *db.DB
	jobMutex sync.Mutex
	jobs     map[uint]cron.EntryID
}

func New(database *db.DB) *Scheduler {
	scheduler := &Scheduler{
		cron: cron.New(cron.WithSeconds()),
		db:   database,
		jobs: make(map[uint]cron.EntryID),
	}

	// Start the cron scheduler
	scheduler.cron.Start()

	// Load existing jobs from database
	scheduler.loadJobs()

	return scheduler
}

func (s *Scheduler) loadJobs() {
	var jobs []db.Job
	if err := s.db.Preload("Config").Find(&jobs).Error; err != nil {
		fmt.Printf("Error loading jobs: %v\n", err)
		return
	}

	fmt.Printf("Loading %d jobs from database\n", len(jobs))
	for _, job := range jobs {
		if job.Enabled {
			if err := s.ScheduleJob(&job); err != nil {
				fmt.Printf("Error scheduling job %d: %v\n", job.ID, err)
				continue
			}
			fmt.Printf("Scheduled job %d with cron expression: %s\n", job.ID, job.Schedule)
		}
	}
}

func (s *Scheduler) ScheduleJob(job *db.Job) error {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	fmt.Printf("Scheduling job %d (enabled: %v, schedule: %s)\n", job.ID, job.Enabled, job.Schedule)

	// Remove existing job if it exists
	if entryID, exists := s.jobs[job.ID]; exists {
		fmt.Printf("Removing existing schedule for job %d\n", job.ID)
		s.cron.Remove(entryID)
		delete(s.jobs, job.ID)
	}

	// Only schedule if job is enabled
	if !job.Enabled {
		fmt.Printf("Job %d is disabled, skipping scheduling\n", job.ID)
		return nil
	}

	// Convert 5-field cron to 6-field by prepending '0' for seconds
	schedule := job.Schedule
	if len(strings.Fields(schedule)) == 5 {
		schedule = "0 " + schedule
	}

	// Validate cron expression
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	if err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", job.Schedule, err)
	}

	// Schedule new job
	entryID, err := s.cron.AddFunc(schedule, func() {
		fmt.Printf("Executing job %d at %s\n", job.ID, time.Now().Format(time.RFC3339))
		s.executeJob(job.ID)
	})
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	s.jobs[job.ID] = entryID
	fmt.Printf("Successfully scheduled job %d with entry ID %v\n", job.ID, entryID)

	// Calculate and log next run time
	if entry := s.cron.Entry(entryID); entry.ID != 0 {
		fmt.Printf("Next run time for job %d: %s\n", job.ID, entry.Next.Format(time.RFC3339))
	}

	return nil
}

func (s *Scheduler) executeJob(jobID uint) {
	fmt.Printf("Starting execution of job %d\n", jobID)

	// Get job details
	var job db.Job
	if err := s.db.Preload("Config").First(&job, jobID).Error; err != nil {
		fmt.Printf("Error loading job %d: %v\n", jobID, err)
		return
	}

	if job.Config.ID == 0 {
		fmt.Printf("Error: job %d has no associated config\n", jobID)
		return
	}

	// Add explicit database reload of the config to ensure we have the latest values
	var config db.TransferConfig
	if err := s.db.First(&config, job.Config.ID).Error; err != nil {
		fmt.Printf("Error loading config %d: %v\n", job.Config.ID, err)
		return
	}
	// Replace the job's config with the freshly loaded one
	job.Config = config

	// Now the rest of your code will use the correct value
	fmt.Printf("Loaded job %d with config: source=%s:%s, dest=%s:%s, skipProcessedFiles=%v\n",
		jobID,
		job.Config.SourceType,
		job.Config.SourcePath,
		job.Config.DestinationType,
		job.Config.DestinationPath,
		job.Config.SkipProcessedFiles,
	)

	// Create job history entry
	startTime := time.Now()
	history := &db.JobHistory{
		JobID:            jobID,
		StartTime:        startTime,
		Status:           "running",
		FilesTransferred: 0,
		BytesTransferred: 0,
		ErrorMessage:     "",
	}
	if err := s.db.CreateJobHistory(history); err != nil {
		fmt.Printf("Error creating job history for job %d: %v\n", jobID, err)
		return
	}

	// Update job last run time
	job.LastRun = &history.StartTime
	if err := s.db.UpdateJobStatus(&job); err != nil {
		fmt.Printf("Error updating job last run time for job %d: %v\n", jobID, err)
	}

	// Reload the job from the database to get the latest values
	if err := s.db.Preload("Config").First(&job, jobID).Error; err != nil {
		fmt.Printf("Error reloading job %d: %v\n", jobID, err)
		return
	}

	// Track files already processed in this job execution to prevent duplicates
	processedFiles := make(map[string]bool)

	// Get rclone config path
	configPath := s.db.GetConfigRclonePath(&job.Config)

	// Use lsjson to get file list and metadata in one operation instead of separate size and ls commands
	listArgs := []string{
		"--config", configPath,
		"lsjson",
		"--hash",
		"--recursive",
	}

	// Add file pattern filter if specified
	if job.Config.FilePattern != "" && job.Config.FilePattern != "*" {
		// Create a temporary filter file for complex patterns
		filterFile, err := createRcloneFilterFile(job.Config.FilePattern)
		if err != nil {
			fmt.Printf("Error creating filter file for job %d: %v\n", jobID, err)
			history.Status = "failed"
			history.ErrorMessage = fmt.Sprintf("Filter Creation Error: %v", err)
			endTime := time.Now()
			history.EndTime = &endTime
			if err := s.db.UpdateJobHistory(history); err != nil {
				fmt.Printf("Error updating job history for job %d: %v\n", jobID, err)
			}
			return
		}
		defer os.Remove(filterFile)
		listArgs = append(listArgs, "--filter-from", filterFile)
	}

	// Add source path with bucket for S3-compatible storage
	var sourceListPath string
	if job.Config.SourceType == "s3" || job.Config.SourceType == "minio" || job.Config.SourceType == "b2" {
		sourceListPath = fmt.Sprintf("source_%d:%s", job.Config.ID, job.Config.SourceBucket)
		if job.Config.SourcePath != "" && job.Config.SourcePath != "/" {
			sourceListPath = fmt.Sprintf("source_%d:%s/%s", job.Config.ID, job.Config.SourceBucket, job.Config.SourcePath)
		}
	} else {
		sourceListPath = fmt.Sprintf("source_%d:%s", job.Config.ID, job.Config.SourcePath)
	}

	listArgs = append(listArgs, sourceListPath)

	// Execute lsjson command
	fmt.Printf("Listing files with metadata for job %d: rclone %s\n", jobID, strings.Join(listArgs, " "))
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}
	listCmd := exec.Command(rclonePath, listArgs...)
	listOutput, listErr := listCmd.CombinedOutput()

	if listErr != nil {
		fmt.Printf("Error listing files for job %d: %v\n", jobID, listErr)
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("File Listing Error: %v\nOutput: %s", listErr, string(listOutput))
		endTime := time.Now()
		history.EndTime = &endTime
		if err := s.db.UpdateJobHistory(history); err != nil {
			fmt.Printf("Error updating job history for job %d: %v\n", jobID, err)
		}
		return
	}

	// Parse JSON output to get file information
	var fileEntries []map[string]interface{}
	if err := json.Unmarshal(listOutput, &fileEntries); err != nil {
		fmt.Printf("Error parsing file list JSON for job %d: %v\n", jobID, err)
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("JSON Parsing Error: %v", err)
		endTime := time.Now()
		history.EndTime = &endTime
		if err := s.db.UpdateJobHistory(history); err != nil {
			fmt.Printf("Error updating job history for job %d: %v\n", jobID, err)
		}
		return
	}

	// Calculate total size and filter out directories
	var files []map[string]interface{}
	var totalSize int64
	for _, entry := range fileEntries {
		// Skip directories
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

	fmt.Printf("Found %d files totaling %d bytes to transfer for job %d\n", len(files), totalSize, jobID)

	// Update history with size information
	history.BytesTransferred = totalSize

	if len(files) == 0 {
		fmt.Printf("No files to transfer for job %d\n", jobID)
		history.Status = "completed"
		history.ErrorMessage = ""
		history.FilesTransferred = 0
	} else {
		var transferErrors []string
		filesTransferred := 0

		// Process each file individually
		for _, fileEntry := range files {
			fileName, ok := fileEntry["Path"].(string)
			if !ok || fileName == "" {
				continue
			}

			// Skip files that have already been processed in this execution
			if processedFiles[fileName] {
				fmt.Printf("Skipping duplicate file entry: %s (already processed in this execution)\n", fileName)
				continue
			}

			// Extract file metadata from the JSON entry
			var fileSize int64
			if size, ok := fileEntry["Size"].(float64); ok {
				fileSize = int64(size)
			}

			// Extract modification time
			modTime := time.Now()
			if modTimeStr, ok := fileEntry["ModTime"].(string); ok {
				if parsedTime, err := time.Parse(time.RFC3339, modTimeStr); err == nil {
					modTime = parsedTime
				}
			}

			// Create time is usually not available for remote files, so we'll use modTime
			createTime := modTime

			// Extract hash if available
			var fileHash string
			if hashes, ok := fileEntry["Hashes"].(map[string]interface{}); ok {
				if md5, ok := hashes["md5"].(string); ok {
					fileHash = md5
				}
			}

			// For local files, calculate hash if not available
			if fileHash == "" && job.Config.SourceType == "local" {
				localFilePath := filepath.Join(job.Config.SourcePath, fileName)
				calculatedHash, hashErr := calculateFileHash(localFilePath)
				if hashErr == nil {
					fileHash = calculatedHash
				}
			}

			skipFiles := job.Config.SkipProcessedFiles

			// Check if this file has been processed before (by hash)
			if fileHash != "" {
				processed, prevMetadata, _ := s.hasFileBeenProcessed(jobID, fileHash)
				if processed {
					fmt.Printf("File %s has been processed before (hash: %s, previous file: %s)\n",
						fileName, fileHash, prevMetadata.FileName)

					// Determine if we should skip this file
					shouldSkip := false
					if skipFiles {
						if prevMetadata.Status == "processed" ||
							prevMetadata.Status == "archived" ||
							prevMetadata.Status == "deleted" ||
							prevMetadata.Status == "archived_and_deleted" {
							shouldSkip = true
						}
					}

					if shouldSkip {
						fmt.Printf("Skipping unchanged file %s (hash matches previous processing)\n", fileName)
						continue
					} else {
						fmt.Printf("Re-processing file %s despite previous processing (skipProcessedFiles=%v)\n", fileName, skipFiles)
					}
				}
			}

			// Also check the processing history for this specific file name
			prevMetadata, histErr := s.checkFileProcessingHistory(jobID, fileName)
			if histErr == nil {
				fmt.Printf("File %s was previously processed on %s with status: %s\n",
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
					fmt.Printf("Skipping unchanged file %s (hash matches previous processing)\n", fileName)
					// Skip this file and continue to the next one
					continue
				} else if fileHash != "" && fileHash == prevMetadata.FileHash {
					fmt.Printf("Re-processing file %s despite matching hash (skipProcessedFiles=%v)\n", fileName, skipFiles)
				}
			}

			// Prepare moveto command for transfer
			transferArgs := []string{
				"--config", configPath,
				"copyto",
				"--progress",
				"--stats-one-line",
				"--verbose",
				"--stats", "1s",
			}

			// Source and destination paths
			var sourcePath, destPath string

			// For S3, MinIO, and B2, include the bucket in the path
			if job.Config.SourceType == "s3" || job.Config.SourceType == "minio" || job.Config.SourceType == "b2" {
				sourcePath = fmt.Sprintf("source_%d:%s/%s", job.Config.ID, job.Config.SourceBucket, fileName)
				if job.Config.SourcePath != "" && job.Config.SourcePath != "/" {
					sourcePath = fmt.Sprintf("source_%d:%s/%s/%s", job.Config.ID, job.Config.SourceBucket, job.Config.SourcePath, fileName)
				}
			} else {
				sourcePath = fmt.Sprintf("source_%d:%s/%s", job.Config.ID, job.Config.SourcePath, fileName)
			}

			var destFile string = fileName

			if job.Config.DestinationType == "s3" || job.Config.DestinationType == "minio" || job.Config.DestinationType == "b2" {
				destPath = fmt.Sprintf("dest_%d:%s/%s", job.Config.ID, job.Config.DestBucket, fileName)
				if job.Config.DestinationPath != "" && job.Config.DestinationPath != "/" {
					destPath = fmt.Sprintf("dest_%d:%s/%s/%s", job.Config.ID, job.Config.DestBucket, job.Config.DestinationPath, fileName)
				}
			} else {
				destPath = fmt.Sprintf("dest_%d:%s/%s", job.Config.ID, job.Config.DestinationPath, fileName)
			}

			// Add output filename pattern if specified
			if job.Config.OutputPattern != "" {
				// Process the output pattern for this specific file
				destFile = ProcessOutputPattern(job.Config.OutputPattern, fileName)

				if job.Config.DestinationType == "s3" || job.Config.DestinationType == "minio" || job.Config.DestinationType == "b2" {
					destPath = fmt.Sprintf("dest_%d:%s/%s", job.Config.ID, job.Config.DestBucket, destFile)
					if job.Config.DestinationPath != "" && job.Config.DestinationPath != "/" {
						destPath = fmt.Sprintf("dest_%d:%s/%s/%s", job.Config.ID, job.Config.DestBucket, job.Config.DestinationPath, destFile)
					}
				} else {
					destPath = fmt.Sprintf("dest_%d:%s/%s", job.Config.ID, job.Config.DestinationPath, destFile)
				}

				fmt.Printf("Renaming file from %s to %s for job %d\n", fileName, destFile, jobID)
			}

			// Add custom flags if specified
			if job.Config.RcloneFlags != "" {
				customFlags := strings.Split(job.Config.RcloneFlags, " ")
				transferArgs = append(transferArgs, customFlags...)
				fmt.Printf("Added custom flags for job %d: %v\n", jobID, customFlags)
			}

			// Add source and destination to the command
			transferArgs = append(transferArgs, sourcePath, destPath)

			// Execute transfer for this file
			fmt.Printf("Executing rclone transfer command for job %d, file %s: rclone %s\n",
				jobID, fileName, strings.Join(transferArgs, " "))
			// Get the rclone path from the environment variable or use the default path
			rclonePath := os.Getenv("RCLONE_PATH")
			if rclonePath == "" {
				rclonePath = "rclone"
			}
			cmd := exec.Command(rclonePath, transferArgs...)
			fileOutput, fileErr := cmd.CombinedOutput()

			// Print the output
			fmt.Printf("Output for file %s: %s\n", fileName, string(fileOutput))

			// Create file metadata record
			fileStatus := "processed"
			var fileErrorMsg string
			var destPathForDB string

			// Check if file was successfully transferred
			if fileErr != nil {
				fmt.Printf("Error transferring file %s for job %d: %v\n", fileName, jobID, fileErr)
				transferErrors = append(transferErrors, fmt.Sprintf("File %s: %v", fileName, fileErr))
				fileStatus = "error"
				fileErrorMsg = fileErr.Error()
			} else {
				filesTransferred++
				fmt.Printf("Successfully transferred file %s for job %d\n", fileName, jobID)

				// Extract the actual destination path (without rclone remote prefix)
				if job.Config.DestinationType == "local" {
					destPathForDB = filepath.Join(job.Config.DestinationPath, destFile)
				} else {
					// For remote destinations, store the path format
					if job.Config.DestinationType == "s3" || job.Config.DestinationType == "minio" || job.Config.DestinationType == "b2" {
						if job.Config.DestinationPath != "" && job.Config.DestinationPath != "/" {
							destPathForDB = fmt.Sprintf("%s/%s/%s", job.Config.DestBucket, job.Config.DestinationPath, destFile)
						} else {
							destPathForDB = fmt.Sprintf("%s/%s", job.Config.DestBucket, destFile)
						}
					} else {
						destPathForDB = fmt.Sprintf("%s/%s", job.Config.DestinationPath, destFile)
					}
				}

				// If archiving is enabled and transfer was successful, move files to archive
				if job.Config.ArchiveEnabled && job.Config.ArchivePath != "" {
					fmt.Printf("Archiving file %s for job %d\n", fileName, jobID)

					// We don't need to move the file since we used moveto, but we can copy it to archive
					archiveArgs := []string{
						"--config", configPath,
						"copyto",
						sourcePath,
					}

					// Construct archive path with bucket if needed
					var archiveDest string
					if job.Config.SourceType == "s3" || job.Config.SourceType == "minio" || job.Config.SourceType == "b2" {
						archiveDest = fmt.Sprintf("source_%d:%s/%s/%s", job.Config.ID, job.Config.SourceBucket, job.Config.ArchivePath, fileName)
					} else {
						archiveDest = fmt.Sprintf("source_%d:%s/%s", job.Config.ID, job.Config.ArchivePath, fileName)
					}

					archiveArgs = append(archiveArgs, archiveDest)

					fmt.Printf("Executing rclone archive command for job %d, file %s: rclone %s\n",
						jobID, fileName, strings.Join(archiveArgs, " "))
					// Get the rclone path from the environment variable or use the default path
					rclonePath := os.Getenv("RCLONE_PATH")
					if rclonePath == "" {
						rclonePath = "rclone"
					}
					archiveCmd := exec.Command(rclonePath, archiveArgs...)
					archiveOutput, archiveErr := archiveCmd.CombinedOutput()

					// Print the output
					fmt.Printf("Output for file %s: %s\n", fileName, string(archiveOutput))

					// Check if file was successfully transferred
					if archiveErr != nil {
						fmt.Printf("Warning: Error archiving file %s for job %d: %v\n", fileName, jobID, archiveErr)
						transferErrors = append(transferErrors,
							fmt.Sprintf("Archive error for file %s: %v", fileName, archiveErr))
					} else {
						fileStatus = "archived"
					}
				}

				if job.Config.DeleteAfterTransfer {
					fmt.Printf("Deleting file %s for job %d\n", fileName, jobID)
					deleteArgs := []string{
						"--config", configPath,
						"deletefile",
						sourcePath}
					deleteCmd := exec.Command(rclonePath, deleteArgs...)
					deleteOutput, deleteErr := deleteCmd.CombinedOutput()
					fmt.Printf("Output for file %s: %s\n", fileName, string(deleteOutput))
					if deleteErr != nil {
						fmt.Printf("Error deleting file %s for job %d: %v\n", fileName, jobID, deleteErr)
						transferErrors = append(transferErrors,
							fmt.Sprintf("Delete error for file %s: %v", fileName, deleteErr))
					} else {
						if fileStatus == "archived" {
							fileStatus = "archived_and_deleted"
						} else {
							fileStatus = "deleted"
						}
					}
				}
			}

			// Mark this file as processed for this execution
			processedFiles[fileName] = true

			// Create and save file metadata
			metadata := &db.FileMetadata{
				JobID:           jobID,
				FileName:        fileName,
				OriginalPath:    job.Config.SourcePath,
				FileSize:        fileSize,
				FileHash:        fileHash,
				CreationTime:    createTime,
				ModTime:         modTime,
				ProcessedTime:   time.Now(),
				DestinationPath: destPathForDB,
				Status:          fileStatus,
				ErrorMessage:    fileErrorMsg,
			}

			if err := s.db.CreateFileMetadata(metadata); err != nil {
				fmt.Printf("Error creating file metadata for %s: %v\n", fileName, err)
			} else {
				fmt.Printf("Created file metadata record for %s (ID: %d)\n", fileName, metadata.ID)
			}
		}

		// Update job history with transfer results
		history.FilesTransferred = filesTransferred

		if len(transferErrors) > 0 {
			history.Status = "completed_with_errors"
			history.ErrorMessage = fmt.Sprintf("Transfer completed with %d errors:\n%s",
				len(transferErrors), strings.Join(transferErrors, "\n"))
		}
	}

	// Update job history with completion status and end time
	endTime := time.Now()
	history.EndTime = &endTime
	if job.Config.ArchiveEnabled && job.Config.ArchivePath != "" {
		if history.ErrorMessage != "" {
			history.Status = "completed_with_archive_error"
		} else {
			history.Status = "completed"
		}
	} else {
		history.Status = "completed"
	}

	if err := s.db.UpdateJobHistory(history); err != nil {
		fmt.Printf("Error updating job history for job %d: %v\n", jobID, err)
	}

	// Update next run time if job is still scheduled
	if entry := s.cron.Entry(s.jobs[jobID]); entry.ID != 0 {
		job.NextRun = &entry.Next
		if err := s.db.UpdateJobStatus(&job); err != nil {
			fmt.Printf("Error updating next run time for job %d: %v\n", jobID, err)
		} else {
			fmt.Printf("Next run time for job %d: %s\n", jobID, entry.Next.Format(time.RFC3339))
		}
	}
}

// ProcessOutputPattern processes an output pattern with variables and returns the result
// This function is useful for testing pattern processing in isolation
func ProcessOutputPattern(pattern string, originalFilename string) string {
	// Process date variables
	dateRegex := regexp.MustCompile(`\${date:([^}]+)}`)
	processedPattern := dateRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		format := dateRegex.FindStringSubmatch(match)[1]
		return time.Now().Format(format)
	})

	// Split the filename and extension
	ext := filepath.Ext(originalFilename)
	filename := strings.TrimSuffix(originalFilename, ext)

	// Replace filename and extension variables
	processedPattern = strings.ReplaceAll(processedPattern, "${filename}", filename)
	processedPattern = strings.ReplaceAll(processedPattern, "${ext}", ext)

	return processedPattern
}

// createRcloneFilterFile creates a temporary filter file for rclone with rename rules
func createRcloneFilterFile(pattern string) (string, error) {
	// Create a temporary file
	tmpFile, err := ioutil.TempFile("", "rclone-filter-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary filter file: %v", err)
	}
	defer tmpFile.Close()

	// Process the pattern to create a rclone filter rule
	// First, replace date variables with current date in the specified format
	dateRegex := regexp.MustCompile(`\${date:([^}]+)}`)
	processedPattern := dateRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		format := dateRegex.FindStringSubmatch(match)[1]
		return time.Now().Format(format)
	})

	// Replace filename and extension variables with rclone's capture group references
	// For rclone rename filters, we need to use {1} for the first capture group, not $1
	// See: https://rclone.org/filtering/#rename

	// Extract filename without extension
	processedPattern = strings.ReplaceAll(processedPattern, "${filename}", "{1}")

	// Extract extension (with the dot)
	processedPattern = strings.ReplaceAll(processedPattern, "${ext}", "{2}")

	// Create a rename rule for rclone using the correct syntax:
	// - The format for rename filters is: "-- SourceRegexp ReplacementPattern"
	// - For files with extension: capture the name and extension separately
	rule := fmt.Sprintf("-- (.*)(\\..+)$ %s\n", processedPattern)

	// Add a fallback rule for files without extension
	fallbackRule := fmt.Sprintf("-- ([^.]+)$ %s\n",
		strings.ReplaceAll(processedPattern, "{2}", ""))

	// Write the rules to the file
	if _, err := tmpFile.WriteString(rule + fallbackRule); err != nil {
		return "", fmt.Errorf("failed to write to filter file: %v", err)
	}

	return tmpFile.Name(), nil
}

func (s *Scheduler) UnscheduleJob(jobID uint) {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	if entryID, exists := s.jobs[jobID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, jobID)
	}
}

func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}

func (s *Scheduler) RunJobNow(jobID uint) error {
	go s.executeJob(jobID)
	return nil
}

// calculateFileHash computes an MD5 hash for the given file path
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("error calculating hash: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getFileInfo retrieves file stats like size, creation time, and modification time
func getFileInfo(filePath string) (int64, time.Time, time.Time, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("error getting file info: %v", err)
	}

	size := info.Size()
	modTime := info.ModTime()

	// Get creation time (this is platform-specific)
	// For simplicity, we'll use modification time as a fallback
	createTime := modTime

	return size, createTime, modTime, nil
}

// hasFileBeenProcessed checks if a file with the same hash has been processed before
func (s *Scheduler) hasFileBeenProcessed(jobID uint, fileHash string) (bool, *db.FileMetadata, error) {
	if fileHash == "" {
		return false, nil, nil
	}

	// First try to find by hash (most reliable)
	metadata, err := s.db.GetFileMetadataByHash(fileHash)
	if err == nil && metadata != nil {
		return true, metadata, nil
	}

	return false, nil, nil
}

// checkFileProcessingHistory checks processing history for a given file
func (s *Scheduler) checkFileProcessingHistory(jobID uint, fileName string) (*db.FileMetadata, error) {
	// Try to find by job and filename
	metadata, err := s.db.GetFileMetadataByJobAndName(jobID, fileName)
	if err == nil && metadata != nil {
		return metadata, nil
	}

	return nil, fmt.Errorf("no history found for file %s in job %d", fileName, jobID)
}

// getRemoteFileInfo gets metadata for a remote file using rclone lsjson
func (s *Scheduler) getRemoteFileInfo(config *db.TransferConfig, file string) (int64, time.Time, time.Time, string, error) {
	// Get rclone config path
	configPath := s.db.GetConfigRclonePath(config)

	// Construct the appropriate source path
	var sourcePath string
	if config.SourceType == "s3" || config.SourceType == "minio" || config.SourceType == "b2" {
		sourcePath = fmt.Sprintf("source_%d:%s", config.ID, config.SourceBucket)
		if config.SourcePath != "" && config.SourcePath != "/" {
			sourcePath = fmt.Sprintf("source_%d:%s/%s", config.ID, config.SourceBucket, config.SourcePath)
		}
	} else {
		sourcePath = fmt.Sprintf("source_%d:%s", config.ID, config.SourcePath)
	}

	// Use rclone lsjson to get file details
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	// Construct the full path to the file
	fullPath := fmt.Sprintf("%s/%s", sourcePath, file)

	// Run rclone lsjson command
	args := []string{
		"--config", configPath,
		"lsjson",
		"--hash",
		fullPath,
	}

	cmd := exec.Command(rclonePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, time.Time{}, time.Time{}, "", fmt.Errorf("error getting remote file info: %v", err)
	}

	// Parse the JSON output
	var files []map[string]interface{}
	if err := json.Unmarshal(output, &files); err != nil {
		return 0, time.Time{}, time.Time{}, "", fmt.Errorf("error parsing lsjson output: %v", err)
	}

	if len(files) == 0 {
		return 0, time.Time{}, time.Time{}, "", fmt.Errorf("file not found: %s", file)
	}

	fileInfo := files[0]

	// Extract file size
	var fileSize int64
	if size, ok := fileInfo["Size"].(float64); ok {
		fileSize = int64(size)
	}

	// Extract modification time
	modTime := time.Now()
	if modTimeStr, ok := fileInfo["ModTime"].(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, modTimeStr); err == nil {
			modTime = parsedTime
		}
	}

	// Create time is usually not available for remote files, so we'll use modTime
	createTime := modTime

	// Calculate hash if available
	var md5Hash string
	if hashes, ok := fileInfo["Hashes"].(map[string]interface{}); ok {
		if md5, ok := hashes["md5"].(string); ok {
			md5Hash = md5
		}
	}

	return fileSize, createTime, modTime, md5Hash, nil
}
