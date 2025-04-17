package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// SecurityMonitor provides aggregate monitoring, alerting, and reporting for security events
type SecurityMonitor struct {
	auditor         *SecurityAuditor
	statsMutex      sync.RWMutex
	eventCounts     map[EventType]int
	errorCounts     map[string]int
	lastEventTime   map[EventType]time.Time
	alertThresholds map[EventType]int
	alertHandler    AlertHandler
}

// AlertLevel represents the severity of a security alert
type AlertLevel string

// Alert levels
const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// SecurityAlert represents a security alert to be sent to handlers
type SecurityAlert struct {
	Timestamp time.Time
	Level     AlertLevel
	EventType EventType
	Message   string
	Count     int
	Details   map[string]interface{}
}

// AlertHandler is the interface for handling security alerts
type AlertHandler interface {
	HandleAlert(alert SecurityAlert)
}

// DefaultAlertHandler is a basic implementation of AlertHandler that logs to a file
type DefaultAlertHandler struct {
	logFile    string
	writer     io.Writer
	writerLock sync.Mutex
}

// NewDefaultAlertHandler creates a new default alert handler
func NewDefaultAlertHandler(logFile string) (*DefaultAlertHandler, error) {
	var writer io.Writer

	if logFile == "" {
		writer = os.Stdout
	} else {
		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open alert log file: %w", err)
		}
		writer = file
	}

	return &DefaultAlertHandler{
		logFile: logFile,
		writer:  writer,
	}, nil
}

// HandleAlert logs the alert to the configured output
func (h *DefaultAlertHandler) HandleAlert(alert SecurityAlert) {
	h.writerLock.Lock()
	defer h.writerLock.Unlock()

	jsonData, err := json.Marshal(alert)
	if err != nil {
		fmt.Fprintf(h.writer, "Error marshaling alert: %v\n", err)
		return
	}

	fmt.Fprintln(h.writer, string(jsonData))
}

// Close closes any open resources
func (h *DefaultAlertHandler) Close() error {
	if h.logFile != "" {
		if closer, ok := h.writer.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}

// NewSecurityMonitor creates a new SecurityMonitor
func NewSecurityMonitor(auditor *SecurityAuditor) *SecurityMonitor {
	// Use provided auditor or global one if nil
	if auditor == nil {
		auditor = GetGlobalAuditor()
	}

	defaultHandler, _ := NewDefaultAlertHandler("")

	return &SecurityMonitor{
		auditor:         auditor,
		eventCounts:     make(map[EventType]int),
		errorCounts:     make(map[string]int),
		lastEventTime:   make(map[EventType]time.Time),
		alertThresholds: make(map[EventType]int),
		alertHandler:    defaultHandler,
	}
}

// SetAlertHandler sets a custom alert handler
func (m *SecurityMonitor) SetAlertHandler(handler AlertHandler) {
	m.alertHandler = handler
}

// SetAlertThreshold sets the threshold for when to generate alerts for a specific event type
func (m *SecurityMonitor) SetAlertThreshold(eventType EventType, threshold int) {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()

	m.alertThresholds[eventType] = threshold
}

// ProcessEvent processes a security event for monitoring
func (m *SecurityMonitor) ProcessEvent(event AuditEvent) {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()

	// Update event statistics
	m.eventCounts[event.EventType]++
	m.lastEventTime[event.EventType] = event.Timestamp

	// Track errors
	if !event.Success && event.Error != "" {
		errorType := classifyError(event.Error)
		m.errorCounts[errorType]++

		// Alert on specific error types
		if strings.Contains(event.Error, "unauthorized") ||
			strings.Contains(event.Error, "permission") ||
			strings.Contains(event.Error, "access denied") {
			m.generateAlert(AlertLevelCritical, event.EventType,
				fmt.Sprintf("Possible security breach detected: %s", event.Error),
				map[string]interface{}{
					"operation":  event.Operation,
					"error":      event.Error,
					"keyVersion": event.KeyVersion,
					"modelType":  event.ModelType,
				})
		}
	}

	// Check thresholds for alerting
	threshold, hasThreshold := m.alertThresholds[event.EventType]
	if hasThreshold && m.eventCounts[event.EventType] >= threshold {
		if event.EventType == EventDecryptionFailure || event.EventType == EventEncryptionFailure {
			m.generateAlert(AlertLevelWarning, event.EventType,
				fmt.Sprintf("High number of %s events detected (%d)", event.EventType, m.eventCounts[event.EventType]),
				map[string]interface{}{
					"count":     m.eventCounts[event.EventType],
					"threshold": threshold,
				})
		} else if event.EventType == EventKeyRotation {
			m.generateAlert(AlertLevelInfo, event.EventType,
				fmt.Sprintf("Key rotation threshold reached (%d operations)", m.eventCounts[event.EventType]),
				map[string]interface{}{
					"count":     m.eventCounts[event.EventType],
					"threshold": threshold,
				})
		}

		// Reset counter after alerting
		m.eventCounts[event.EventType] = 0
	}
}

// GenerateReport generates a report of security events for a time period
func (m *SecurityMonitor) GenerateReport(startTime, endTime time.Time, writer io.Writer) error {
	m.statsMutex.RLock()
	defer m.statsMutex.RUnlock()

	report := struct {
		TimeRange struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		} `json:"time_range"`
		EventCounts    map[EventType]int       `json:"event_counts"`
		ErrorCounts    map[string]int          `json:"error_counts"`
		LastEventTimes map[EventType]time.Time `json:"last_event_times"`
		GeneratedAt    time.Time               `json:"generated_at"`
	}{
		TimeRange: struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		}{
			Start: startTime,
			End:   endTime,
		},
		EventCounts:    m.eventCounts,
		ErrorCounts:    m.errorCounts,
		LastEventTimes: m.lastEventTime,
		GeneratedAt:    time.Now(),
	}

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	_, err = writer.Write(jsonData)
	return err
}

// generateAlert creates and sends a security alert
func (m *SecurityMonitor) generateAlert(level AlertLevel, eventType EventType, message string, details map[string]interface{}) {
	if m.alertHandler == nil {
		return
	}

	alert := SecurityAlert{
		Timestamp: time.Now(),
		Level:     level,
		EventType: eventType,
		Message:   message,
		Count:     m.eventCounts[eventType],
		Details:   details,
	}

	go m.alertHandler.HandleAlert(alert)
}

// classifyError examines an error string and categorizes it
func classifyError(errorStr string) string {
	errorStr = strings.ToLower(errorStr)

	if strings.Contains(errorStr, "decrypt") {
		return "decryption_error"
	} else if strings.Contains(errorStr, "encrypt") {
		return "encryption_error"
	} else if strings.Contains(errorStr, "key") {
		return "key_error"
	} else if strings.Contains(errorStr, "permission") || strings.Contains(errorStr, "unauthorized") {
		return "permission_error"
	} else {
		return "other_error"
	}
}

// AttachToAuditor creates a wrapper function for the auditor's LogEvent method
// that processes events through the monitor before passing them to the original function.
// Returns the wrapped function that should be set on the auditor.
func (m *SecurityMonitor) AttachToAuditor() func(AuditEvent) {
	originalLogEvent := m.auditor.LogEvent

	// Create a wrapper function that processes events and then calls the original
	return func(event AuditEvent) {
		// Process the event for monitoring
		m.ProcessEvent(event)

		// Call the original LogEvent function
		originalLogEvent(event)
	}
}
