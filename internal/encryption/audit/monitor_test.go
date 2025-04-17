package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuditor is a mock implementation of an auditor
type MockAuditor struct {
	mock.Mock
}

// LogEvent implements the required interface method
func (m *MockAuditor) LogEvent(event AuditEvent) {
	m.Called(event)
}

// MockAlertHandler is a mock implementation of an AlertHandler
type MockAlertHandler struct {
	mock.Mock
}

// HandleAlert implements the AlertHandler interface
func (m *MockAlertHandler) HandleAlert(alert SecurityAlert) {
	m.Called(alert)
}

func TestSecurityMonitor(t *testing.T) {
	// Create mocks
	mockAuditor := new(MockAuditor)
	mockAlertHandler := new(MockAlertHandler)

	// Create the monitor
	monitor := NewSecurityMonitor(mockAuditor)
	monitor.SetAlertHandler(mockAlertHandler)

	// Set up expectations
	testEvent := AuditEvent{
		Type:        "key_rotation",
		Description: "Key rotation completed",
		Timestamp:   time.Now(),
	}

	// The original auditor will be called
	mockAuditor.On("LogEvent", testEvent).Return()

	// Replace the auditor's LogEvent with our wrapped version
	wrappedLogEvent := monitor.AttachToAuditor()

	// Call the wrapped function
	wrappedLogEvent(testEvent)

	// Verify the expectations
	mockAuditor.AssertExpectations(t)

	// Test alert generation and handling
	mockAlertHandler.On("HandleAlert", mock.Anything).Return()

	errorEvent := AuditEvent{
		Type:        "error",
		Description: "Failed to decrypt data: invalid key",
		Timestamp:   time.Now(),
		Success:     false,
	}

	// Process the error event directly to test alert generation
	monitor.ProcessEvent(errorEvent)

	// Verify alert was handled
	mockAlertHandler.AssertExpectations(t)

	// Test reporting functionality
	report := monitor.GenerateReport()
	assert.Contains(t, report.EventCounts, "key_rotation")
	assert.Contains(t, report.ErrorCategories, "decryption_error")
}

func TestClassifyError(t *testing.T) {
	testCases := []struct {
		errorMsg      string
		expectedClass string
	}{
		{"failed to decrypt data", "decryption_error"},
		{"encryption operation failed", "encryption_error"},
		{"invalid key format", "key_error"},
		{"unauthorized access to encryption key", "permission_error"},
		{"some other random error", "other_error"},
	}

	for _, tc := range testCases {
		t.Run(tc.errorMsg, func(t *testing.T) {
			result := classifyError(tc.errorMsg)
			assert.Equal(t, tc.expectedClass, result)
		})
	}
}
