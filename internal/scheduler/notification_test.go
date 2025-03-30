package scheduler

import (
	"crypto/hmac"   // Added import
	"crypto/sha256" // Added import
	"encoding/hex"  // Added import
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"gorm.io/gorm"
)

// --- Mock DB Implementation ---

// Ensure mockNotificationDB implements the NotificationDB interface
var _ NotificationDB = (*mockNotificationDB)(nil)

type mockNotificationDB struct {
	GetNotificationServicesFunc   func(enabledOnly bool) ([]db.NotificationService, error)
	UpdateNotificationServiceFunc func(service *db.NotificationService) error
	GetJobFunc                    func(jobID uint) (*db.Job, error)
	CreateJobNotificationFunc     func(userID uint, jobID uint, historyID uint, notificationType db.NotificationType, title string, message string) error
	CreateFunc                    func(value interface{}) *gorm.DB

	// Mutex to protect concurrent access to mock data if needed
	mu sync.Mutex
	// Store data for verification if needed
	updatedServices      []*db.NotificationService
	createdNotifications []map[string]interface{}
	createdHistory       *db.JobHistory
}

func (m *mockNotificationDB) GetNotificationServices(enabledOnly bool) ([]db.NotificationService, error) {
	if m.GetNotificationServicesFunc != nil {
		return m.GetNotificationServicesFunc(enabledOnly)
	}
	return nil, errors.New("mock GetNotificationServicesFunc not implemented")
}

func (m *mockNotificationDB) UpdateNotificationService(service *db.NotificationService) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedServices = append(m.updatedServices, service) // Store for verification
	if m.UpdateNotificationServiceFunc != nil {
		return m.UpdateNotificationServiceFunc(service)
	}
	return nil // Default success
}

func (m *mockNotificationDB) GetJob(jobID uint) (*db.Job, error) {
	if m.GetJobFunc != nil {
		return m.GetJobFunc(jobID)
	}
	// Default mock behavior: return a basic job
	return &db.Job{ID: jobID, Name: "Mock Job", CreatedBy: 1}, nil
}

func (m *mockNotificationDB) CreateJobNotification(userID uint, jobID uint, historyID uint, notificationType db.NotificationType, title string, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdNotifications = append(m.createdNotifications, map[string]interface{}{
		"userID": userID, "jobID": jobID, "historyID": historyID, "type": notificationType, "title": title, "message": message,
	})
	if m.CreateJobNotificationFunc != nil {
		return m.CreateJobNotificationFunc(userID, jobID, historyID, notificationType, title, message)
	}
	return nil // Default success
}

func (m *mockNotificationDB) Create(value interface{}) *gorm.DB {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hist, ok := value.(*db.JobHistory); ok {
		m.createdHistory = hist // Store for verification if needed
	}
	if m.CreateFunc != nil {
		return m.CreateFunc(value)
	}
	// Default mock behavior: return success with no error
	return &gorm.DB{Error: nil}
}

// Helper to reset mock state between tests
func (m *mockNotificationDB) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedServices = nil
	m.createdNotifications = nil
	m.createdHistory = nil
}

// --- Test Helpers ---

func createTestJob(id uint, webhookEnabled bool, webhookURL string, notifySuccess bool, notifyFailure bool) *db.Job {
	job := db.Job{
		// Removed gorm.Model nesting, set ID directly
		ID:              id,
		Name:            "Test Job",
		WebhookEnabled:  &webhookEnabled,
		WebhookURL:      webhookURL,
		NotifyOnSuccess: &notifySuccess,
		NotifyOnFailure: &notifyFailure,
		CreatedBy:       1, // Assume user ID 1
	}
	return &job
}

func createTestHistory(id uint, jobID uint, status string, errMsg string) *db.JobHistory {
	now := time.Now()
	hist := db.JobHistory{
		ID:           id,
		JobID:        jobID,
		Status:       status,
		StartTime:    now.Add(-1 * time.Minute),
		ErrorMessage: errMsg,
	}
	if status != "running" {
		endTime := now
		hist.EndTime = &endTime
	}
	return &hist
}

func createTestConfig(id uint) *db.TransferConfig {
	return &db.TransferConfig{
		// Removed gorm.Model nesting, set ID directly
		ID:              id,
		Name:            "Test Config",
		SourceType:      "local",
		SourcePath:      "/tmp/source",
		DestinationType: "local",
		DestinationPath: "/tmp/dest",
	}
}

func createTestNotificationService(id uint, name, svcType string, enabled bool, triggers []string, config map[string]string) db.NotificationService {
	return db.NotificationService{
		ID:            id,
		Name:          name,
		Type:          svcType,
		IsEnabled:     enabled, // Corrected field name
		EventTriggers: triggers,
		Config:        config,
		// Initialize other fields as needed for tests, e.g., RetryPolicy
		RetryPolicy: "none",
	}
}

// --- Tests ---

func TestSendJobWebhookNotification(t *testing.T) {
	logger, logBuf := newTestLogger(LogLevelDebug)
	defer logger.Close()
	mockDB := &mockNotificationDB{} // Not used directly by this function, but Notifier needs it
	notifier := NewNotifier(mockDB, logger)

	var receivedPayload map[string]interface{}
	var receivedHeaders http.Header
	var receivedSignature string

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedSignature = r.Header.Get("X-Hub-Signature-256")
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &receivedPayload)
		w.WriteHeader(http.StatusOK) // Respond with success
	}))
	defer server.Close()

	job := createTestJob(1, true, server.URL, true, true)
	job.WebhookSecret = "test-secret"                         // Add secret for signature testing
	job.WebhookHeaders = `{"X-Custom-Header": "CustomValue"}` // Add custom headers
	history := createTestHistory(10, 1, "completed", "")
	config := createTestConfig(5)

	notifier.sendJobWebhookNotification(job, history, config)

	// Assertions
	if receivedPayload == nil {
		t.Fatal("Webhook server did not receive a payload")
	}
	if receivedPayload["job_id"].(float64) != float64(job.ID) {
		t.Errorf("Expected job_id %d, got %v", job.ID, receivedPayload["job_id"])
	}
	if receivedPayload["status"] != history.Status {
		t.Errorf("Expected status %q, got %q", history.Status, receivedPayload["status"])
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", receivedHeaders.Get("Content-Type"))
	}
	if receivedHeaders.Get("User-Agent") != "GoMFT-Webhook/1.0" {
		t.Errorf("Expected User-Agent 'GoMFT-Webhook/1.0', got %q", receivedHeaders.Get("User-Agent"))
	}
	if receivedHeaders.Get("X-Custom-Header") != "CustomValue" {
		t.Errorf("Expected X-Custom-Header 'CustomValue', got %q", receivedHeaders.Get("X-Custom-Header"))
	}

	// Verify signature
	// Re-marshal the *received* payload to ensure byte-for-byte match for signature calculation
	payloadBytes, err := json.Marshal(receivedPayload)
	if err != nil {
		t.Fatalf("Failed to re-marshal received payload for signature check: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(job.WebhookSecret))
	mac.Write(payloadBytes)
	expectedSignature := hex.EncodeToString(mac.Sum(nil)) // Use imported hex

	if receivedSignature == "" {
		t.Error("Expected X-Hub-Signature-256 header, but it was missing")
	} else if receivedSignature != expectedSignature {
		t.Errorf("Signature mismatch: got %q, want %q. Payload received: %s", receivedSignature, expectedSignature, string(payloadBytes))
	}

	// Check logs
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Webhook notification for job 1 sent successfully") {
		t.Errorf("Expected success log message, but got:\n%s", logOutput)
	}
}

func TestSendGlobalNotifications_Webhook(t *testing.T) {
	logger, _ := newTestLogger(LogLevelDebug)
	defer logger.Close()

	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mockDB := &mockNotificationDB{}
	mockDB.GetNotificationServicesFunc = func(enabledOnly bool) ([]db.NotificationService, error) {
		allServices := []db.NotificationService{
			createTestNotificationService(1, "Test Webhook", "webhook", true, []string{"job_complete"}, map[string]string{"webhook_url": server.URL}),
			createTestNotificationService(2, "Disabled Webhook", "webhook", false, []string{"job_complete"}, map[string]string{"webhook_url": "http://disabled.invalid"}),
			createTestNotificationService(3, "Wrong Trigger", "webhook", true, []string{"job_error"}, map[string]string{"webhook_url": "http://wrongtrigger.invalid"}),
		}
		if enabledOnly {
			var enabledServices []db.NotificationService
			for _, s := range allServices {
				if s.IsEnabled { // Use IsEnabled field
					enabledServices = append(enabledServices, s)
				}
			}
			t.Logf("Mock GetNotificationServices(true) returning %d services", len(enabledServices)) // Add log
			return enabledServices, nil
		}
		t.Logf("Mock GetNotificationServices(false) returning %d services", len(allServices)) // Add log
		return allServices, nil
	}
	mockDB.UpdateNotificationServiceFunc = func(service *db.NotificationService) error {
		// Add debug logging
		t.Logf("UpdateNotificationServiceFunc called with service ID: %d, Name: %s, SuccessCount: %d", service.ID, service.Name, service.SuccessCount)
		if service.ID != 1 {
			t.Errorf("Expected UpdateNotificationService for ID 1, got %d", service.ID)
		}
		if service.SuccessCount != 1 {
			t.Errorf("Expected SuccessCount 1, got %d", service.SuccessCount)
		}
		return nil
	}

	notifier := NewNotifier(mockDB, logger)

	job := createTestJob(10, false, "", false, false) // Job-specific webhook disabled
	history := createTestHistory(100, 10, "completed", "")
	config := createTestConfig(50)

	notifier.sendGlobalNotifications(job, history, config)

	// Assertions
	if receivedPayload == nil {
		t.Fatal("Webhook server did not receive a payload from global notification")
	}
	if jobData, ok := receivedPayload["job"].(map[string]interface{}); ok {
		if jobData["id"].(float64) != float64(job.ID) {
			t.Errorf("Expected job.id %d, got %v", job.ID, jobData["id"])
		}
		if jobData["status"] != history.Status {
			t.Errorf("Expected job.status %q, got %q", history.Status, jobData["status"])
		}
	} else {
		t.Fatal("Payload missing 'job' field or not a map")
	}

	// Verify DB update was called correctly
	mockDB.mu.Lock()
	if len(mockDB.updatedServices) != 1 || mockDB.updatedServices[0].ID != 1 {
		t.Errorf("Expected 1 call to UpdateNotificationService for service ID 1, got %d calls", len(mockDB.updatedServices))
	}
	mockDB.mu.Unlock()
}

// TODO: Add tests for other notification service types (email, pushbullet, ntfy, gotify, pushover)
// TODO: Add tests for SendNotifications (combining job-specific and global)
// TODO: Add tests for template variable replacement (replaceVariables, generateCustomPayload)
// TODO: Add tests for createJobNotification and updateJobStatus (if kept)
