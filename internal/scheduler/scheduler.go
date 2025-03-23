package scheduler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/starfleetcptn/gomft/internal/db"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel represents the verbosity level of logging
type LogLevel int

const (
	// LogLevelError only logs errors
	LogLevelError LogLevel = iota
	// LogLevelInfo logs info and errors
	LogLevelInfo
	// LogLevelDebug logs everything including debug messages
	LogLevelDebug
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelError:
		return "error"
	case LogLevelInfo:
		return "info"
	case LogLevelDebug:
		return "debug"
	default:
		return "unknown"
	}
}

// ParseLogLevel parses a string into a LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "error":
		return LogLevelError
	case "info":
		return LogLevelInfo
	case "debug":
		return LogLevelDebug
	default:
		return LogLevelInfo // Default to info level
	}
}

// Logger handles log output to file and console
type Logger struct {
	Info     *log.Logger
	Error    *log.Logger
	Debug    *log.Logger
	file     *lumberjack.Logger
	logLevel LogLevel
}

// LogInfo logs an info message if the log level allows it
func (l *Logger) LogInfo(format string, v ...interface{}) {
	if l.logLevel >= LogLevelInfo {
		l.Info.Printf(format, v...)
	}
}

// LogError logs an error message if the log level allows it
func (l *Logger) LogError(format string, v ...interface{}) {
	if l.logLevel >= LogLevelError {
		l.Error.Printf(format, v...)
	}
}

// LogDebug logs a debug message if the log level allows it
func (l *Logger) LogDebug(format string, v ...interface{}) {
	if l.logLevel >= LogLevelDebug {
		l.Debug.Printf(format, v...)
	}
}

// NewLogger creates a new logger that writes to both file and console
func NewLogger() *Logger {
	// Get data directory from environment or use default
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Ensure logs directory exists
	logsDir := filepath.Join(dataDir, "logs")
	if envLogsDir := os.Getenv("LOGS_DIR"); envLogsDir != "" {
		logsDir = envLogsDir
	}

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Printf("Error creating logs directory: %v\n", err)
	}

	// Get log rotation settings from environment or use defaults
	maxSize := 10 // Default: 10MB
	if envSize := os.Getenv("LOG_MAX_SIZE"); envSize != "" {
		if size, err := strconv.Atoi(envSize); err == nil && size > 0 {
			maxSize = size
		}
	}

	maxBackups := 5 // Default: keep 5 backups
	if envBackups := os.Getenv("LOG_MAX_BACKUPS"); envBackups != "" {
		if backups, err := strconv.Atoi(envBackups); err == nil && backups >= 0 {
			maxBackups = backups
		}
	}

	maxAge := 30 // Default: 30 days
	if envAge := os.Getenv("LOG_MAX_AGE"); envAge != "" {
		if age, err := strconv.Atoi(envAge); err == nil && age >= 0 {
			maxAge = age
		}
	}

	compress := true // Default: compress logs
	if envCompress := os.Getenv("LOG_COMPRESS"); envCompress == "false" {
		compress = false
	}

	// Get log level from environment or use default
	logLevel := LogLevelInfo // Default to info level
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		logLevel = ParseLogLevel(envLogLevel)
	}

	// Setup log rotation
	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(logsDir, "scheduler.log"),
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}

	// Create multi-writer for both file and console
	consoleAndFile := io.MultiWriter(os.Stdout, logFile)

	// Create loggers with different prefixes
	logger := &Logger{
		Info:     log.New(consoleAndFile, "INFO: ", log.Ldate|log.Ltime),
		Error:    log.New(consoleAndFile, "ERROR: ", log.Ldate|log.Ltime),
		Debug:    log.New(consoleAndFile, "DEBUG: ", log.Ldate|log.Ltime),
		file:     logFile,
		logLevel: logLevel,
	}

	// Log rotation settings and log level
	if logLevel >= LogLevelInfo {
		logger.Info.Printf("Log rotation configured: file=%s, maxSize=%dMB, maxBackups=%d, maxAge=%d days, compress=%v, logLevel=%s",
			filepath.Join(logsDir, "scheduler.log"), maxSize, maxBackups, maxAge, compress, logLevel.String())
	}

	if logLevel >= LogLevelDebug {
		logger.Debug.Printf("Log rotation details: file=%s, maxSize=%dMB, maxBackups=%d, maxAge=%d days, compress=%v",
			filepath.Join(logsDir, "scheduler.log"), maxSize, maxBackups, maxAge, compress)
	}

	return logger
}

// Close closes the log file
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// RotateLogs manually triggers log rotation
func (l *Logger) RotateLogs() error {
	if l.file != nil {
		return l.file.Rotate()
	}
	return nil
}

type Scheduler struct {
	cron     *cron.Cron
	db       *db.DB
	jobMutex sync.Mutex
	jobs     map[uint]cron.EntryID
	log      *Logger
}

func New(database *db.DB) *Scheduler {
	// Create a new logger
	logger := NewLogger()

	logger.Info.Println("Initializing scheduler")
	c := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))
	c.Start()

	s := &Scheduler{
		cron:     c,
		db:       database,
		jobMutex: sync.Mutex{},
		jobs:     make(map[uint]cron.EntryID),
		log:      logger,
	}

	// Load existing jobs
	s.loadJobs()

	return s
}

func (s *Scheduler) loadJobs() {
	s.log.LogInfo("Loading scheduled jobs")

	// Get all jobs from the database
	jobs, err := s.db.GetActiveJobs()
	if err != nil {
		s.log.LogError("Error loading jobs: %v", err)
		return
	}

	// Clear the job map to ensure we're starting fresh
	s.jobMutex.Lock()
	s.jobs = make(map[uint]cron.EntryID)
	s.jobMutex.Unlock()

	// Initialize job count to track successfully loaded jobs
	loadedCount := 0

	for _, job := range jobs {
		// Skip disabled jobs
		if !job.GetEnabled() {
			s.log.LogInfo("Job %d (%s) is disabled, skipping scheduling", job.ID, job.Name)
			continue
		}

		if err := s.ScheduleJob(&job); err != nil {
			s.log.LogError("Error scheduling job %d: %v", job.ID, err)
		} else {
			s.log.LogInfo("Loaded job %d: %s", job.ID, job.Name)
			loadedCount++
		}
	}

	s.log.LogInfo("Loaded %d jobs", loadedCount)
}

func (s *Scheduler) ScheduleJob(job *db.Job) error {
	s.log.LogDebug("Attempting to schedule job ID %d: %+v", job.ID, job)

	s.log.LogInfo("Scheduling job %d: %s with schedule %s", job.ID, job.Name, job.Schedule)

	// Remove existing job if it exists
	if entryID, exists := s.jobs[job.ID]; exists {
		s.log.LogInfo("Removing existing schedule for job %d", job.ID)
		s.cron.Remove(entryID)
		delete(s.jobs, job.ID)
	}

	// Only schedule if job is enabled
	if !job.GetEnabled() {
		s.log.LogInfo("Job %d is disabled, skipping scheduling", job.ID)
		return nil
	}

	// Convert 5-field cron to 6-field by prepending '0' for seconds
	schedule := job.Schedule
	if len(strings.Fields(schedule)) == 5 {
		schedule = "0 " + schedule
	}

	s.log.LogDebug("Converted schedule from '%s' to '%s'", job.Schedule, schedule)

	// Validate cron expression
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	if err != nil {
		return fmt.Errorf("invalid cron expression '%s': %w", job.Schedule, err)
	}

	s.log.LogDebug("Validated cron expression '%s' for job %d", schedule, job.ID)

	// Schedule the job
	entryID, err := s.cron.AddFunc(job.Schedule, func() {
		s.executeJob(job.ID)
	})

	if err != nil {
		s.log.LogError("Error scheduling job %d: %v", job.ID, err)
		return err
	}

	s.log.LogDebug("Scheduled job %d with cron entry ID %d", job.ID, entryID)

	// Store mapping of job ID to cron entry ID
	s.jobMutex.Lock()
	s.jobs[job.ID] = entryID
	s.jobMutex.Unlock()

	// Get next run time
	entry := s.cron.Entry(entryID)
	job.NextRun = &entry.Next
	if err := s.db.UpdateJobStatus(job); err != nil {
		s.log.LogError("Error updating job status for job %d: %v", job.ID, err)
		return err
	}

	return nil
}

func (s *Scheduler) executeJob(jobID uint) {
	s.log.LogDebug("Entering executeJob for job ID %d", jobID)
	defer s.log.LogDebug("Exiting executeJob for job ID %d", jobID)

	s.log.LogInfo("Starting execution of job %d", jobID)

	// Get job details
	var job db.Job
	if err := s.db.First(&job, jobID).Error; err != nil {
		s.log.LogError("Error loading job %d: %v", jobID, err)
		return
	}

	s.log.LogDebug("Loaded job details: %+v", job)

	// Get all configurations associated with this job
	configs, err := s.db.GetConfigsForJob(jobID)
	if err != nil {
		s.log.LogError("Error loading configurations for job %d: %v", jobID, err)
		return
	}

	s.log.LogDebug("Loaded %d configurations for job %d", len(configs), jobID)

	if len(configs) == 0 {
		s.log.LogError("Error: job %d has no associated configurations", jobID)
		return
	}

	// Get the ordered config IDs from the job
	orderedConfigIDs := job.GetConfigIDsList()
	s.log.LogDebug("Ordered config IDs for job %d: %v", jobID, orderedConfigIDs)

	// Create a map of configs for easy lookup
	configMap := make(map[uint]db.TransferConfig)
	for _, config := range configs {
		configMap[config.ID] = config
	}

	// Process configurations in the specified order
	var orderedConfigs []db.TransferConfig

	// First, add configs in the order specified in the job's ConfigIDs
	for _, configID := range orderedConfigIDs {
		if config, exists := configMap[configID]; exists {
			orderedConfigs = append(orderedConfigs, config)
			delete(configMap, configID) // Remove from map to avoid duplicates
		}
	}

	// Add any remaining configs not in the ordered list (shouldn't happen, but just in case)
	for _, config := range configMap {
		orderedConfigs = append(orderedConfigs, config)
	}

	s.log.LogInfo("Processing job %d with %d configurations in specified order", jobID, len(orderedConfigs))

	// Log the order of execution
	for i, config := range orderedConfigs {
		s.log.LogDebug("Execution order %d/%d: Config ID %d (%s)", i+1, len(orderedConfigs), config.ID, config.Name)
	}

	// Update job last run time
	startTime := time.Now()
	job.LastRun = &startTime
	if err := s.db.UpdateJobStatus(&job); err != nil {
		s.log.LogError("Error updating job last run time for job %d: %v", jobID, err)
	}

	// Process each configuration in the specified order
	for i, config := range orderedConfigs {
		s.processConfiguration(&job, &config, i+1, len(orderedConfigs))
	}

	// Update next run time after execution
	s.jobMutex.Lock()
	entryID, exists := s.jobs[jobID]
	s.jobMutex.Unlock()

	if exists {
		entry := s.cron.Entry(entryID)
		nextRun := entry.Next
		job.NextRun = &nextRun
		s.log.LogInfo("Next run time for job %d: %v", jobID, nextRun)
		if err := s.db.UpdateJobStatus(&job); err != nil {
			s.log.LogError("Error updating job next run time for job %d: %v", jobID, err)
		}
	}
}

// processConfiguration processes a single configuration for a job
func (s *Scheduler) processConfiguration(job *db.Job, config *db.TransferConfig, index int, totalConfigs int) {
	s.log.LogDebug("Processing configuration %d: %+v", config.ID, config)

	s.log.LogInfo("Processing configuration %d (%d/%d) for job %d: source=%s:%s, dest=%s:%s",
		config.ID,
		index,
		totalConfigs,
		job.ID,
		config.SourceType,
		config.SourcePath,
		config.DestinationType,
		config.DestinationPath,
	)

	// Create job history entry for this configuration
	history := &db.JobHistory{
		JobID:            job.ID,
		ConfigID:         config.ID,
		StartTime:        time.Now(),
		Status:           "running",
		FilesTransferred: 0,
		BytesTransferred: 0,
		ErrorMessage:     "",
	}
	if err := s.db.CreateJobHistory(history); err != nil {
		s.log.LogError("Error creating job history for job %d, config %d: %v", job.ID, config.ID, err)
		return
	}

	s.log.LogDebug("Creating job history record: %+v", history)

	// Send webhook notification for job start
	s.sendWebhookNotification(job, history, config)

	// Execute the configuration transfer
	s.executeConfigTransfer(*job, *config, history)
}

// executeConfigTransfer performs the actual file transfer for a single configuration
func (s *Scheduler) executeConfigTransfer(job db.Job, config db.TransferConfig, history *db.JobHistory) {
	s.log.LogDebug("Starting transfer for config %d with params: %+v", config.ID, config)

	// Track files already processed in this job execution to prevent duplicates
	processedFiles := make(map[string]bool)

	// Get rclone config path
	configPath := s.db.GetConfigRclonePath(&config)

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
		filterFile, err := createRcloneFilterFile(config.FilePattern)
		if err != nil {
			s.log.LogError("Error creating filter file for job %d, config %d: %v", job.ID, config.ID, err)
			history.Status = "failed"
			history.ErrorMessage = fmt.Sprintf("Filter Creation Error: %v", err)
			endTime := time.Now()
			history.EndTime = &endTime
			if err := s.db.UpdateJobHistory(history); err != nil {
				s.log.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
			}
			// Send webhook notification for failure
			s.sendWebhookNotification(&job, history, &config)

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
	s.log.LogDebug("Full lsjson command: %s %v", os.Getenv("RCLONE_PATH"), listArgs)
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}
	listCmd := exec.Command(rclonePath, listArgs...)
	listOutput, listErr := listCmd.CombinedOutput()

	// Add debug logging of raw output
	if listErr == nil {
		s.log.LogDebug("Raw lsjson output for job %d config %d:\n%s",
			job.ID,
			config.ID,
			string(listOutput))
	} else {
		s.log.LogDebug("Raw lsjson output (error case) for job %d config %d:\n%s",
			job.ID,
			config.ID,
			string(listOutput))
	}

	if listErr != nil {
		s.log.LogError("Error listing files for job %d, config %d: %v", job.ID, config.ID, listErr)
		// s.log.Debug.Printf("Output: %s", string(listOutput))
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("File Listing Error: %v\nOutput: %s", listErr, string(listOutput))
		endTime := time.Now()
		history.EndTime = &endTime
		if err := s.db.UpdateJobHistory(history); err != nil {
			s.log.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send webhook notification for failure
		s.sendWebhookNotification(&job, history, &config)
		return
	}

	// Parse JSON output to get file information
	var fileEntries []map[string]interface{}
	if err := json.Unmarshal(listOutput, &fileEntries); err != nil {
		s.log.LogError("Error parsing file list JSON for job %d, config %d: %v", job.ID, config.ID, err)
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("JSON Parsing Error: %v", err)
		endTime := time.Now()
		history.EndTime = &endTime
		if err := s.db.UpdateJobHistory(history); err != nil {
			s.log.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send webhook notification for failure
		s.sendWebhookNotification(&job, history, &config)
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

	s.log.LogInfo("Found %d files totaling %d bytes to transfer for job %d, config %d", len(files), totalSize, job.ID, config.ID)

	// Update history with size information
	history.BytesTransferred = totalSize

	if len(files) == 0 {
		s.log.LogInfo("No files to transfer for job %d, config %d", job.ID, config.ID)
		history.Status = "completed"
		history.ErrorMessage = ""
		history.FilesTransferred = 0
		endTime := time.Now()
		history.EndTime = &endTime
		if err := s.db.UpdateJobHistory(history); err != nil {
			s.log.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
		}
		// Send webhook notification for empty completion
		s.sendWebhookNotification(&job, history, &config)
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

	s.log.LogInfo("Using %d concurrent transfers for job %d, config %d", maxConcurrent, job.ID, config.ID)

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
			s.log.LogDebug("Skipping duplicate file entry: %s (already processed in this execution)", fileName)
			continue
		}

		// Extract hash from the file entry
		fileHash := ""
		if hashes, ok := fileEntry["Hashes"].(map[string]interface{}); ok {
			// Try several hash algorithms in order of preference
			for _, hashType := range []string{"SHA-1", "sha1", "MD5", "md5", "sha256", "crc32"} {
				if hashValue, found := hashes[hashType]; found {
					if hashStr, ok := hashValue.(string); ok && hashStr != "" {
						s.log.LogDebug("Found hash %s: %s for file %s", hashType, hashStr, fileName)
						fileHash = hashStr
						break
					}
				}
			}
		}

		// Log if no hash was found
		if fileHash == "" {
			s.log.LogDebug("No hash found for file %s. Available fields: %v", fileName, fileEntry)
		}

		// Extract size from the file entry
		fileSize := int64(0)
		if size, ok := fileEntry["Size"].(float64); ok {
			fileSize = int64(size)
		}

		// Skip files that have already been processed based on hash
		skipFiles := config.GetSkipProcessedFiles()

		if skipFiles && fileHash != "" {
			alreadyProcessed, prevMetadata, err := s.hasFileBeenProcessed(job.ID, fileHash)
			if err == nil && alreadyProcessed {
				s.log.LogDebug("File %s with hash %s was previously processed on %s with status: %s",
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
					s.log.LogInfo("Skipping unchanged file %s (hash matches previous processing)", fileName)
					continue
				} else {
					s.log.LogInfo("Re-processing file %s despite previous processing (skipProcessedFiles=%v)", fileName, skipFiles)
				}
			}
		}

		// Also check the processing history for this specific file name
		prevMetadata, histErr := s.checkFileProcessingHistory(job.ID, fileName)
		if histErr == nil {
			s.log.LogDebug("File %s was previously processed on %s with status: %s",
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
				s.log.LogInfo("Skipping unchanged file %s (hash matches previous processing)", fileName)
				// Skip this file and continue to the next one
				continue
			} else if fileHash != "" && fileHash == prevMetadata.FileHash {
				s.log.LogInfo("Re-processing file %s despite matching hash (skipProcessedFiles=%v)", fileName, skipFiles)
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
		s.log.LogDebug("Processing file %d/%d: %s (Size: %d, Hash: %s)",
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
				destFile = ProcessOutputPattern(config.OutputPattern, currentFileName)

				if config.DestinationType == "s3" || config.DestinationType == "minio" || config.DestinationType == "b2" {
					destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, config.DestBucket, destFile)
					if config.DestinationPath != "" && config.DestinationPath != "/" {
						destPath = fmt.Sprintf("dest_%d:%s/%s/%s", config.ID, config.DestBucket, config.DestinationPath, destFile)
					}
				} else {
					destPath = fmt.Sprintf("dest_%d:%s/%s", config.ID, config.DestinationPath, destFile)
				}

				s.log.LogDebug("Renaming file from %s to %s for job %d, config %d", currentFileName, destFile, job.ID, config.ID)
			}

			// Add custom flags if specified
			if config.RcloneFlags != "" {
				customFlags := strings.Split(config.RcloneFlags, " ")
				transferArgs = append(transferArgs, customFlags...)
				s.log.LogDebug("Added custom flags for job %d, config %d: %v", job.ID, config.ID, customFlags)
			}

			// Add source and destination to the command
			transferArgs = append(transferArgs, sourcePath, destPath)

			// Execute transfer for this file
			s.log.LogDebug("Full transfer command: %s %v", rclonePath, transferArgs)
			s.log.LogDebug("Environment: RCLONE_PATH=%s", os.Getenv("RCLONE_PATH"))
			cmd := exec.Command(rclonePath, transferArgs...)
			fileOutput, fileErr := cmd.CombinedOutput()

			// Print the output
			s.log.LogDebug("Output for file %s: %s", currentFileName, string(fileOutput))

			// Create file metadata record
			fileStatus := "processed"
			var fileErrorMsg string
			var destPathForDB string

			// Check if file was successfully transferred
			if fileErr != nil {
				s.log.LogError("Error transferring file %s for job %d, config %d: %v", currentFileName, job.ID, config.ID, fileErr)
				mutex.Lock()
				transferErrors = append(transferErrors, fmt.Sprintf("File %s: %v", currentFileName, fileErr))
				mutex.Unlock()
				fileStatus = "error"
				fileErrorMsg = fileErr.Error()
			} else {
				mutex.Lock()
				filesTransferred++
				mutex.Unlock()
				s.log.LogInfo("Successfully transferred file %s for job %d, config %d", currentFileName, job.ID, config.ID)

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
					s.log.LogInfo("Archiving file %s for job %d, config %d", currentFileName, job.ID, config.ID)

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

					s.log.LogInfo("Executing rclone archive command for job %d, config %d, file %s: rclone %s",
						job.ID, config.ID, currentFileName, strings.Join(archiveArgs, " "))
					// Get the rclone path from the environment variable or use the default path
					rclonePath := os.Getenv("RCLONE_PATH")
					if rclonePath == "" {
						rclonePath = "rclone"
					}
					archiveCmd := exec.Command(rclonePath, archiveArgs...)
					archiveOutput, archiveErr := archiveCmd.CombinedOutput()

					// Print the output
					s.log.LogDebug("Output for file %s: %s", currentFileName, string(archiveOutput))

					// Check if file was successfully transferred
					if archiveErr != nil {
						s.log.LogError("Warning: Error archiving file %s for job %d, config %d: %v", currentFileName, job.ID, config.ID, archiveErr)
						mutex.Lock()
						transferErrors = append(transferErrors,
							fmt.Sprintf("Archive error for file %s: %v", currentFileName, archiveErr))
						mutex.Unlock()
					} else {
						fileStatus = "archived"
					}
				}

				if config.GetDeleteAfterTransfer() {
					s.log.LogInfo("Deleting file %s for job %d, config %d", currentFileName, job.ID, config.ID)
					deleteArgs := []string{
						"--config", configPath,
						"deletefile",
						sourcePath}
					deleteCmd := exec.Command(rclonePath, deleteArgs...)
					deleteOutput, deleteErr := deleteCmd.CombinedOutput()
					s.log.LogDebug("Output for file %s: %s", currentFileName, string(deleteOutput))
					if deleteErr != nil {
						s.log.LogError("Error deleting file %s for job %d, config %d: %v", currentFileName, job.ID, config.ID, deleteErr)
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

			if err := s.db.CreateFileMetadata(metadata); err != nil {
				s.log.LogError("Error creating file metadata for %s: %v", currentFileName, err)
			} else {
				s.log.LogDebug("Created file metadata record for %s (ID: %d) with hash: %s", currentFileName, metadata.ID, currentFileHash)
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

	if err := s.db.UpdateJobHistory(history); err != nil {
		s.log.LogError("Error updating job history for job %d, config %d: %v", job.ID, config.ID, err)
	}

	// Create job notification
	if err := s.createJobNotification(&job, history); err != nil {
		s.log.LogError("Failed to create job notification", "jobID", job.ID, "error", err)
	}

	// Send webhook notification for success or with errors
	s.sendWebhookNotification(&job, history, &config)

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
	s.log.LogInfo("Stopping scheduler")
	s.cron.Stop()
	s.log.Close()
}

// RotateLogs manually triggers log rotation
func (s *Scheduler) RotateLogs() error {
	s.log.LogInfo("Manually rotating logs")
	return s.log.RotateLogs()
}

func (s *Scheduler) RunJobNow(jobID uint) error {
	go s.executeJob(jobID)
	return nil
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

// sendWebhookNotification sends a notification to the configured webhook URL
func (s *Scheduler) sendWebhookNotification(job *db.Job, history *db.JobHistory, config *db.TransferConfig) {
	// First, handle job-specific webhook if configured
	if job.GetWebhookEnabled() && job.WebhookURL != "" {
		// Skip notifications based on settings
		if history.Status == "completed" && !job.GetNotifyOnSuccess() {
			s.log.LogDebug("Skipping success notification for job %d (notifyOnSuccess=false)", job.ID)
		} else if history.Status == "failed" && !job.GetNotifyOnFailure() {
			s.log.LogDebug("Skipping failure notification for job %d (notifyOnFailure=false)", job.ID)
		} else {
			s.log.LogInfo("Sending job-specific webhook notification for job %d", job.ID)
			s.sendJobWebhookNotification(job, history, config)
		}
	}

	// Next, process global notification services
	s.sendGlobalNotifications(job, history, config)
}

// sendJobWebhookNotification sends a notification to the job's configured webhook URL
func (s *Scheduler) sendJobWebhookNotification(job *db.Job, history *db.JobHistory, config *db.TransferConfig) {
	// Create the payload with useful information
	payload := map[string]interface{}{
		"event_type":        "job_execution",
		"job_id":            job.ID,
		"job_name":          job.Name,
		"config_id":         config.ID,
		"config_name":       config.Name,
		"status":            history.Status,
		"start_time":        history.StartTime.Format(time.RFC3339),
		"history_id":        history.ID,
		"bytes_transferred": history.BytesTransferred,
		"files_transferred": history.FilesTransferred,
	}

	if history.EndTime != nil {
		payload["end_time"] = history.EndTime.Format(time.RFC3339)
		duration := history.EndTime.Sub(history.StartTime)
		payload["duration_seconds"] = duration.Seconds()
	}

	if history.ErrorMessage != "" {
		payload["error_message"] = history.ErrorMessage
	}

	// Add source and destination information
	payload["source"] = map[string]string{
		"type": config.SourceType,
		"path": config.SourcePath,
	}
	payload["destination"] = map[string]string{
		"type": config.DestinationType,
		"path": config.DestinationPath,
	}

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		s.log.LogError("Error marshaling webhook payload for job %d: %v", job.ID, err)
		return
	}

	s.log.LogDebug("Webhook payload: %s", string(jsonPayload))

	// Create HTTP request
	req, err := http.NewRequest("POST", job.WebhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		s.log.LogError("Error creating webhook request for job %d: %v", job.ID, err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoMFT-Webhook/1.0")

	// Add X-Hub-Signature if secret is configured
	if job.WebhookSecret != "" {
		h := hmac.New(sha256.New, []byte(job.WebhookSecret))
		h.Write(jsonPayload)
		signature := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Hub-Signature-256", signature)
	}

	// Add custom headers if specified
	if job.WebhookHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(job.WebhookHeaders), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	s.log.LogDebug("Webhook headers: %+v", req.Header)

	// Send the request with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		s.log.LogError("Error sending webhook for job %d: %v", job.ID, err)
		return
	}
	defer resp.Body.Close()

	// Log the response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.log.LogInfo("Webhook notification for job %d sent successfully (status: %d)", job.ID, resp.StatusCode)
	} else {
		s.log.LogError("Webhook notification for job %d failed with status: %d", job.ID, resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		if len(respBody) > 0 {
			s.log.LogDebug("Webhook response: %s", respBody)
		}
	}
}

// sendGlobalNotifications sends notifications through all configured notification services
func (s *Scheduler) sendGlobalNotifications(job *db.Job, history *db.JobHistory, config *db.TransferConfig) {
	// Fetch all enabled notification services
	services, err := s.db.GetNotificationServices(true)
	if err != nil {
		s.log.LogError("Error fetching notification services: %v", err)
		return
	}

	if len(services) == 0 {
		s.log.LogDebug("No enabled notification services found")
		return
	}

	s.log.LogInfo("Found %d enabled notification services", len(services))

	// Determine event type based on job status
	var eventType string
	switch history.Status {
	case "running":
		eventType = "job_start"
	case "completed", "completed_with_errors":
		eventType = "job_complete"
	case "failed":
		eventType = "job_error"
	default:
		eventType = "job_status"
	}

	// Process each notification service
	for i := range services {
		service := &services[i] // Use pointer to update stats
		s.log.LogInfo("Processing notification service %s (%s)", service.Name, service.Type)
		// Check if this service should handle this event type
		shouldSend := false
		for _, trigger := range service.EventTriggers {
			if trigger == eventType {
				shouldSend = true
				break
			}
		}

		// Skip if this service doesn't handle this event type
		if !shouldSend {
			s.log.LogDebug("Skipping notification service %s (%s) for event %s (not in triggers)",
				service.Name, service.Type, eventType)
			continue
		}

		s.log.LogInfo("Sending notification via service %s (%s) for job %d",
			service.Name, service.Type, job.ID)

		// Send notification based on service type
		var notifyErr error
		switch service.Type {
		case "email":
			notifyErr = s.sendEmailNotification(service, job, history, config, eventType)
		case "webhook":
			notifyErr = s.sendServiceWebhookNotification(service, job, history, config, eventType)
		default:
			s.log.LogError("Unsupported notification service type: %s", service.Type)
			continue
		}

		// Update service success/failure count
		if notifyErr != nil {
			service.FailureCount++
			s.log.LogError("Notification service %s failed: %v", service.Name, notifyErr)
		} else {
			service.SuccessCount++
			service.LastUsed = time.Now()
			s.log.LogInfo("Notification service %s sent successfully", service.Name)
		}

		// Update notification service stats in the database
		if err := s.db.UpdateNotificationService(service); err != nil {
			s.log.LogError("Error updating notification service stats: %v", err)
		}
	}
}

// sendEmailNotification sends an email notification using the configured email service
func (s *Scheduler) sendEmailNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	s.log.LogDebug("Preparing email notification via service %s for job %d", service.Name, job.ID)

	// Extract SMTP settings from service config
	smtpHost := service.Config["smtp_host"]
	smtpPortStr := service.Config["smtp_port"]
	fromEmail := service.Config["from_email"]
	toEmail := service.Config["to_email"]

	// Validate required settings
	if smtpHost == "" || smtpPortStr == "" || fromEmail == "" || toEmail == "" {
		return fmt.Errorf("missing required SMTP settings")
	}

	// Parse SMTP port
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %v", err)
	}

	// Prepare email content
	subject := fmt.Sprintf("[GoMFT] Job %s: %s", job.Name, history.Status)
	body := generateEmailBody(job, history, config, eventType)

	// TODO: Implement actual email sending logic
	// This would typically involve using a package like "net/smtp" or a third-party
	// email library to send the actual email.
	// For actual implementation, you would use:
	// - smtpUsername := service.Config["smtp_username"]
	// - smtpPassword := service.Config["smtp_password"]

	s.log.LogInfo("Email would be sent to %s with subject: %s", toEmail, subject)
	s.log.LogDebug("Email body: %s", body)

	// Placeholder for actual email sending
	// For now, we'll just log that the email would be sent
	s.log.LogInfo("Email notification prepared (SMTP: %s:%d, From: %s, To: %s)",
		smtpHost, smtpPort, fromEmail, toEmail)

	return nil
}

// generateEmailBody creates the email body for job notifications
func generateEmailBody(job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Job: %s (ID: %d)\n", job.Name, job.ID))
	b.WriteString(fmt.Sprintf("Status: %s\n", history.Status))
	b.WriteString(fmt.Sprintf("Start Time: %s\n", history.StartTime.Format(time.RFC3339)))

	if history.EndTime != nil {
		b.WriteString(fmt.Sprintf("End Time: %s\n", history.EndTime.Format(time.RFC3339)))
		duration := history.EndTime.Sub(history.StartTime)
		b.WriteString(fmt.Sprintf("Duration: %.2f seconds\n", duration.Seconds()))
	}

	b.WriteString(fmt.Sprintf("Files Transferred: %d\n", history.FilesTransferred))
	b.WriteString(fmt.Sprintf("Bytes Transferred: %d\n", history.BytesTransferred))

	b.WriteString("\nTransfer Configuration:\n")
	b.WriteString(fmt.Sprintf("Name: %s (ID: %d)\n", config.Name, config.ID))
	b.WriteString(fmt.Sprintf("Source: %s:%s\n", config.SourceType, config.SourcePath))
	b.WriteString(fmt.Sprintf("Destination: %s:%s\n", config.DestinationType, config.DestinationPath))

	if history.ErrorMessage != "" {
		b.WriteString("\nError Details:\n")
		b.WriteString(history.ErrorMessage)
	}

	return b.String()
}

// sendServiceWebhookNotification sends a webhook notification using a configured notification service
func (s *Scheduler) sendServiceWebhookNotification(service *db.NotificationService, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) error {
	s.log.LogDebug("Preparing webhook notification via service %s for job %d", service.Name, job.ID)

	// Extract webhook settings
	webhookURL := service.Config["webhook_url"]
	method := service.Config["method"]
	if method == "" {
		method = "POST" // Default to POST if not specified
	}

	// Validate required settings
	if webhookURL == "" {
		return fmt.Errorf("missing webhook URL")
	}

	// Prepare payload
	var payload map[string]interface{}

	// Use custom payload template if provided
	if service.PayloadTemplate != "" {
		// Parse the template and fill in variables
		payload = generateCustomPayload(service.PayloadTemplate, job, history, config, eventType)
	} else {
		// Use default payload format
		payload = generateDefaultPayload(job, history, config, eventType)
	}

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling webhook payload: %v", err)
	}

	s.log.LogDebug("Webhook payload: %s", string(jsonPayload))

	// Create HTTP request
	req, err := http.NewRequest(method, webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error creating webhook request: %v", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoMFT-Notification/1.0")

	// Add signature if secret key is provided
	if service.SecretKey != "" {
		h := hmac.New(sha256.New, []byte(service.SecretKey))
		h.Write(jsonPayload)
		signature := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-GoMFT-Signature", signature)
	}

	// Add custom headers if specified
	if headersStr := service.Config["headers"]; headersStr != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersStr), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	s.log.LogDebug("Webhook headers: %+v", req.Header)

	// Determine timeout based on retry policy
	timeout := 10 * time.Second
	maxRetries := 0

	switch service.RetryPolicy {
	case "none":
		maxRetries = 0
	case "simple":
		maxRetries = 3
		timeout = 15 * time.Second
	case "exponential":
		maxRetries = 5
		timeout = 30 * time.Second
	default:
		// Default to simple
		maxRetries = 3
		timeout = 15 * time.Second
	}

	// Prepare client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Attempt to send with retries
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with increasing backoff
			backoffDuration := time.Duration(1<<uint(attempt-1)) * time.Second
			s.log.LogInfo("Retrying webhook notification (attempt %d/%d) after %v",
				attempt, maxRetries, backoffDuration)
			time.Sleep(backoffDuration)
		}

		resp, err = client.Do(req)
		if err == nil {
			// Check for success status code
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				defer resp.Body.Close()
				s.log.LogInfo("Webhook notification sent successfully (status: %d)", resp.StatusCode)
				return nil
			}

			// Error status code
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, respBody)
			s.log.LogError("Webhook error (attempt %d/%d): %v", attempt+1, maxRetries+1, lastErr)
		} else {
			// Network or request error
			lastErr = fmt.Errorf("webhook request failed: %v", err)
			s.log.LogError("Webhook request error (attempt %d/%d): %v", attempt+1, maxRetries+1, lastErr)
		}
	}

	return lastErr
}

// generateDefaultPayload creates a standard webhook payload
func generateDefaultPayload(job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) map[string]interface{} {
	payload := map[string]interface{}{
		"event": eventType,
		"job": map[string]interface{}{
			"id":             job.ID,
			"name":           job.Name,
			"status":         history.Status,
			"message":        history.ErrorMessage,
			"started_at":     history.StartTime.Format(time.RFC3339),
			"config_id":      config.ID,
			"config_name":    config.Name,
			"transfer_bytes": history.BytesTransferred,
			"file_count":     history.FilesTransferred,
		},
		"instance": map[string]interface{}{
			"id":          "gomft",
			"name":        "GoMFT",
			"version":     "1.0",        // TODO: Get actual version
			"environment": "production", // TODO: Get from env
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if history.EndTime != nil {
		payload["job"].(map[string]interface{})["completed_at"] = history.EndTime.Format(time.RFC3339)
		duration := history.EndTime.Sub(history.StartTime)
		payload["job"].(map[string]interface{})["duration_seconds"] = duration.Seconds()
	}

	return payload
}

// generateCustomPayload creates a webhook payload from a template
func generateCustomPayload(template string, job *db.Job, history *db.JobHistory, config *db.TransferConfig, eventType string) map[string]interface{} {
	// Start with the default payload as a base
	defaultPayload := generateDefaultPayload(job, history, config, eventType)

	// Parse the template string to JSON
	var customPayload map[string]interface{}
	if err := json.Unmarshal([]byte(template), &customPayload); err != nil {
		// If template can't be parsed, fall back to default payload
		return defaultPayload
	}

	// Replace variables in the template
	// This is a simplified version - a real implementation would do deep traversal
	// and replace all variables in the structure
	processedPayload := processPayloadVariables(customPayload, defaultPayload)

	return processedPayload
}

// processPayloadVariables recursively processes a payload structure and replaces variables
func processPayloadVariables(customPayload map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Process each key-value pair in the custom payload
	for key, value := range customPayload {
		switch v := value.(type) {
		case string:
			// Replace string variables
			result[key] = replaceVariables(v, variables)
		case map[string]interface{}:
			// Recursively process nested maps
			result[key] = processPayloadVariables(v, variables)
		case []interface{}:
			// Process arrays
			result[key] = processArrayVariables(v, variables)
		default:
			// Keep other types as is
			result[key] = value
		}
	}

	return result
}

// processArrayVariables processes array elements for variable replacement
func processArrayVariables(array []interface{}, variables map[string]interface{}) []interface{} {
	result := make([]interface{}, len(array))

	for i, value := range array {
		switch v := value.(type) {
		case string:
			result[i] = replaceVariables(v, variables)
		case map[string]interface{}:
			result[i] = processPayloadVariables(v, variables)
		case []interface{}:
			result[i] = processArrayVariables(v, variables)
		default:
			result[i] = value
		}
	}

	return result
}

// replaceVariables replaces variable placeholders in a string with their values
func replaceVariables(template string, variables map[string]interface{}) string {
	// Check for variable pattern like {{job.name}}
	re := regexp.MustCompile(`{{([^{}]+)}}`)
	result := re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract variable path (e.g., "job.name")
		varPath := re.FindStringSubmatch(match)[1]
		parts := strings.Split(varPath, ".")

		// Navigate the variables structure to find the value
		var current interface{} = variables
		for _, part := range parts {
			if m, ok := current.(map[string]interface{}); ok {
				if val, exists := m[part]; exists {
					current = val
				} else {
					return match // Keep original if not found
				}
			} else {
				return match // Keep original if structure doesn't match
			}
		}

		// Convert the found value to string
		switch v := current.(type) {
		case string:
			return v
		case int, int64, uint, uint64, float32, float64:
			return fmt.Sprintf("%v", v)
		case bool:
			return fmt.Sprintf("%v", v)
		case time.Time:
			return v.Format(time.RFC3339)
		default:
			// For complex types, convert to JSON
			if bytes, err := json.Marshal(v); err == nil {
				return string(bytes)
			}
			return match
		}
	})

	return result
}

func (s *Scheduler) updateJobStatus(jobID uint, status string, startTime, endTime time.Time, message string) (*db.JobHistory, error) {
	// Create the history record
	history := &db.JobHistory{
		JobID:        jobID,
		Status:       status,
		StartTime:    startTime,
		EndTime:      &endTime,
		ErrorMessage: message,
	}

	// Add code to create notifications for job events
	if job, err := s.db.GetJob(jobID); err == nil {
		// Get the user who created the job
		userID := job.CreatedBy

		// Create job title from job name or ID
		jobTitle := job.Name
		if jobTitle == "" {
			jobTitle = fmt.Sprintf("Job #%d", job.ID)
		}

		// Check which notification to send based on status
		var notificationType db.NotificationType
		var title string
		var message string

		switch status {
		case "running":
			notificationType = db.NotificationJobStart
			title = "Job Started"
			message = jobTitle
		case "completed":
			notificationType = db.NotificationJobComplete
			title = "Job Complete"
			message = jobTitle
		case "failed":
			notificationType = db.NotificationJobFail
			title = "Job Failed"
			message = jobTitle
			if history.ErrorMessage != "" {
				message = jobTitle + ": " + history.ErrorMessage
			}
		default:
			// Don't create notifications for other statuses
			return history, nil
		}

		// Create the notification
		err = s.db.CreateJobNotification(
			userID,
			jobID,
			history.ID, // Use the job history ID
			notificationType,
			title,
			message,
		)

		if err != nil {
			s.log.LogError("Failed to create job notification", "jobID", job.ID, "error", err)
			// Continue anyway, not critical
		}
	}

	return history, nil
}

// Create a job history record and send notification
func (s *Scheduler) createJobHistoryAndNotify(job *db.Job, status string, startTime time.Time, endTime time.Time, message string) error {
	// Create the job history entry
	history := db.JobHistory{
		JobID:        job.ID,
		Status:       status,
		StartTime:    startTime,
		EndTime:      &endTime,
		ErrorMessage: message,
	}

	// Save to database
	if err := s.db.Create(&history).Error; err != nil {
		s.log.LogError("Failed to create job history", "jobID", job.ID, "error", err)
		return err
	}

	// Create notification
	err := s.createJobNotification(job, &history)
	if err != nil {
		s.log.LogError("Failed to create job notification", "jobID", job.ID, "error", err)
		// Continue anyway - notification is not critical
	}

	return nil
}

// Create a notification for a job event
func (s *Scheduler) createJobNotification(job *db.Job, history *db.JobHistory) error {
	// Get the user who created the job
	userID := job.CreatedBy

	// Create job title from job name or ID
	jobTitle := job.Name
	if jobTitle == "" {
		jobTitle = fmt.Sprintf("Job #%d", job.ID)
	}

	// Determine notification type and content
	var notificationType db.NotificationType
	var title string
	var message string

	switch history.Status {
	case "running":
		notificationType = db.NotificationJobStart
		title = "Job Started"
		message = jobTitle
	case "completed":
		notificationType = db.NotificationJobComplete
		title = "Job Complete"
		message = jobTitle
	case "failed":
		notificationType = db.NotificationJobFail
		title = "Job Failed"
		message = jobTitle
		if history.ErrorMessage != "" {
			message = jobTitle + ": " + history.ErrorMessage
		}
	default:
		// Don't create notifications for other statuses
		return nil
	}

	// Create the notification
	return s.db.CreateJobNotification(
		userID,
		job.ID,
		history.ID,
		notificationType,
		title,
		message,
	)
}
