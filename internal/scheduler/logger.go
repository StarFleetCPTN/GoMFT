package scheduler

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
