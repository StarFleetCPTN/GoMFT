package rclone_service

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
)

// TestRcloneConnection attempts to connect to a provider using temporary config created via `rclone config create`.
// It returns success (bool), a message (string), and an error.
func TestRcloneConnection(config db.TransferConfig, providerType string, dbInstance *db.DB) (bool, string, error) {
	var remoteName string
	var remotePath string
	var provider string
	// Removed bucket, share as they are not used in current rclone config create args
	var host, user, pass, keyFile, region, accessKey, secretKey, endpoint, domain, clientID, clientSecret, driveID, teamDrive string
	var port int
	// Add other necessary fields like passiveMode, readOnly etc. if needed by rclone config create for specific types
	var err error

	// Create a temporary directory for the config file
	tempDir, err := os.MkdirTemp("", "gomft-rclone-test-")
	if err != nil {
		return false, "Failed to create temp directory for rclone config", err
	}
	defer os.RemoveAll(tempDir) // Clean up the temp directory
	tempConfigPath := filepath.Join(tempDir, "rclone_test.conf")

	// Extract parameters based on providerType
	if providerType == "source" {
		remoteName = "testSource"
		remotePath = config.SourcePath
		provider = config.SourceType
		host = config.SourceHost
		port = config.SourcePort
		user = config.SourceUser
		pass = config.SourcePassword
		keyFile = config.SourceKeyFile
		// bucket = config.SourceBucket // Removed - Not used in create args yet
		region = config.SourceRegion
		accessKey = config.SourceAccessKey
		secretKey = config.SourceSecretKey
		endpoint = config.SourceEndpoint
		// share = config.SourceShare // Removed - Not used in create args yet
		domain = config.SourceDomain
		clientID = config.SourceClientID
		clientSecret = config.SourceClientSecret
		driveID = config.SourceDriveID
		teamDrive = config.SourceTeamDrive
		// Extract other source fields as needed (e.g., passiveMode, readOnly)
	} else if providerType == "destination" {
		remoteName = "testDest"
		remotePath = config.DestinationPath
		provider = config.DestinationType
		host = config.DestHost
		port = config.DestPort
		user = config.DestUser
		pass = config.DestPassword
		keyFile = config.DestKeyFile
		// bucket = config.DestBucket // Removed - Not used in create args yet
		region = config.DestRegion
		accessKey = config.DestAccessKey
		secretKey = config.DestSecretKey
		endpoint = config.DestEndpoint
		// share = config.DestShare // Removed - Not used in create args yet
		domain = config.DestDomain
		clientID = config.DestClientID
		clientSecret = config.DestClientSecret
		driveID = config.DestDriveID
		teamDrive = config.DestTeamDrive
		// Extract other dest fields as needed (e.g., passiveMode, readOnly)
	} else {
		return false, "Invalid provider type specified", fmt.Errorf("unknown provider type: %s", providerType)
	}

	// Get the rclone path from the environment variable or use the default path
	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	// --- Use `rclone config create` to generate the temporary config section ---
	createArgs := []string{
		"config", "create", remoteName, provider,
		"--config", tempConfigPath,
		"--non-interactive",
		"--log-level", "DEBUG", // Use DEBUG for create to see details if it fails
	}

	// --- Declare variables for lsd command *before* the switch/goto ---
	var ctx context.Context
	var cancel context.CancelFunc
	var lsdArgs []string
	var stdout, stderr bytes.Buffer
	var lsdCmd *exec.Cmd
	var createCmd *exec.Cmd // Declare createCmd here as well

	// Add provider-specific arguments
	// Mirroring logic from db.GenerateRcloneConfig but passing args to CLI
	switch provider {
	case "sftp":
		createArgs = append(createArgs, "host", host, "user", user)
		if port != 0 {
			createArgs = append(createArgs, "port", fmt.Sprintf("%d", port))
		}
		if pass != "" {
			createArgs = append(createArgs, "pass", pass) // Pass directly
		}
		if keyFile != "" {
			createArgs = append(createArgs, "key_file", keyFile)
			// Note: Passphrase for keyfile might need 'pass' too, rclone handles this context.
		}
		// Add other SFTP args like ssh_agent, use_insecure_cipher etc. if needed
	case "s3":
		createArgs = append(createArgs, "provider", "AWS", "env_auth", "false") // Assume AWS for generic S3
		if accessKey != "" {
			createArgs = append(createArgs, "access_key_id", accessKey)
		}
		if secretKey != "" {
			createArgs = append(createArgs, "secret_access_key", secretKey) // Pass directly
		}
		if region != "" {
			createArgs = append(createArgs, "region", region)
		}
		if endpoint != "" {
			createArgs = append(createArgs, "endpoint", endpoint)
		}
		// Add other S3 args like acl, storage_class if needed
	case "minio":
		createArgs = append(createArgs, "provider", "Minio", "env_auth", "false")
		if accessKey != "" {
			createArgs = append(createArgs, "access_key_id", accessKey)
		}
		if secretKey != "" {
			createArgs = append(createArgs, "secret_access_key", secretKey) // Pass directly
		}
		if endpoint != "" {
			createArgs = append(createArgs, "endpoint", endpoint)
		}
		if region != "" { // Minio might ignore region, but add if present
			createArgs = append(createArgs, "region", region)
		}
	case "ftp":
		createArgs = append(createArgs, "host", host, "user", user)
		if port != 0 {
			createArgs = append(createArgs, "port", fmt.Sprintf("%d", port))
		}
		if pass != "" {
			createArgs = append(createArgs, "pass", pass)
		}
		// Add explicit_tls, passive_mode based on config boolean pointers
		if config.GetSourcePassiveMode() || config.GetDestPassiveMode() { // Check based on providerType
			createArgs = append(createArgs, "passive_mode", "true")
		} else {
			createArgs = append(createArgs, "passive_mode", "false")
		}
		createArgs = append(createArgs, "explicit_tls", "true") // Defaulting to true
	case "smb":
		createArgs = append(createArgs, "host", host, "user", user)
		if port != 0 {
			createArgs = append(createArgs, "port", fmt.Sprintf("%d", port))
		}
		if pass != "" {
			createArgs = append(createArgs, "pass", pass)
		}
		if domain != "" {
			createArgs = append(createArgs, "domain", domain)
		}
	case "webdav":
		createArgs = append(createArgs, "url", endpoint, "vendor", "other", "user", user) // Default vendor
		if pass != "" {
			createArgs = append(createArgs, "pass", pass)
		}
	case "nextcloud":
		createArgs = append(createArgs, "url", endpoint, "vendor", "nextcloud", "user", user)
		if pass != "" {
			createArgs = append(createArgs, "pass", pass)
		}
	// Add cases for gdrive, gphotos - these are complex due to token handling
	// For testing, they might require pre-existing tokens or manual auth flow outside this scope.
	// Passing client_id/secret might work for initial setup but not subsequent tests without a token.
	case "gdrive":
		createArgs = append(createArgs, "scope", "drive") // Default scope
		if clientID != "" {
			createArgs = append(createArgs, "client_id", clientID)
		}
		if clientSecret != "" {
			createArgs = append(createArgs, "client_secret", clientSecret)
		}
		if driveID != "" { // Use root_folder_id for specific drive/folder
			createArgs = append(createArgs, "root_folder_id", driveID)
		}
		if teamDrive != "" {
			createArgs = append(createArgs, "team_drive", teamDrive)
		}
		// Cannot pass token directly via 'config create' easily for testing non-interactive flow.
		log.Println("Warning: Google Drive test may require pre-existing token or manual auth.")

	case "gphotos":
		// Similar complexity to gdrive regarding tokens
		if clientID != "" {
			createArgs = append(createArgs, "client_id", clientID)
		}
		if clientSecret != "" {
			createArgs = append(createArgs, "client_secret", clientSecret)
		}
		// Add read_only, start_year, include_archived based on config
		// Cannot pass token directly via 'config create' easily for testing non-interactive flow.
		log.Println("Warning: Google Photos test may require pre-existing token or manual auth.")

	case "local":
		// 'rclone config create' might not be needed or work well for 'local' type.
		// Write a minimal config manually for local.
		localConfigContent := fmt.Sprintf("[%s]\ntype = local\nnounc = true\n", remoteName)
		if err := os.WriteFile(tempConfigPath, []byte(localConfigContent), 0600); err != nil {
			return false, fmt.Sprintf("Failed to write temporary local config: %v", err), err
		}
		goto RunLsd // Skip rclone config create for local
	default:
		return false, fmt.Sprintf("Provider type '%s' not yet supported for testing via 'rclone config create'", provider), fmt.Errorf("unsupported provider")
	}

	log.Printf("Executing rclone config create command: %s %s", rclonePath, strings.Join(createArgs, " "))
	createCmd = exec.Command(rclonePath, createArgs...) // Assign value using =
	if output, err := createCmd.CombinedOutput(); err != nil {
		// Log the config file content on error for debugging
		configContentBytes, _ := os.ReadFile(tempConfigPath)
		log.Printf("Temp config content on create error:\n---\n%s\n---", string(configContentBytes))
		return false, fmt.Sprintf("Failed to create temp config section: %v\nOutput: %s", err, string(output)), err
	}
	log.Printf("Successfully created temp config section for %s", remoteName)

	// Declarations moved before the switch statement

RunLsd: // Label to jump to for local type

	// --- Execute `rclone lsd` using the temporary config ---
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second) // Assign values
	defer cancel()

	lsdArgs = []string{ // Assign value
		"--config", tempConfigPath,
		"lsd",
		fmt.Sprintf("%s:%s", remoteName, remotePath),
		"--low-level-retries", "1",
		"--retries", "1",
		// Add -vv for verbose logging during test if needed
		// "-vv",
	}

	log.Printf("Executing rclone lsd command: %s %s", rclonePath, strings.Join(lsdArgs, " "))
	lsdCmd = exec.CommandContext(ctx, rclonePath, lsdArgs...) // Assign value

	// var stdout, stderr bytes.Buffer // Moved declaration up
	lsdCmd.Stdout = &stdout // Assign buffer
	lsdCmd.Stderr = &stderr // Assign buffer

	err = lsdCmd.Run() // Assign error

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	log.Printf("Rclone lsd stdout:\n%s", stdoutStr)
	log.Printf("Rclone lsd stderr:\n%s", stderrStr)

	if ctx.Err() == context.DeadlineExceeded {
		return false, "Connection test timed out after 30 seconds.", ctx.Err()
	}

	if err != nil {
		// Try to provide a more specific error message based on lsd output
		errMsg := fmt.Sprintf("Connection test failed: %v. Stderr: %s", err, stderrStr)
		// Add specific error checks based on stderrStr if needed
		if strings.Contains(stderrStr, "connect: connection refused") {
			errMsg = "Connection test failed: Connection refused by host."
		} else if strings.Contains(stderrStr, "no such host") || strings.Contains(stderrStr, "name resolution error") {
			errMsg = "Connection test failed: Hostname not found or DNS resolution error."
		} else if strings.Contains(stderrStr, "authentication failed") || strings.Contains(stderrStr, "login incorrect") || strings.Contains(stderrStr, "permission denied") {
			errMsg = "Connection test failed: Authentication failed (check credentials/permissions)."
		} else if strings.Contains(stderrStr, "directory not found") {
			errMsg = "Connection test failed: Directory/Path not found (check path)."
		} else if strings.Contains(stderrStr, "Couldn't find section") { // Error from rclone config create
			errMsg = "Connection test failed: Invalid parameters provided for provider type."
		}
		return false, errMsg, err
	}

	// If lsd runs without error, the connection is likely okay
	return true, "Connection test successful!", nil
}
