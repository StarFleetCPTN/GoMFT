package db

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/starfleetcptn/gomft/internal/db/migrations"
	"gorm.io/gorm"
)

type User struct {
	ID                  uint   `gorm:"primarykey"`
	Email               string `gorm:"unique;not null"`
	PasswordHash        string `gorm:"not null"`
	IsAdmin             *bool  `gorm:"default:false"`
	LastPasswordChange  time.Time
	FailedLoginAttempts int   `gorm:"default:0"`
	AccountLocked       *bool `gorm:"default:false"`
	LockoutUntil        *time.Time
	Theme               string `gorm:"default:'light'"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type PasswordHistory struct {
	ID           uint   `gorm:"primarykey"`
	UserID       uint   `gorm:"not null"`
	User         User   `gorm:"foreignkey:UserID"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}

type PasswordResetToken struct {
	ID        uint      `gorm:"primarykey"`
	UserID    uint      `gorm:"not null"`
	User      User      `gorm:"foreignkey:UserID"`
	Token     string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Used      *bool     `gorm:"default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TransferConfig struct {
	ID             uint   `gorm:"primarykey"`
	Name           string `gorm:"not null" form:"name"`
	SourceType     string `gorm:"not null" form:"source_type"`
	SourcePath     string `gorm:"not null" form:"source_path"`
	SourceHost     string `form:"source_host"`
	SourcePort     int    `gorm:"default:22" form:"source_port"`
	SourceUser     string `form:"source_user"`
	SourcePassword string `form:"source_password" gorm:"-"` // Not stored in DB, only used for form
	SourceKeyFile  string `form:"source_key_file"`
	// S3 source fields
	SourceBucket    string `form:"source_bucket"`
	SourceRegion    string `form:"source_region"`
	SourceAccessKey string `form:"source_access_key"`
	SourceSecretKey string `form:"source_secret_key" gorm:"-"` // Not stored in DB, only used for form
	SourceEndpoint  string `form:"source_endpoint"`
	// SMB source fields
	SourceShare  string `form:"source_share"`
	SourceDomain string `form:"source_domain"`
	// FTP source fields
	SourcePassiveMode *bool `gorm:"default:true" form:"source_passive_mode"`
	// OneDrive and Google Drive source fields
	SourceClientID     string `form:"source_client_id"`
	SourceClientSecret string `form:"source_client_secret" gorm:"-"` // Not stored in DB, only used for form
	SourceDriveID      string `form:"source_drive_id"`               // For OneDrive
	SourceTeamDrive    string `form:"source_team_drive"`             // For Google Drive
	// Google Photos source fields
	SourceReadOnly        *bool `form:"source_read_only"`        // For Google Photos
	SourceStartYear       int   `form:"source_start_year"`       // For Google Photos
	SourceIncludeArchived *bool `form:"source_include_archived"` // For Google Photos
	// General fields
	FilePattern     string `gorm:"default:'*'" form:"file_pattern"`
	OutputPattern   string `form:"output_pattern"` // Pattern for output filenames with date variables
	DestinationType string `gorm:"not null" form:"destination_type"`
	DestinationPath string `gorm:"not null" form:"destination_path"`
	DestHost        string `form:"dest_host"`
	DestPort        int    `gorm:"default:22" form:"dest_port"`
	DestUser        string `form:"dest_user"`
	DestPassword    string `form:"dest_password" gorm:"-"` // Not stored in DB, only used for form
	DestKeyFile     string `form:"dest_key_file"`
	// S3 destination fields
	DestBucket    string `form:"dest_bucket"`
	DestRegion    string `form:"dest_region"`
	DestAccessKey string `form:"dest_access_key"`
	DestSecretKey string `form:"dest_secret_key" gorm:"-"` // Not stored in DB, only used for form
	DestEndpoint  string `form:"dest_endpoint"`
	// SMB destination fields
	DestShare  string `form:"dest_share"`
	DestDomain string `form:"dest_domain"`
	// FTP destination fields
	DestPassiveMode *bool `gorm:"default:true" form:"dest_passive_mode"`
	// OneDrive and Google Drive destination fields
	DestClientID     string `form:"dest_client_id"`
	DestClientSecret string `form:"dest_client_secret" gorm:"-"` // Not stored in DB, only used for form
	DestDriveID      string `form:"dest_drive_id"`               // For OneDrive
	DestTeamDrive    string `form:"dest_team_drive"`             // For Google Drive
	// Google Photos destination fields
	DestReadOnly        *bool `form:"dest_read_only"`        // For Google Photos
	DestStartYear       int   `form:"dest_start_year"`       // For Google Photos
	DestIncludeArchived *bool `form:"dest_include_archived"` // For Google Photos
	// Security fields
	UseBuiltinAuthSource     *bool `form:"use_builtin_auth_source"` // For Google and other OAuth services
	UseBuiltinAuthDest       *bool `form:"use_builtin_auth_dest"`   // For Google and other OAuth services
	GoogleDriveAuthenticated *bool // Whether Google Drive auth is completed
	// General fields
	ArchivePath            string `form:"archive_path"`
	ArchiveEnabled         *bool  `gorm:"default:false" form:"archive_enabled"`
	RcloneFlags            string `form:"rclone_flags"`
	DeleteAfterTransfer    *bool  `gorm:"default:false" form:"delete_after_transfer"`
	SkipProcessedFiles     *bool  `gorm:"default:true" form:"skip_processed_files"`
	MaxConcurrentTransfers int    `gorm:"default:4" form:"max_concurrent_transfers"` // Number of concurrent file transfers
	CreatedBy              uint
	User                   User `gorm:"foreignkey:CreatedBy"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type Job struct {
	ID        uint           `gorm:"primarykey"`
	Name      string         `form:"name"`
	ConfigID  uint           `gorm:"not null" form:"config_id"`
	Config    TransferConfig `gorm:"foreignkey:ConfigID"`
	ConfigIDs string         `gorm:"column:config_ids"` // Comma-separated list of config IDs
	Schedule  string         `gorm:"not null" form:"schedule"`
	Enabled   *bool          `gorm:"default:true" form:"enabled"`
	LastRun   *time.Time
	NextRun   *time.Time
	// Webhook notification fields
	WebhookEnabled  *bool  `gorm:"default:false" form:"webhook_enabled"`
	WebhookURL      string `form:"webhook_url"`
	WebhookSecret   string `form:"webhook_secret"`
	WebhookHeaders  string `form:"webhook_headers"` // JSON-encoded headers
	NotifyOnSuccess *bool  `gorm:"default:true" form:"notify_on_success"`
	NotifyOnFailure *bool  `gorm:"default:true" form:"notify_on_failure"`
	CreatedBy       uint
	User            User `gorm:"foreignkey:CreatedBy"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GetConfigIDsList returns the list of config IDs as integers
func (j *Job) GetConfigIDsList() []uint {
	if j.ConfigIDs == "" {
		// If ConfigIDs is empty but ConfigID is set, return that as the only ID
		if j.ConfigID > 0 {
			return []uint{j.ConfigID}
		}
		return []uint{}
	}

	// Split the comma-separated string
	strIDs := strings.Split(j.ConfigIDs, ",")
	ids := make([]uint, 0, len(strIDs))

	// Convert each string to uint
	for _, strID := range strIDs {
		if id, err := strconv.ParseUint(strings.TrimSpace(strID), 10, 32); err == nil {
			ids = append(ids, uint(id))
		}
	}

	return ids
}

// SetConfigIDsList sets the config IDs from a slice of uint
func (j *Job) SetConfigIDsList(ids []uint) {
	// Convert to strings
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = strconv.FormatUint(uint64(id), 10)
	}

	// Join with commas
	j.ConfigIDs = strings.Join(strIDs, ",")

	// If there's at least one ID, set ConfigID to the first one for backward compatibility
	if len(ids) > 0 {
		j.ConfigID = ids[0]
	}
}

// GetConfigIDsAsStrings returns the list of config IDs as strings for template rendering
func (j *Job) GetConfigIDsAsStrings() []string {
	ids := j.GetConfigIDsList()
	strIDs := make([]string, len(ids))

	for i, id := range ids {
		strIDs[i] = fmt.Sprintf("'%d'", id)
	}

	return strIDs
}

type JobHistory struct {
	ID               uint      `gorm:"primarykey"`
	JobID            uint      `gorm:"not null"`
	Job              Job       `gorm:"foreignkey:JobID"`
	ConfigID         uint      `gorm:"default:0"` // The specific config ID this history entry is for
	StartTime        time.Time `gorm:"not null"`
	EndTime          *time.Time
	Status           string `gorm:"not null"`
	BytesTransferred int64
	FilesTransferred int
	ErrorMessage     string
}

// FileMetadata stores information about processed files
type FileMetadata struct {
	ID              uint   `gorm:"primarykey"`
	JobID           uint   `gorm:"not null;index"`
	Job             Job    `gorm:"foreignkey:JobID"`
	ConfigID        uint   `gorm:"default:0"` // The specific config ID this file was processed with
	FileName        string `gorm:"not null"`
	OriginalPath    string `gorm:"not null"`
	FileSize        int64  `gorm:"not null"`
	FileHash        string `gorm:"index"` // MD5 or other hash for file identity
	CreationTime    time.Time
	ModTime         time.Time
	ProcessedTime   time.Time `gorm:"not null"`
	DestinationPath string    `gorm:"not null"`
	Status          string    `gorm:"not null"` // processed, archived, deleted, etc.
	ErrorMessage    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DB struct {
	*gorm.DB
}

func Initialize(dbPath string) (*DB, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	// Open database connection with modernc.org/sqlite driver
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Initialize and run migrations
	m := migrations.InitMigrations(db)
	if err := m.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return &DB{DB: db}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// User operations
func (db *DB) CreateUser(user *User) error {
	return db.Create(user).Error
}

func (db *DB) GetUserByEmail(email string) (*User, error) {
	var user User
	err := db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) GetUserByID(id uint) (*User, error) {
	var user User
	err := db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) UpdateUser(user *User) error {
	return db.Save(user).Error
}

// PasswordResetToken operations
func (db *DB) CreatePasswordResetToken(token *PasswordResetToken) error {
	return db.Create(token).Error
}

func (db *DB) GetPasswordResetToken(token string) (*PasswordResetToken, error) {
	var resetToken PasswordResetToken
	err := db.Where("token = ? AND used = ? AND expires_at > ?", token, false, time.Now()).First(&resetToken).Error
	if err != nil {
		return nil, err
	}
	return &resetToken, nil
}

func (db *DB) MarkPasswordResetTokenAsUsed(tokenID uint) error {
	return db.Model(&PasswordResetToken{}).Where("id = ?", tokenID).Update("used", true).Error
}

// TransferConfig operations
func (db *DB) CreateTransferConfig(config *TransferConfig) error {
	return db.Create(config).Error
}

func (db *DB) GetTransferConfigs(userID uint) ([]TransferConfig, error) {
	var configs []TransferConfig
	err := db.Where("created_by = ?", userID).Find(&configs).Error
	return configs, err
}

func (db *DB) GetTransferConfig(id uint) (*TransferConfig, error) {
	var config TransferConfig
	err := db.First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (db *DB) UpdateTransferConfig(config *TransferConfig) error {
	return db.Save(config).Error
}

func (db *DB) DeleteTransferConfig(id uint) error {
	// First check if any jobs are using this config
	var count int64
	if err := db.Model(&Job{}).Where("config_id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check for dependent jobs: %v", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete config: %d jobs are using this configuration", count)
	}

	// Delete the config
	return db.Delete(&TransferConfig{}, id).Error
}

// Job operations
func (db *DB) CreateJob(job *Job) error {
	// Use Omit to prevent GORM from creating a new config
	return db.Omit("Config").Create(job).Error
}

func (db *DB) GetJobs(userID uint) ([]Job, error) {
	var jobs []Job
	err := db.Preload("Config").Where("created_by = ?", userID).Find(&jobs).Error
	return jobs, err
}

func (db *DB) GetJob(id uint) (*Job, error) {
	var job Job
	err := db.Preload("Config").First(&job, id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (db *DB) UpdateJob(job *Job) error {
	// Use Omit to prevent GORM from updating or creating a new config
	return db.Omit("Config").Save(job).Error
}

func (db *DB) DeleteJob(id uint) error {
	// Delete associated job history records first
	if err := db.Where("job_id = ?", id).Delete(&JobHistory{}).Error; err != nil {
		return fmt.Errorf("failed to delete job history: %v", err)
	}

	// Delete the job
	return db.Delete(&Job{}, id).Error
}

func (db *DB) UpdateJobStatus(job *Job) error {
	return db.Save(job).Error
}

// JobHistory operations
func (db *DB) CreateJobHistory(history *JobHistory) error {
	return db.Create(history).Error
}

func (db *DB) UpdateJobHistory(history *JobHistory) error {
	return db.Save(history).Error
}

func (db *DB) GetJobHistory(jobID uint) ([]JobHistory, error) {
	var histories []JobHistory
	err := db.Where("job_id = ?", jobID).Order("start_time desc").Find(&histories).Error
	return histories, err
}

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

// GetConfigRclonePath returns the path to the rclone config file for a given transfer config
func (db *DB) GetConfigRclonePath(config *TransferConfig) string {
	// Get data directory from environment or use default
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Store configs in the data directory
	return filepath.Join(dataDir, "configs", fmt.Sprintf("config_%d.conf", config.ID))
}

func (db *DB) GenerateRcloneConfig(config *TransferConfig) error {
	configPath := db.GetConfigRclonePath(config)

	// Get the directory part of the path
	configDir := filepath.Dir(configPath)

	// Ensure configs directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create configs directory: %v", err)
	}

	// Get the rclone path from the environment variable or use the default path
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	sourceName := fmt.Sprintf("source_%d", config.ID)
	// Generate rclone config using rclone CLI
	switch config.SourceType {
	case "sftp":
		args := []string{
			"config", "create", sourceName, "sftp",
			"host", config.SourceHost,
			"user", config.SourceUser,
			"port", fmt.Sprintf("%d", config.SourcePort),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		if config.SourcePassword != "" {
			args = append(args, "pass", config.SourcePassword)
		}
		if config.SourceKeyFile != "" {
			args = append(args, "key_file", config.SourceKeyFile)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "s3":
		args := []string{
			"config", "create", sourceName, "s3",
			"provider", "AWS",
			"env_auth", "false",
			"access_key_id", config.SourceAccessKey,
			"secret_access_key", config.SourceSecretKey,
			"region", config.SourceRegion,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.SourceEndpoint != "" {
			args = append(args, "endpoint", config.SourceEndpoint)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "minio":
		args := []string{
			"config", "create", sourceName, "s3",
			"provider", "Minio",
			"env_auth", "false",
			"access_key_id", config.SourceAccessKey,
			"secret_access_key", config.SourceSecretKey,
			"endpoint", config.SourceEndpoint,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "b2":
		args := []string{
			"config", "create", sourceName, "b2",
			"account", config.SourceAccessKey,
			"key", config.SourceSecretKey,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "smb":
		args := []string{
			"config", "create", sourceName, "smb",
			"host", config.SourceHost,
			"user", config.SourceUser,
			"pass", config.SourcePassword,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.SourceDomain != "" {
			args = append(args, "domain", config.SourceDomain)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "ftp":
		args := []string{
			"config", "create", sourceName, "ftp",
			"host", config.SourceHost,
			"user", config.SourceUser,
			"pass", config.SourcePassword,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.SourcePassiveMode != nil && *config.SourcePassiveMode {
			args = append(args, "passive", "true")
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "webdav":
		args := []string{
			"config", "create", sourceName, "webdav",
			"url", config.SourceHost,
			"user", config.SourceUser,
			"pass", config.SourcePassword,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "nextcloud":
		args := []string{
			"config", "create", sourceName, "webdav",
			"url", config.SourceHost,
			"user", config.SourceUser,
			"pass", config.SourcePassword,
			"vendor", "nextcloud",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "onedrive":
		args := []string{
			"config", "create", sourceName, "onedrive",
			"client_id", config.SourceClientID,
			"client_secret", config.SourceClientSecret,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.SourceDriveID != "" {
			args = append(args, "drive_id", config.SourceDriveID)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "gdrive":
		args := []string{
			"config", "create", sourceName, "drive",
			"client_id", config.SourceClientID,
			"client_secret", config.SourceClientSecret,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.SourceTeamDrive != "" {
			args = append(args, "team_drive", config.SourceTeamDrive)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	case "gphotos":
		args := []string{
			"config", "create", sourceName, "google photos",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Only add client_id and client_secret if they're provided (not empty)
		// This allows using rclone's built-in authentication
		if config.SourceClientID != "" && config.SourceClientSecret != "" {
			args = append(args, "client_id", config.SourceClientID)
			args = append(args, "client_secret", config.SourceClientSecret)
		}

		// Add read_only option if specified
		if config.SourceReadOnly != nil && *config.SourceReadOnly {
			args = append(args, "read_only", "true")
		}

		// Add start_year if specified
		if config.SourceStartYear > 0 {
			args = append(args, "start_year", strconv.Itoa(config.SourceStartYear))
		}

		// Add include_archived if specified
		if config.SourceIncludeArchived != nil && *config.SourceIncludeArchived {
			args = append(args, "include_archived", "true")
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create source config: %v\nOutput: %s", err, output)
		}
	default:
		// Write local config
		content := fmt.Sprintf("[source_%d]\ntype = local\n\n", config.ID)
		if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write source config: %v", err)
		}
	}

	destName := fmt.Sprintf("dest_%d", config.ID)
	switch config.DestinationType {
	case "sftp":
		args := []string{
			"config", "create", destName, "sftp",
			"host", config.DestHost,
			"user", config.DestUser,
			"port", fmt.Sprintf("%d", config.DestPort),
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}
		if config.DestPassword != "" {
			args = append(args, "pass", config.DestPassword)
		}
		if config.DestKeyFile != "" {
			args = append(args, "key_file", config.DestKeyFile)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "s3":
		args := []string{
			"config", "create", destName, "s3",
			"provider", "AWS",
			"env_auth", "false",
			"access_key_id", config.DestAccessKey,
			"secret_access_key", config.DestSecretKey,
			"region", config.DestRegion,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.DestEndpoint != "" {
			args = append(args, "endpoint", config.DestEndpoint)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "minio":
		args := []string{
			"config", "create", destName, "s3",
			"provider", "Minio",
			"env_auth", "false",
			"access_key_id", config.DestAccessKey,
			"secret_access_key", config.DestSecretKey,
			"endpoint", config.DestEndpoint,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "b2":
		args := []string{
			"config", "create", destName, "b2",
			"account", config.DestAccessKey,
			"key", config.DestSecretKey,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "smb":
		args := []string{
			"config", "create", destName, "smb",
			"host", config.DestHost,
			"user", config.DestUser,
			"pass", config.DestPassword,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.DestDomain != "" {
			args = append(args, "domain", config.DestDomain)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "ftp":
		args := []string{
			"config", "create", destName, "ftp",
			"host", config.DestHost,
			"user", config.DestUser,
			"pass", config.DestPassword,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.DestPassiveMode != nil && *config.DestPassiveMode {
			args = append(args, "passive", "true")
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "webdav":
		args := []string{
			"config", "create", destName, "webdav",
			"url", config.DestHost,
			"user", config.DestUser,
			"pass", config.DestPassword,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "nextcloud":
		args := []string{
			"config", "create", destName, "webdav",
			"url", config.DestHost,
			"user", config.DestUser,
			"pass", config.DestPassword,
			"vendor", "nextcloud",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "onedrive":
		args := []string{
			"config", "create", destName, "onedrive",
			"client_id", config.DestClientID,
			"client_secret", config.DestClientSecret,
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		if config.DestDriveID != "" {
			args = append(args, "drive_id", config.DestDriveID)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "gdrive":
		args := []string{
			"config", "create", destName, "drive",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Only add client_id and client_secret if they're provided (not empty)
		// This allows using rclone's built-in authentication
		if config.DestClientID != "" && config.DestClientSecret != "" {
			args = append(args, "client_id", config.DestClientID)
			args = append(args, "client_secret", config.DestClientSecret)
		}

		if config.DestTeamDrive != "" {
			args = append(args, "team_drive", config.DestTeamDrive)
		}

		if config.DestDriveID != "" {
			args = append(args, "root_folder_id", config.DestDriveID)
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	case "gphotos":
		args := []string{
			"config", "create", destName, "google photos",
			"--non-interactive",
			"--config", configPath,
			"--log-level", "ERROR",
		}

		// Only add client_id and client_secret if they're provided (not empty)
		// This allows using rclone's built-in authentication
		if config.DestClientID != "" && config.DestClientSecret != "" {
			args = append(args, "client_id", config.DestClientID)
			args = append(args, "client_secret", config.DestClientSecret)
		}

		// Add read_only option if specified
		if config.DestReadOnly != nil && *config.DestReadOnly {
			args = append(args, "read_only", "true")
		}

		// Add start_year if specified
		if config.DestStartYear > 0 {
			args = append(args, "start_year", strconv.Itoa(config.DestStartYear))
		}

		// Add include_archived if specified
		if config.DestIncludeArchived != nil && *config.DestIncludeArchived {
			args = append(args, "include_archived", "true")
		}

		cmd := exec.Command(rclonePath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create destination config: %v\nOutput: %s", err, output)
		}
	default:
		// Append local config
		content := fmt.Sprintf("[dest_%d]\ntype = local\n", config.ID)
		f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to open config file: %v", err)
		}
		defer f.Close()
		if _, err := f.WriteString(content); err != nil {
			return fmt.Errorf("failed to write destination config: %v", err)
		}
	}

	return nil
}

// GetActiveJobs returns all active jobs
func (db *DB) GetActiveJobs() ([]Job, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	var jobs []Job
	// For boolean pointer fields, need to check either NULL (for default) or true value
	err := db.Preload("Config").Where("enabled IS NULL OR enabled = ?", true).Find(&jobs).Error
	return jobs, err
}

// GetConfigsForJob returns all transfer configurations associated with a job
func (db *DB) GetConfigsForJob(jobID uint) ([]TransferConfig, error) {
	var job Job
	if err := db.First(&job, jobID).Error; err != nil {
		return nil, err
	}

	// Get the list of config IDs
	configIDs := job.GetConfigIDsList()
	if len(configIDs) == 0 {
		// If there are no IDs in the list but there is a configID, use that
		if job.ConfigID > 0 {
			configIDs = []uint{job.ConfigID}
		} else {
			return []TransferConfig{}, nil
		}
	}

	// Fetch all configs
	var configs []TransferConfig
	if err := db.Where("id IN ?", configIDs).Find(&configs).Error; err != nil {
		return nil, err
	}

	return configs, nil
}

// GetSkipProcessedFiles returns the value of SkipProcessedFiles with a default if nil
func (tc *TransferConfig) GetSkipProcessedFiles() bool {
	if tc.SkipProcessedFiles == nil {
		return true // Default to true if not set
	}
	return *tc.SkipProcessedFiles
}

// SetSkipProcessedFiles sets the SkipProcessedFiles field
func (tc *TransferConfig) SetSkipProcessedFiles(value bool) {
	tc.SkipProcessedFiles = &value
}

// StoreGoogleDriveToken stores the Google Drive auth token for a config
func (db *DB) StoreGoogleDriveToken(configIDStr string, token string) error {
	configID, err := strconv.ParseUint(configIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid config ID: %v", err)
	}

	// Get the existing config
	config, err := db.GetTransferConfig(uint(configID))
	if err != nil {
		return fmt.Errorf("failed to get config: %v", err)
	}

	// Mark as authenticated
	authenticated := true
	config.GoogleDriveAuthenticated = &authenticated

	// Update the config in the database
	if err := db.UpdateTransferConfig(config); err != nil {
		return fmt.Errorf("failed to update config: %v", err)
	}

	// Get the rclone config path
	configPath := db.GetConfigRclonePath(config)

	// Read existing config if it exists
	existingConfig := ""
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read existing config: %v", err)
		}
		existingConfig = string(data)
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Write the new config with the token
	destName := fmt.Sprintf("dest_%d", config.ID)
	newConfig := fmt.Sprintf("[%s]\ntype = drive\ntoken = %s\n", destName, token)

	// If the config has client ID and secret, add them
	if config.DestClientID != "" && config.DestClientSecret != "" {
		newConfig += fmt.Sprintf("client_id = %s\nclient_secret = %s\n", config.DestClientID, config.DestClientSecret)
	}

	// Add root folder ID if specified
	if config.DestDriveID != "" {
		newConfig += fmt.Sprintf("root_folder_id = %s\n", config.DestDriveID)
	}

	// Add team drive if specified
	if config.DestTeamDrive != "" {
		newConfig += fmt.Sprintf("team_drive = %s\n", config.DestTeamDrive)
	}

	// If there's existing config, append to it; otherwise create new file
	var content string
	if existingConfig != "" {
		// Replace/update existing dest section if it exists, otherwise append
		if strings.Contains(existingConfig, fmt.Sprintf("[%s]", destName)) {
			// This is a simplistic approach - in production you might want a more robust regex-based replacement
			// Truncate at the beginning of the dest section
			parts := strings.SplitN(existingConfig, fmt.Sprintf("[%s]", destName), 2)
			// Check if there are more sections after this one
			nextSectionIdx := strings.Index(parts[1], "[")
			if nextSectionIdx != -1 {
				content = parts[0] + newConfig + parts[1][nextSectionIdx:]
			} else {
				content = parts[0] + newConfig
			}
		} else {
			content = existingConfig + "\n" + newConfig
		}
	} else {
		content = newConfig
	}

	// Write the config file
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

// GenerateRcloneConfigWithToken generates a rclone config file for a transfer config with a provided token
func (db *DB) GenerateRcloneConfigWithToken(config *TransferConfig, token string) error {
	// Get the config path
	configPath := db.GetConfigRclonePath(config)
	if configPath == "" {
		return fmt.Errorf("failed to get config path")
	}

	// Clean up the token to ensure it's a single line JSON
	token = strings.TrimSpace(token)
	token = strings.ReplaceAll(token, "\n", "")
	token = strings.ReplaceAll(token, "\r", "")

	// Determine if this is a source or destination config
	var configType, section, clientID, clientSecret string
	var readOnly, includeArchived *bool
	var startYear int

	if config.DestinationType == "gdrive" || config.DestinationType == "gphotos" {
		configType = config.DestinationType
		section = "dest"
		clientID = config.DestClientID
		clientSecret = config.DestClientSecret
		readOnly = config.DestReadOnly
		startYear = config.DestStartYear
		includeArchived = config.DestIncludeArchived
	} else if config.SourceType == "gdrive" || config.SourceType == "gphotos" {
		configType = config.SourceType
		section = "source"
		clientID = config.SourceClientID
		clientSecret = config.SourceClientSecret
		readOnly = config.SourceReadOnly
		startYear = config.SourceStartYear
		includeArchived = config.SourceIncludeArchived
	} else {
		return fmt.Errorf("config is not for Google Drive or Google Photos")
	}

	// Read the existing config
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Prepare the section content
	var sectionContent string
	if configType == "gdrive" {
		sectionContent = fmt.Sprintf("[%s_%d]\ntype = drive\n", section, config.ID)
		if clientID != "" {
			sectionContent += fmt.Sprintf("client_id = %s\n", clientID)
		}
		if clientSecret != "" {
			sectionContent += fmt.Sprintf("client_secret = %s\n", clientSecret)
		}
		sectionContent += fmt.Sprintf("token = %s\n", token)

		// Add team drive if specified
		if section == "source" && config.SourceTeamDrive != "" {
			sectionContent += fmt.Sprintf("team_drive = %s\n", config.SourceTeamDrive)
		} else if section == "dest" && config.DestTeamDrive != "" {
			sectionContent += fmt.Sprintf("team_drive = %s\n", config.DestTeamDrive)
		}

		// Add read-only flag if specified
		if readOnly != nil && *readOnly {
			sectionContent += "read_only = true\n"
		}
	} else if configType == "gphotos" {
		sectionContent = fmt.Sprintf("[%s_%d]\ntype = google photos\n", section, config.ID)
		if clientID != "" {
			sectionContent += fmt.Sprintf("client_id = %s\n", clientID)
		}
		if clientSecret != "" {
			sectionContent += fmt.Sprintf("client_secret = %s\n", clientSecret)
		}
		sectionContent += fmt.Sprintf("token = %s\n", token)

		// Add read-only flag if specified
		if readOnly != nil && *readOnly {
			sectionContent += "read_only = true\n"
		}

		// Add start year if specified
		if startYear > 0 {
			sectionContent += fmt.Sprintf("start_year = %d\n", startYear)
		}

		// Add include_archived flag if specified and true
		if includeArchived != nil && *includeArchived {
			sectionContent += "include_archived = true\n"
		}
	}

	// Find the section in the existing config
	sectionPattern := regexp.MustCompile(fmt.Sprintf(`\[%s_%d\][^\[]*`, section, config.ID))
	if sectionPattern.MatchString(string(content)) {
		// Replace the existing section
		newContent := sectionPattern.ReplaceAllString(string(content), sectionContent)
		err = os.WriteFile(configPath, []byte(newContent), 0644)
		if err != nil {
			return fmt.Errorf("failed to write updated config file: %v", err)
		}
	} else {
		// Append the section to the config
		file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open config file for appending: %v", err)
		}
		defer file.Close()

		_, err = file.WriteString("\n" + sectionContent)
		if err != nil {
			return fmt.Errorf("failed to append to config file: %v", err)
		}
	}

	// Update the authentication status
	authenticated := true
	if config.DestinationType == "gdrive" || config.DestinationType == "gphotos" {
		config.SetGoogleAuthenticated(authenticated)
	} else if config.SourceType == "gdrive" || config.SourceType == "gphotos" {
		config.SetGoogleAuthenticated(authenticated)
	}

	return nil
}

// GetIsAdmin returns the value of IsAdmin with a default if nil
func (u *User) GetIsAdmin() bool {
	if u.IsAdmin == nil {
		return false // Default to false if not set
	}
	return *u.IsAdmin
}

// SetIsAdmin sets the IsAdmin field
func (u *User) SetIsAdmin(value bool) {
	u.IsAdmin = &value
}

// GetAccountLocked returns the value of AccountLocked with a default if nil
func (u *User) GetAccountLocked() bool {
	if u.AccountLocked == nil {
		return false // Default to false if not set
	}
	return *u.AccountLocked
}

// SetAccountLocked sets the AccountLocked field
func (u *User) SetAccountLocked(value bool) {
	u.AccountLocked = &value
}

// GetUsed returns the value of Used with a default if nil
func (t *PasswordResetToken) GetUsed() bool {
	if t.Used == nil {
		return false // Default to false if not set
	}
	return *t.Used
}

// SetUsed sets the Used field
func (t *PasswordResetToken) SetUsed(value bool) {
	t.Used = &value
}

// GetSourcePassiveMode returns the value of SourcePassiveMode with a default if nil
func (tc *TransferConfig) GetSourcePassiveMode() bool {
	if tc.SourcePassiveMode == nil {
		return true // Default to true if not set
	}
	return *tc.SourcePassiveMode
}

// SetSourcePassiveMode sets the SourcePassiveMode field
func (tc *TransferConfig) SetSourcePassiveMode(value bool) {
	tc.SourcePassiveMode = &value
}

// GetDestPassiveMode returns the value of DestPassiveMode with a default if nil
func (tc *TransferConfig) GetDestPassiveMode() bool {
	if tc.DestPassiveMode == nil {
		return true // Default to true if not set
	}
	return *tc.DestPassiveMode
}

// SetDestPassiveMode sets the DestPassiveMode field
func (tc *TransferConfig) SetDestPassiveMode(value bool) {
	tc.DestPassiveMode = &value
}

// GetGoogleDriveAuthenticated returns whether the transfer config has been authenticated with Google Drive
func (tc *TransferConfig) GetGoogleDriveAuthenticated() bool {
	return tc.GoogleDriveAuthenticated != nil && *tc.GoogleDriveAuthenticated
}

// SetGoogleDriveAuthenticated sets the Google Drive authentication status
func (tc *TransferConfig) SetGoogleDriveAuthenticated(value bool) {
	tc.GoogleDriveAuthenticated = &value
}

// GetGoogleAuthenticated is an alias for GetGoogleDriveAuthenticated for better semantics when working with Google Photos
func (tc *TransferConfig) GetGoogleAuthenticated() bool {
	return tc.GetGoogleDriveAuthenticated()
}

// SetGoogleAuthenticated is an alias for SetGoogleDriveAuthenticated for better semantics when working with Google Photos
func (tc *TransferConfig) SetGoogleAuthenticated(value bool) {
	tc.SetGoogleDriveAuthenticated(value)
}

// GetArchiveEnabled returns the value of ArchiveEnabled with a default if nil
func (tc *TransferConfig) GetArchiveEnabled() bool {
	if tc.ArchiveEnabled == nil {
		return false // Default to false if not set
	}
	return *tc.ArchiveEnabled
}

// SetArchiveEnabled sets the ArchiveEnabled field
func (tc *TransferConfig) SetArchiveEnabled(value bool) {
	tc.ArchiveEnabled = &value
}

// GetDeleteAfterTransfer returns the value of DeleteAfterTransfer with a default if nil
func (tc *TransferConfig) GetDeleteAfterTransfer() bool {
	if tc.DeleteAfterTransfer == nil {
		return false // Default to false if not set
	}
	return *tc.DeleteAfterTransfer
}

// SetDeleteAfterTransfer sets the DeleteAfterTransfer field
func (tc *TransferConfig) SetDeleteAfterTransfer(value bool) {
	tc.DeleteAfterTransfer = &value
}

// GetEnabled returns the value of Enabled with a default if nil
func (j *Job) GetEnabled() bool {
	if j.Enabled == nil {
		return true // Default to true if not set
	}
	return *j.Enabled
}

// SetEnabled sets the Enabled field
func (j *Job) SetEnabled(value bool) {
	j.Enabled = &value
}

// GetWebhookEnabled returns the value of WebhookEnabled with a default if nil
func (j *Job) GetWebhookEnabled() bool {
	if j.WebhookEnabled == nil {
		return false // Default to false if not set
	}
	return *j.WebhookEnabled
}

// SetWebhookEnabled sets the WebhookEnabled field
func (j *Job) SetWebhookEnabled(value bool) {
	j.WebhookEnabled = &value
}

// GetNotifyOnSuccess returns the value of NotifyOnSuccess with a default if nil
func (j *Job) GetNotifyOnSuccess() bool {
	if j.NotifyOnSuccess == nil {
		return true // Default to true if not set
	}
	return *j.NotifyOnSuccess
}

// SetNotifyOnSuccess sets the NotifyOnSuccess field
func (j *Job) SetNotifyOnSuccess(value bool) {
	j.NotifyOnSuccess = &value
}

// GetNotifyOnFailure returns the value of NotifyOnFailure with a default if nil
func (j *Job) GetNotifyOnFailure() bool {
	if j.NotifyOnFailure == nil {
		return true // Default to true if not set
	}
	return *j.NotifyOnFailure
}

// SetNotifyOnFailure sets the NotifyOnFailure field
func (j *Job) SetNotifyOnFailure(value bool) {
	j.NotifyOnFailure = &value
}

// GetGDriveCredentialsFromConfig extracts Google Drive client ID and secret from an existing rclone config file
func (db *DB) GetGDriveCredentialsFromConfig(config *TransferConfig) (string, string) {
	configPath := db.GetConfigRclonePath(config)
	if configPath == "" {
		return "", ""
	}

	// Check if the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", ""
	}

	// Read the rclone config file
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", ""
	}

	// Parse the content to extract client_id and client_secret from both source and destination sections
	lines := strings.Split(string(content), "\n")

	// Define section names based on config ID
	sourceSectionName := fmt.Sprintf("[source_%d]", config.ID)
	destSectionName := fmt.Sprintf("[dest_%d]", config.ID)

	var inSourceSection, inDestSection bool
	var sourceClientID, sourceClientSecret, destClientID, destClientSecret string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check if we're entering a new section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSourceSection = line == sourceSectionName
			inDestSection = line == destSectionName
			continue
		}

		// Extract credentials from source section
		if inSourceSection {
			if strings.HasPrefix(line, "client_id") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					sourceClientID = strings.TrimSpace(parts[1])
				}
			} else if strings.HasPrefix(line, "client_secret") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					sourceClientSecret = strings.TrimSpace(parts[1])
				}
			}
		}

		// Extract credentials from destination section
		if inDestSection {
			if strings.HasPrefix(line, "client_id") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					destClientID = strings.TrimSpace(parts[1])
				}
			} else if strings.HasPrefix(line, "client_secret") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					destClientSecret = strings.TrimSpace(parts[1])
				}
			}
		}

		// If we found both values in both sections, we can stop processing
		if sourceClientID != "" && sourceClientSecret != "" && destClientID != "" && destClientSecret != "" {
			break
		}
	}

	// Prefer destination credentials since we're authenticating for destination
	if destClientID != "" && destClientSecret != "" {
		return destClientID, destClientSecret
	}

	// Fall back to source credentials if available
	if sourceClientID != "" && sourceClientSecret != "" {
		return sourceClientID, sourceClientSecret
	}

	return "", ""
}

// GetUseBuiltinAuthSource returns the value of UseBuiltinAuthSource with a default if nil
func (tc *TransferConfig) GetUseBuiltinAuthSource() bool {
	if tc.UseBuiltinAuthSource == nil {
		return true // Default to true if not set
	}
	return *tc.UseBuiltinAuthSource
}

// SetUseBuiltinAuthSource sets the UseBuiltinAuthSource field
func (tc *TransferConfig) SetUseBuiltinAuthSource(value bool) {
	tc.UseBuiltinAuthSource = &value
}

// GetUseBuiltinAuthDest returns the value of UseBuiltinAuthDest with a default if nil
func (tc *TransferConfig) GetUseBuiltinAuthDest() bool {
	if tc.UseBuiltinAuthDest == nil {
		return true // Default to true if not set
	}
	return *tc.UseBuiltinAuthDest
}

// SetUseBuiltinAuthDest sets the UseBuiltinAuthDest field
func (tc *TransferConfig) SetUseBuiltinAuthDest(value bool) {
	tc.UseBuiltinAuthDest = &value
}
