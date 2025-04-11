package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/starfleetcptn/gomft/internal/web"
)

var (
	// Global logger instance
	stdLogger *Logger

	// Mutex to protect the logger
	loggerMutex sync.RWMutex

	// Flag to prevent recursive logging
	isLogging sync.Mutex

	// Log levels
	LevelDebug   = "debug"
	LevelInfo    = "info"
	LevelWarning = "warn"
	LevelError   = "error"
	LevelFatal   = "fatal"

	// Regex to parse standard log lines (YYYY/MM/DD HH:MM:SS file:line msg)
	// Adjust if Lmicroseconds is used
	logLineRegex *regexp.Regexp
)

func init() {
	// Check log flags to build the correct regex
	flags := log.Flags()
	timestampFormat := `\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`
	if flags&log.Lmicroseconds != 0 {
		timestampFormat += `\.\d{6}`
	}
	fileFormat := ``
	if flags&log.Lshortfile != 0 || flags&log.Llongfile != 0 {
		fileFormat = ` (.+?:\d+): ` // Group 1: file:line
	}
	// Regex captures: 1=file:line (optional), 2=message
	logLineRegex = regexp.MustCompile(fmt.Sprintf(`^%s%s(.*)$`, timestampFormat, fileFormat))
}

// Logger is a custom logger that broadcasts to WebSocket and writes to file
type Logger struct {
	fileWriter io.Writer
	broadcast  bool
}

// Setup initializes the global logger
func Setup(logsDir string, broadcast bool) error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create or open the log file
	logFilePath := filepath.Join(logsDir, "scheduler.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create a multi-writer to log to both stderr and file
	multiWriter := io.MultiWriter(os.Stderr, logFile)

	// Initialize the logger with mutex protection
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	stdLogger = &Logger{
		fileWriter: multiWriter,
		broadcast:  broadcast,
	}

	// Configure the standard log package to use our custom logger
	log.SetOutput(stdLogger)
	// Ensure standard flags are set (adjust regex if flags change)
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	log.Printf("Logger initialized: broadcasting to WebSocket = %v, file = %s", broadcast, logFilePath)
	return nil
}

// Write implements io.Writer interface for capturing standard log output
func (l *Logger) Write(p []byte) (n int, err error) {
	// Write to the original outputs first
	n, err = l.fileWriter.Write(p)
	if err != nil {
		return n, err // Return error from underlying writer
	}

	if !l.broadcast {
		return n, nil // Broadcasting disabled
	}

	// Use a mutex to prevent recursive logging from BroadcastLog itself
	if !isLogging.TryLock() {
		return n, nil // Already processing a log, skip to avoid recursion
	}
	defer isLogging.Unlock()

	// Parse the full log line
	logLine := string(p)
	level, source, message := parseLogEntry(logLine)

	// *** DEBUG: Print parsed result to stderr ***
	fmt.Fprintf(os.Stderr, "[DEBUG-LOGGER-V2] Parsed: Level='%s', Source='%s', Message='%s'\n", level, source, strings.TrimSpace(message))

	// --- TEMPORARILY DISABLED FILTER ---
	/*
		// Don't broadcast logs about WebSocket activity to avoid potential loops
		if source == "handler" || source == "admin_handlers" || strings.Contains(message, "WebSocket") || strings.Contains(message, "Broadcasting log") || source == "routes" {
			fmt.Fprintf(os.Stderr, "[DEBUG-LOGGER-V2] Filtered out log from source '%s'\n", source)
			return n, nil
		}
	*/
	// --- END TEMPORARILY DISABLED FILTER ---

	// Broadcast to WebSocket clients if handlers are initialized
	if handlers, ok := web.GetHandlersInstance(); ok && handlers != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG-LOGGER-V2] Broadcasting: Level='%s', Source='%s'\n", level, source)
		handlers.BroadcastLog(level, message, source) // Pass parsed values
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG-LOGGER-V2] Skipped broadcast: handlers not ready\n")
	}

	return n, nil // Return the number of bytes written and no error
}

// parseLogEntry extracts level, source, and message from a standard Go log line
func parseLogEntry(logLine string) (level, source, message string) {
	// Default values
	level = LevelInfo
	source = "system"
	message = strings.TrimSpace(logLine) // Use full line as message by default

	matches := logLineRegex.FindStringSubmatch(logLine)
	flags := log.Flags()
	hasFileInfo := flags&log.Lshortfile != 0 || flags&log.Llongfile != 0

	msgIndex := 1 // Index of the message part in regex matches
	if hasFileInfo {
		msgIndex = 2
	}

	if len(matches) > msgIndex {
		rawMessage := strings.TrimSpace(matches[msgIndex])
		message = rawMessage // Assign raw message first

		// Extract source from file info if present
		if hasFileInfo && len(matches) > 1 && matches[1] != "" {
			fileInfo := matches[1]
			parts := strings.Split(fileInfo, ":")
			if len(parts) > 0 {
				fileName := filepath.Base(parts[0])
				source = strings.TrimSuffix(fileName, ".go")
			}
		} else {
			// Attempt to infer source if no file info
			if strings.Contains(rawMessage, "scheduler") {
				source = "scheduler"
			} // Add other inferences if needed
		}

		// Now, parse the level based on prefixes *within* the rawMessage
		parsedLevel, cleanMessage := parseLevelFromMessage(rawMessage)
		level = parsedLevel    // Update level if prefix found
		message = cleanMessage // Update message to remove prefix

	} else {
		// Regex didn't match, try basic prefix check on the whole line (fallback)
		level, message = parseLevelFromMessage(message) // Use original full message
	}

	return level, source, message
}

// parseLevelFromMessage checks for level prefixes within a message string
func parseLevelFromMessage(msg string) (level string, cleanMsg string) {
	level = LevelInfo // Default
	cleanMsg = msg

	// Check common prefixes
	if strings.HasPrefix(msg, "DEBUG:") {
		level = LevelDebug
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "DEBUG:"))
	} else if strings.HasPrefix(msg, "INFO:") {
		level = LevelInfo
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "INFO:"))
	} else if strings.HasPrefix(msg, "ERROR:") {
		level = LevelError
		cleanMsg = strings.TrimPrefix(msg, "ERROR:")
	} else if strings.HasPrefix(msg, "WARN:") {
		level = LevelWarning
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "WARN:"))
	} else if strings.HasPrefix(msg, "WARNING:") {
		level = LevelWarning
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "WARNING:"))
	} else if strings.HasPrefix(msg, "FATAL:") {
		level = LevelFatal
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "FATAL:"))
	} else if strings.HasPrefix(msg, "[debug]") {
		level = LevelDebug
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[debug]"))
	} else if strings.HasPrefix(msg, "[info]") {
		level = LevelInfo
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[info]"))
	} else if strings.HasPrefix(msg, "[warn]") {
		level = LevelWarning
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[warn]"))
	} else if strings.HasPrefix(msg, "[warning]") {
		level = LevelWarning
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[warning]"))
	} else if strings.HasPrefix(msg, "[error]") {
		level = LevelError
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[error]"))
	} else if strings.HasPrefix(msg, "[fatal]") {
		level = LevelFatal
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(msg, "[fatal]"))
	}

	return level, cleanMsg
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	return stdLogger
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	if stdLogger == nil {
		// Fall back to standard logger if not initialized
		log.Printf("[debug] "+format, v...)
		return
	}

	log.Printf("[debug] "+format, v...)
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	if stdLogger == nil {
		// Fall back to standard logger if not initialized
		log.Printf("[info] "+format, v...)
		return
	}

	log.Printf("[info] "+format, v...)
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	if stdLogger == nil {
		// Fall back to standard logger if not initialized
		log.Printf("[warn] "+format, v...)
		return
	}

	log.Printf("[warn] "+format, v...)
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	if stdLogger == nil {
		// Fall back to standard logger if not initialized
		log.Printf("[error] "+format, v...)
		return
	}

	log.Printf("[error] "+format, v...)
}

// Fatal logs a fatal message and exits
func Fatal(format string, v ...interface{}) {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	if stdLogger == nil {
		// Fall back to standard logger if not initialized
		log.Fatalf("[fatal] "+format, v...)
		return
	}

	log.Fatalf("[fatal] "+format, v...)
}
