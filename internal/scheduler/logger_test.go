package scheduler

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Helper function to create a logger with a buffer for testing output
func newTestLogger(level LogLevel) (*Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	// Use a discard lumberjack logger for testing purposes
	discardLumberjack := &lumberjack.Logger{
		Filename:   filepath.Join(os.TempDir(), "test-discard.log"), // Write to temp dir
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}
	// Ensure the temp file can be cleaned up
	os.Remove(discardLumberjack.Filename)

	logger := &Logger{
		Info:     log.New(&buf, "INFO: ", 0), // No flags for simpler matching
		Error:    log.New(&buf, "ERROR: ", 0),
		Debug:    log.New(&buf, "DEBUG: ", 0),
		file:     discardLumberjack, // Use discard logger
		logLevel: level,
	}
	return logger, &buf
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelError, "error"},
		{LogLevelInfo, "info"},
		{LogLevelDebug, "debug"},
		{LogLevel(99), "unknown"}, // Test unknown level
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("LogLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		levelStr string
		want     LogLevel
	}{
		{"error", LogLevelError},
		{"ERROR", LogLevelError},
		{"info", LogLevelInfo},
		{"INFO", LogLevelInfo},
		{"debug", LogLevelDebug},
		{"DEBUG", LogLevelDebug},
		{"", LogLevelInfo},        // Default
		{"unknown", LogLevelInfo}, // Default
		{"warn", LogLevelInfo},    // Default
	}

	for _, tt := range tests {
		if got := ParseLogLevel(tt.levelStr); got != tt.want {
			t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.levelStr, got, tt.want)
		}
	}
}

func TestLoggerOutputLevels(t *testing.T) {
	tests := []struct {
		name        string
		level       LogLevel
		logFunc     func(l *Logger, format string, v ...interface{})
		wantPrefix  string
		wantMessage string
	}{
		// LogError tests
		{"ErrorLevel_LogError", LogLevelError, (*Logger).LogError, "ERROR: ", "error message 1"},
		{"InfoLevel_LogError", LogLevelInfo, (*Logger).LogError, "ERROR: ", "error message 2"},
		{"DebugLevel_LogError", LogLevelDebug, (*Logger).LogError, "ERROR: ", "error message 3"},
		// LogInfo tests
		{"ErrorLevel_LogInfo", LogLevelError, (*Logger).LogInfo, "", ""}, // Should not log
		{"InfoLevel_LogInfo", LogLevelInfo, (*Logger).LogInfo, "INFO: ", "info message 1"},
		{"DebugLevel_LogInfo", LogLevelDebug, (*Logger).LogInfo, "INFO: ", "info message 2"},
		// LogDebug tests
		{"ErrorLevel_LogDebug", LogLevelError, (*Logger).LogDebug, "", ""}, // Should not log
		{"InfoLevel_LogDebug", LogLevelInfo, (*Logger).LogDebug, "", ""},   // Should not log
		{"DebugLevel_LogDebug", LogLevelDebug, (*Logger).LogDebug, "DEBUG: ", "debug message 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newTestLogger(tt.level)
			defer logger.Close() // Close the discard logger

			message := tt.wantMessage // Use the message intended for the successful case
			if tt.wantPrefix == "" {
				message = "should not appear" // Use a different message if it shouldn't log
			}

			tt.logFunc(logger, "%s %d", message, 42) // Add formatting args

			got := buf.String()
			expectedOutput := ""
			if tt.wantPrefix != "" {
				expectedOutput = tt.wantPrefix + message + " 42\n" // Include formatting args in expected output
			}

			if got != expectedOutput {
				t.Errorf("Log output = %q, want %q", got, expectedOutput)
			}
		})
	}
}

func TestNewLoggerInitialization(t *testing.T) {
	// Temporarily set env vars for testing initialization
	os.Setenv("DATA_DIR", "/tmp/test_gomft_data")
	os.Setenv("LOGS_DIR", "/tmp/test_gomft_data/logs")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_MAX_SIZE", "5")
	os.Setenv("LOG_MAX_BACKUPS", "2")
	os.Setenv("LOG_MAX_AGE", "7")
	os.Setenv("LOG_COMPRESS", "false")

	defer func() {
		// Clean up env vars and created directories
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("LOGS_DIR")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_MAX_SIZE")
		os.Unsetenv("LOG_MAX_BACKUPS")
		os.Unsetenv("LOG_MAX_AGE")
		os.Unsetenv("LOG_COMPRESS")
		os.RemoveAll("/tmp/test_gomft_data")
	}()

	// Capture stdout to check initialization logs
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := NewLogger()
	defer logger.Close()

	w.Close()
	os.Stdout = oldStdout // Restore stdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	initOutput := buf.String()

	// Check log level
	if logger.logLevel != LogLevelDebug {
		t.Errorf("Expected log level %v, got %v", LogLevelDebug, logger.logLevel)
	}

	// Check lumberjack config
	if logger.file.MaxSize != 5 {
		t.Errorf("Expected MaxSize 5, got %d", logger.file.MaxSize)
	}
	if logger.file.MaxBackups != 2 {
		t.Errorf("Expected MaxBackups 2, got %d", logger.file.MaxBackups)
	}
	if logger.file.MaxAge != 7 {
		t.Errorf("Expected MaxAge 7, got %d", logger.file.MaxAge)
	}
	if logger.file.Compress != false {
		t.Errorf("Expected Compress false, got %v", logger.file.Compress)
	}
	expectedLogPath := filepath.Join("/tmp/test_gomft_data/logs", "scheduler.log")
	if logger.file.Filename != expectedLogPath {
		t.Errorf("Expected Filename %q, got %q", expectedLogPath, logger.file.Filename)
	}

	// Check if logs directory was created
	if _, err := os.Stat("/tmp/test_gomft_data/logs"); os.IsNotExist(err) {
		t.Errorf("Expected logs directory %q to be created", "/tmp/test_gomft_data/logs")
	}

	// Check initialization log messages
	if !strings.Contains(initOutput, "Log rotation configured:") {
		t.Errorf("Expected initialization log message 'Log rotation configured:', but not found in output:\n%s", initOutput)
	}
	if !strings.Contains(initOutput, "logLevel=debug") {
		t.Errorf("Expected 'logLevel=debug' in initialization log, but not found in output:\n%s", initOutput)
	}
	if !strings.Contains(initOutput, "Log rotation details:") {
		t.Errorf("Expected initialization log message 'Log rotation details:', but not found in output:\n%s", initOutput)
	}
}

// Note: Testing Close() and RotateLogs() directly would require more complex mocking
// of the lumberjack.Logger or filesystem interactions. For now, we focus on the
// Logger wrapper's core logic (level handling, formatting).
