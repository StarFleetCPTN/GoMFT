package rclone_service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
)

// --- Mockable os/exec ---

// execCommandContext allows mocking exec.CommandContext during tests.
var execCommandContext = exec.CommandContext

// cmdCombinedOutput allows mocking the CombinedOutput method during tests.
var cmdCombinedOutput = (*exec.Cmd).CombinedOutput

// cmdRun allows mocking the Run method during tests.
var cmdRun = (*exec.Cmd).Run

// --- Function Implementation ---

// TestRcloneConnection attempts to connect to a provider using temporary config created via `rclone config create`.
// It returns success (bool), a message (string), and an error.
func TestRcloneConnection(config db.TransferConfig, providerType string, dbInstance *db.DB) (bool, string, error) {
	var remoteName string
	var remotePath string
	var provider string
	var host, user, pass, keyFile, region, accessKey, secretKey, endpoint, domain, clientID, clientSecret, driveID, teamDrive string
	var port int
	var err error

	tempDir, err := os.MkdirTemp("", "gomft-rclone-test-")
	if err != nil {
		return false, "Failed to create temp directory for rclone config", err
	}
	defer os.RemoveAll(tempDir)
	tempConfigPath := filepath.Join(tempDir, "rclone_test.conf")

	if providerType == "source" {
		remoteName = "testSource"
		remotePath = config.SourcePath
		provider = config.SourceType
		host = config.SourceHost
		port = config.SourcePort
		user = config.SourceUser
		pass = config.SourcePassword
		keyFile = config.SourceKeyFile
		region = config.SourceRegion
		accessKey = config.SourceAccessKey
		secretKey = config.SourceSecretKey
		endpoint = config.SourceEndpoint
		domain = config.SourceDomain
		clientID = config.SourceClientID
		clientSecret = config.SourceClientSecret
		driveID = config.SourceDriveID
		teamDrive = config.SourceTeamDrive
	} else if providerType == "destination" {
		remoteName = "testDest"
		remotePath = config.DestinationPath
		provider = config.DestinationType
		host = config.DestHost
		port = config.DestPort
		user = config.DestUser
		pass = config.DestPassword
		keyFile = config.DestKeyFile
		region = config.DestRegion
		accessKey = config.DestAccessKey
		secretKey = config.DestSecretKey
		endpoint = config.DestEndpoint
		domain = config.DestDomain
		clientID = config.DestClientID
		clientSecret = config.DestClientSecret
		driveID = config.DestDriveID
		teamDrive = config.DestTeamDrive
	} else {
		return false, "Invalid provider type specified", fmt.Errorf("unknown provider type: %s", providerType)
	}

	rclonePath := os.Getenv("RCLONE_PATH")
	if rclonePath == "" {
		rclonePath = "rclone"
	}

	if provider == "hetzner" {
		provider = "sftp"
	}

	createArgs := []string{
		"config", "create", remoteName, provider,
		"--config", tempConfigPath,
		"--non-interactive",
		"--log-level", "DEBUG",
	}

	var ctx context.Context
	var cancel context.CancelFunc
	var lsdArgs []string
	var stdout, stderr bytes.Buffer
	var lsdCmd *exec.Cmd
	var createCmd *exec.Cmd

	switch provider {
	case "sftp", "hetzner":
		createArgs = append(createArgs, "host", host, "user", user)
		if port != 0 {
			createArgs = append(createArgs, "port", fmt.Sprintf("%d", port))
		}
		if pass != "" {
			createArgs = append(createArgs, "pass", pass)
		}
		if keyFile != "" {
			createArgs = append(createArgs, "key_file", keyFile)
		}
	case "s3":
		createArgs = append(createArgs, "provider", "AWS", "env_auth", "false")
		if accessKey != "" {
			createArgs = append(createArgs, "access_key_id", accessKey)
		}
		if secretKey != "" {
			createArgs = append(createArgs, "secret_access_key", secretKey)
		}
		if region != "" {
			createArgs = append(createArgs, "region", region)
		}
		if endpoint != "" {
			createArgs = append(createArgs, "endpoint", endpoint)
		}
	case "wasabi":
		createArgs = append(createArgs, "provider", "Wasabi", "env_auth", "false")
		if accessKey != "" {
			createArgs = append(createArgs, "access_key_id", accessKey)
		}
		if secretKey != "" {
			createArgs = append(createArgs, "secret_access_key", secretKey)
		}
		if region != "" {
			createArgs = append(createArgs, "region", region)
		}
		endpointValue := endpoint
		if endpointValue == "" {
			endpointValue = "s3.wasabisys.com"
		}
		createArgs = append(createArgs, "endpoint", endpointValue)
	case "b2":
		createArgs = append(createArgs, "provider", "B2", "env_auth", "false")
		if accessKey != "" {
			createArgs = append(createArgs, "account", accessKey)
		}
		if secretKey != "" {
			createArgs = append(createArgs, "key", secretKey)
		}
		if endpoint != "" {
			createArgs = append(createArgs, "endpoint", endpoint)
		}
		if region != "" {
			createArgs = append(createArgs, "region", region)
		}
	case "minio":
		createArgs = append(createArgs, "provider", "Minio", "env_auth", "false")
		if accessKey != "" {
			createArgs = append(createArgs, "access_key_id", accessKey)
		}
		if secretKey != "" {
			createArgs = append(createArgs, "secret_access_key", secretKey)
		}
		if endpoint != "" {
			createArgs = append(createArgs, "endpoint", endpoint)
		}
		if region != "" {
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
		if config.GetSourcePassiveMode() || config.GetDestPassiveMode() {
			createArgs = append(createArgs, "passive_mode", "true")
		} else {
			createArgs = append(createArgs, "passive_mode", "false")
		}
		createArgs = append(createArgs, "explicit_tls", "true")
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
		createArgs = append(createArgs, "url", endpoint, "vendor", "other", "user", user)
		if pass != "" {
			createArgs = append(createArgs, "pass", pass)
		}
	case "nextcloud":
		createArgs = append(createArgs, "url", endpoint, "vendor", "nextcloud", "user", user)
		if pass != "" {
			createArgs = append(createArgs, "pass", pass)
		}
	case "gdrive":
		createArgs = append(createArgs, "scope", "drive")
		if clientID != "" {
			createArgs = append(createArgs, "client_id", clientID)
		}
		if clientSecret != "" {
			createArgs = append(createArgs, "client_secret", clientSecret)
		}
		if driveID != "" {
			createArgs = append(createArgs, "root_folder_id", driveID)
		}
		if teamDrive != "" {
			createArgs = append(createArgs, "team_drive", teamDrive)
		}
		log.Println("Warning: Google Drive test may require pre-existing token or manual auth.")
	case "gphotos":
		if clientID != "" {
			createArgs = append(createArgs, "client_id", clientID)
		}
		if clientSecret != "" {
			createArgs = append(createArgs, "client_secret", clientSecret)
		}
		log.Println("Warning: Google Photos test may require pre-existing token or manual auth.")
	case "local":
		localConfigContent := fmt.Sprintf("[%s]\ntype = local\nnounc = true\n", remoteName)
		if err := os.WriteFile(tempConfigPath, []byte(localConfigContent), 0600); err != nil {
			return false, fmt.Sprintf("Failed to write temporary local config: %v", err), err
		}
		goto RunLsd
	default:
		return false, fmt.Sprintf("Provider type '%s' not yet supported for testing via 'rclone config create'", provider), fmt.Errorf("unsupported provider")
	}

	log.Printf("Executing rclone config create command: %s %s", rclonePath, strings.Join(createArgs, " "))
	createCmd = execCommandContext(context.Background(), rclonePath, createArgs...)
	// Use the mockable function variable
	if output, err := cmdCombinedOutput(createCmd); err != nil {
		configContentBytes, _ := os.ReadFile(tempConfigPath)
		log.Printf("Temp config content on create error:\n---\n%s\n---", string(configContentBytes))
		return false, fmt.Sprintf("Failed to create temp config section: %v\nOutput: %s", err, string(output)), err
	}
	log.Printf("Successfully created temp config section for %s", remoteName)

RunLsd:

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lsdArgs = []string{
		"--config", tempConfigPath,
		"lsd",
		fmt.Sprintf("%s:%s", remoteName, remotePath),
		"--low-level-retries", "1",
		"--retries", "1",
	}

	log.Printf("Executing rclone lsd command: %s %s", rclonePath, strings.Join(lsdArgs, " "))
	lsdCmd = execCommandContext(ctx, rclonePath, lsdArgs...)

	lsdCmd.Stdout = &stdout
	lsdCmd.Stderr = &stderr

	// Use the mockable function variable
	err = cmdRun(lsdCmd)

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	log.Printf("Rclone lsd stdout:\n%s", stdoutStr)
	log.Printf("Rclone lsd stderr:\n%s", stderrStr)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return false, "Connection test timed out after 30 seconds.", context.DeadlineExceeded
		}
		// Check ctx.Err() as a fallback - This check might be redundant now
		if ctx.Err() == context.DeadlineExceeded {
			return false, "Connection test timed out after 30 seconds.", ctx.Err()
		}

		errMsg := fmt.Sprintf("Connection test failed: %v. Stderr: %s", err, stderrStr)
		if strings.Contains(stderrStr, "connect: connection refused") {
			errMsg = "Connection test failed: Connection refused by host."
		} else if strings.Contains(stderrStr, "no such host") || strings.Contains(stderrStr, "name resolution error") {
			errMsg = "Connection test failed: Hostname not found or DNS resolution error."
		} else if strings.Contains(stderrStr, "authentication failed") || strings.Contains(stderrStr, "login incorrect") || strings.Contains(stderrStr, "permission denied") {
			errMsg = "Connection test failed: Authentication failed (check credentials/permissions)."
		} else if strings.Contains(stderrStr, "directory not found") {
			errMsg = "Connection test failed: Directory/Path not found (check path)."
		} else if strings.Contains(stderrStr, "Couldn't find section") {
			errMsg = "Connection test failed: Invalid parameters provided for provider type."
		}
		return false, errMsg, err
	}

	return true, "Connection test successful!", nil
}
