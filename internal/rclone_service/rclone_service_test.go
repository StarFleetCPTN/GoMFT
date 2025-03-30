package rclone_service

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/starfleetcptn/gomft/internal/db"
)

// --- Mock os/exec ---

// Note: The package-level variable 'execCommandContext' is defined in rclone_service.go
// This helper function replaces it for the duration of a test.

// MockExecCommand replaces the package-level execCommandContext variable (defined in rclone_service.go)
// with a function provided by the test and returns a function to restore the original.
func MockExecCommand(mockFunc func(ctx context.Context, command string, args ...string) *exec.Cmd) (restore func()) {
	original := execCommandContext
	execCommandContext = mockFunc
	return func() { execCommandContext = original }
}

// Helper function to find the actual rclone command within args, skipping flags.
func findRcloneCommand(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
			}
			continue
		}
		return arg
	}
	return ""
}

// --- Tests ---

func TestTestRcloneConnection_Success_SFTP(t *testing.T) {
	config := db.TransferConfig{
		SourceType:     "sftp",
		SourceHost:     "testhost",
		SourceUser:     "testuser",
		SourcePassword: "testpassword",
	}
	providerType := "source"
	var dbInstance *db.DB
	configCreateCalled := false

	// Mock execCommandContext (only needed to return a basic cmd struct)
	restoreExec := MockExecCommand(func(ctx context.Context, command string, args ...string) *exec.Cmd {
		// Return a simple, non-nil command object. The actual execution is mocked below.
		return exec.Command("echo", "mocked")
	})
	defer restoreExec()

	// Mock cmdCombinedOutput for config create
	originalCombinedOutput := cmdCombinedOutput
	cmdCombinedOutput = func(c *exec.Cmd) ([]byte, error) {
		configCreateCalled = true
		return []byte(""), nil // Simulate success
	}
	defer func() { cmdCombinedOutput = originalCombinedOutput }() // Restore original

	// Mock cmdRun for lsd
	originalRun := cmdRun
	cmdRun = func(c *exec.Cmd) error {
		if !configCreateCalled {
			t.Fatalf("lsd (Run) called before config create")
		}
		// Simulate success by returning nil error
		// We also need to simulate writing to stdout if the main func uses it
		if stdoutWriter, ok := c.Stdout.(interface{ WriteString(string) (int, error) }); ok {
			stdoutWriter.WriteString("          -1 2023-01-01 10:00:00        -1 some_dir\n")
		}
		return nil
	}
	defer func() { cmdRun = originalRun }() // Restore original

	success, msg, err := TestRcloneConnection(config, providerType, dbInstance)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if !success {
		t.Errorf("Expected success=true, but got false. Message: %s", msg)
	}
	if msg != "Connection test successful!" {
		t.Errorf("Expected success message, but got: %q", msg)
	}
}

func TestTestRcloneConnection_ConfigCreateFail(t *testing.T) {
	config := db.TransferConfig{
		SourceType: "sftp",
		SourceHost: "testhost",
		SourceUser: "testuser",
	}
	providerType := "source"
	var dbInstance *db.DB
	expectedStderr := "invalid parameters"
	expectedErr := errors.New("exit status 1")

	// Mock execCommandContext
	restoreExec := MockExecCommand(func(ctx context.Context, command string, args ...string) *exec.Cmd {
		return exec.Command("echo", "mocked")
	})
	defer restoreExec()

	// Mock cmdCombinedOutput for config create failure
	originalCombinedOutput := cmdCombinedOutput
	cmdCombinedOutput = func(c *exec.Cmd) ([]byte, error) {
		return []byte(expectedStderr), expectedErr // Simulate failure
	}
	defer func() { cmdCombinedOutput = originalCombinedOutput }()

	// Mock cmdRun (should not be called)
	originalRun := cmdRun
	cmdRun = func(c *exec.Cmd) error {
		t.Fatalf("lsd (Run) called after config create failure")
		return errors.New("should not be called")
	}
	defer func() { cmdRun = originalRun }()

	success, msg, err := TestRcloneConnection(config, providerType, dbInstance)

	if err == nil {
		t.Error("Expected an error from config create failure, but got nil")
	} else if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if success {
		t.Error("Expected success=false for config create failure, but got true")
	}
	if !strings.Contains(msg, "Failed to create temp config section") {
		t.Errorf("Expected message containing 'Failed to create temp config section', got: %q", msg)
	}
	// Note: The CombinedOutput mock returns stderr in the output byte slice
	if !strings.Contains(msg, expectedStderr) {
		t.Errorf("Expected message containing stderr %q, got: %q", expectedStderr, msg)
	}
}

func TestTestRcloneConnection_LsdTimeout(t *testing.T) {
	config := db.TransferConfig{
		SourceType:     "sftp",
		SourceHost:     "testhost",
		SourceUser:     "testuser",
		SourcePassword: "pw",
	}
	providerType := "source"
	var dbInstance *db.DB

	// Mock execCommandContext
	restoreExec := MockExecCommand(func(ctx context.Context, command string, args ...string) *exec.Cmd {
		return exec.Command("echo", "mocked")
	})
	defer restoreExec()

	// Mock cmdCombinedOutput for config create success
	originalCombinedOutput := cmdCombinedOutput
	cmdCombinedOutput = func(c *exec.Cmd) ([]byte, error) {
		return []byte(""), nil
	}
	defer func() { cmdCombinedOutput = originalCombinedOutput }()

	// Mock cmdRun for lsd timeout
	originalRun := cmdRun
	cmdRun = func(c *exec.Cmd) error {
		// Simulate timeout error
		return context.DeadlineExceeded
	}
	defer func() { cmdRun = originalRun }()

	success, msg, err := TestRcloneConnection(config, providerType, dbInstance)

	if err == nil {
		t.Error("Expected a timeout error, but got nil")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got: %v (type: %T)", err, err)
	}
	if success {
		t.Error("Expected success=false for timeout, but got true")
	}
	if !strings.Contains(msg, "Connection test timed out") {
		t.Errorf("Expected message containing 'Connection test timed out', got: %q", msg)
	}
}

func TestTestRcloneConnection_LsdAuthFail(t *testing.T) {
	config := db.TransferConfig{
		SourceType:     "sftp",
		SourceHost:     "testhost",
		SourceUser:     "wronguser",
		SourcePassword: "wrongpassword",
	}
	providerType := "source"
	var dbInstance *db.DB
	expectedStderr := "authentication failed"
	expectedErr := errors.New("exit status 1")

	// Mock execCommandContext
	restoreExec := MockExecCommand(func(ctx context.Context, command string, args ...string) *exec.Cmd {
		return exec.Command("echo", "mocked")
	})
	defer restoreExec()

	// Mock cmdCombinedOutput for config create success
	originalCombinedOutput := cmdCombinedOutput
	cmdCombinedOutput = func(c *exec.Cmd) ([]byte, error) {
		return []byte(""), nil
	}
	defer func() { cmdCombinedOutput = originalCombinedOutput }()

	// Mock cmdRun for lsd failure
	originalRun := cmdRun
	cmdRun = func(c *exec.Cmd) error {
		// Simulate failure by returning error and writing to stderr buffer
		if stderrWriter, ok := c.Stderr.(interface{ WriteString(string) (int, error) }); ok {
			stderrWriter.WriteString(expectedStderr)
		}
		return expectedErr
	}
	defer func() { cmdRun = originalRun }()

	success, msg, err := TestRcloneConnection(config, providerType, dbInstance)

	if err == nil {
		t.Error("Expected an error from lsd auth failure, but got nil")
	} else if !errors.Is(err, expectedErr) {
		if !strings.Contains(err.Error(), "exit status 1") {
			t.Errorf("Expected error containing 'exit status 1', got: %v", err)
		}
	}
	if success {
		t.Error("Expected success=false for lsd auth failure, but got true")
	}
	// Check the parsed error message based on stderr
	if !strings.Contains(msg, "Authentication failed") {
		t.Errorf("Expected message containing 'Authentication failed', got: %q", msg)
	}
}

func TestTestRcloneConnection_LocalSuccess(t *testing.T) {
	tempPath := t.TempDir()
	config := db.TransferConfig{
		SourceType: "local",
		SourcePath: tempPath,
	}
	providerType := "source"
	var dbInstance *db.DB

	// Mock execCommandContext (only lsd should be called)
	restoreExec := MockExecCommand(func(ctx context.Context, command string, args ...string) *exec.Cmd {
		rcloneCmd := findRcloneCommand(args)
		if rcloneCmd != "lsd" {
			t.Fatalf("Unexpected command call for local provider: %q", rcloneCmd)
		}
		return exec.Command("echo", "mocked for lsd")
	})
	defer restoreExec()

	// Mock cmdCombinedOutput (should not be called)
	originalCombinedOutput := cmdCombinedOutput
	cmdCombinedOutput = func(c *exec.Cmd) ([]byte, error) {
		t.Fatalf("CombinedOutput called unexpectedly for local provider")
		return nil, errors.New("should not be called")
	}
	defer func() { cmdCombinedOutput = originalCombinedOutput }()

	// Mock cmdRun for lsd success
	originalRun := cmdRun
	cmdRun = func(c *exec.Cmd) error {
		// Simulate success
		if stdoutWriter, ok := c.Stdout.(interface{ WriteString(string) (int, error) }); ok {
			stdoutWriter.WriteString("          -1 2023-01-01 10:00:00        -1 some_local_dir\n")
		}
		return nil
	}
	defer func() { cmdRun = originalRun }()

	success, msg, err := TestRcloneConnection(config, providerType, dbInstance)

	if err != nil {
		t.Errorf("Expected no error for local success, but got: %v", err)
	}
	if !success {
		t.Errorf("Expected success=true for local success, but got false. Message: %s", msg)
	}
	if msg != "Connection test successful!" {
		t.Errorf("Expected success message, but got: %q", msg)
	}
}

// TODO: Add more tests for other providers (S3, FTP, WebDAV, etc.)
// TODO: Add tests for destination providerType
// TODO: Add tests for specific error string parsing (connection refused, dir not found)
