package scheduler

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings" // Added import
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/starfleetcptn/gomft/internal/db"
	"gorm.io/gorm"
)

// --- Mock Implementations ---

// Mock JobExecutorDB
var _ JobExecutorDB = (*mockJobExecutorDB)(nil)

type mockJobExecutorDB struct {
	mu                   sync.Mutex
	FirstFunc            func(dest interface{}, conds ...interface{}) *gorm.DB
	GetConfigsForJobFunc func(jobID uint) ([]db.TransferConfig, error)
	UpdateJobStatusFunc  func(job *db.Job) error
	CreateJobHistoryFunc func(history *db.JobHistory) error

	// Store calls/data
	firstCalledWithDest  interface{}
	firstCalledWithConds []interface{}
	configsForJobID      uint
	updatedJobStatus     *db.Job
	createdHistory       *db.JobHistory
}

func (m *mockJobExecutorDB) First(dest interface{}, conds ...interface{}) *gorm.DB {
	m.mu.Lock()
	m.firstCalledWithDest = dest
	m.firstCalledWithConds = conds
	m.mu.Unlock()
	if m.FirstFunc != nil {
		return m.FirstFunc(dest, conds...)
	}
	// Default: Simulate job found by populating dest
	if job, ok := dest.(*db.Job); ok && len(conds) > 0 {
		if jobID, ok := conds[0].(uint); ok {
			job.ID = jobID
			job.Name = fmt.Sprintf("Mock Job %d", jobID)
			job.ConfigIDs = "1,2" // Default config IDs
			enabled := true
			job.Enabled = &enabled
			return &gorm.DB{Error: nil} // Success
		}
	}
	return &gorm.DB{Error: gorm.ErrRecordNotFound} // Default not found
}

func (m *mockJobExecutorDB) GetConfigsForJob(jobID uint) ([]db.TransferConfig, error) {
	m.mu.Lock()
	m.configsForJobID = jobID
	m.mu.Unlock()
	if m.GetConfigsForJobFunc != nil {
		return m.GetConfigsForJobFunc(jobID)
	}
	// Default: return some mock configs
	return []db.TransferConfig{
		{ID: 1, Name: "Config 1"}, // Corrected initialization
		{ID: 2, Name: "Config 2"}, // Corrected initialization
	}, nil
}

func (m *mockJobExecutorDB) UpdateJobStatus(job *db.Job) error {
	m.mu.Lock()
	m.updatedJobStatus = job // Store last updated job
	m.mu.Unlock()
	if m.UpdateJobStatusFunc != nil {
		return m.UpdateJobStatusFunc(job)
	}
	return nil // Default success
}

func (m *mockJobExecutorDB) CreateJobHistory(history *db.JobHistory) error {
	m.mu.Lock()
	m.createdHistory = history // Store last created history
	m.mu.Unlock()
	if m.CreateJobHistoryFunc != nil {
		return m.CreateJobHistoryFunc(history)
	}
	history.ID = 999 // Assign mock ID
	return nil       // Default success
}

func (m *mockJobExecutorDB) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.firstCalledWithDest = nil
	m.firstCalledWithConds = nil
	m.configsForJobID = 0
	m.updatedJobStatus = nil
	m.createdHistory = nil
}

// Mock JobExecutorCron
var _ JobExecutorCron = (*mockJobExecutorCron)(nil)

type mockJobExecutorCron struct {
	mu        sync.Mutex
	EntryFunc func(id cron.EntryID) cron.Entry

	// Store calls
	entryCalledWithID cron.EntryID
}

func (m *mockJobExecutorCron) Entry(id cron.EntryID) cron.Entry {
	m.mu.Lock()
	m.entryCalledWithID = id
	m.mu.Unlock()
	if m.EntryFunc != nil {
		return m.EntryFunc(id)
	}
	// Default: return a basic entry with a future next run time
	return cron.Entry{
		ID:   id,
		Next: time.Now().Add(1 * time.Hour),
	}
}
func (m *mockJobExecutorCron) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entryCalledWithID = 0
}

// Mock JobExecutorTransferExecutor
var _ JobExecutorTransferExecutor = (*mockJobExecutorTransferExecutor)(nil)

type mockJobExecutorTransferExecutor struct {
	mu                        sync.Mutex
	ExecuteConfigTransferFunc func(job db.Job, config db.TransferConfig, history *db.JobHistory)

	// Store calls
	executeConfigTransferCalls []map[string]interface{}
}

func (m *mockJobExecutorTransferExecutor) executeConfigTransfer(job db.Job, config db.TransferConfig, history *db.JobHistory) {
	m.mu.Lock()
	m.executeConfigTransferCalls = append(m.executeConfigTransferCalls, map[string]interface{}{
		"job": job, "config": config, "history": history,
	})
	m.mu.Unlock()
	if m.ExecuteConfigTransferFunc != nil {
		m.ExecuteConfigTransferFunc(job, config, history)
	}
	// Default: Do nothing, just record the call
}
func (m *mockJobExecutorTransferExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executeConfigTransferCalls = nil
}

// Mock JobExecutorNotifier
var _ JobExecutorNotifier = (*mockJobExecutorNotifier)(nil)

type mockJobExecutorNotifier struct {
	mu                    sync.Mutex
	SendNotificationsFunc func(job *db.Job, history *db.JobHistory, config *db.TransferConfig)

	// Store calls
	sendNotificationsCalls []map[string]interface{}
}

func (m *mockJobExecutorNotifier) SendNotifications(job *db.Job, history *db.JobHistory, config *db.TransferConfig) {
	m.mu.Lock()
	m.sendNotificationsCalls = append(m.sendNotificationsCalls, map[string]interface{}{
		"job": job, "history": history, "config": config,
	})
	m.mu.Unlock()
	if m.SendNotificationsFunc != nil {
		m.SendNotificationsFunc(job, history, config)
	}
}
func (m *mockJobExecutorNotifier) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendNotificationsCalls = nil
}

// --- Test Setup ---

type testJobExecutorComponents struct {
	db       *mockJobExecutorDB
	logger   *Logger
	logBuf   *bytes.Buffer
	cron     *mockJobExecutorCron
	transfer *mockJobExecutorTransferExecutor
	notifier *mockJobExecutorNotifier
	executor *JobExecutor
	jobsMap  map[uint]cron.EntryID
	jobMutex *sync.Mutex
}

func setupTestJobExecutor() testJobExecutorComponents {
	dbMock := &mockJobExecutorDB{}
	logger, logBuf := newTestLogger(LogLevelDebug)
	cronMock := &mockJobExecutorCron{}
	transferMock := &mockJobExecutorTransferExecutor{}
	notifierMock := &mockJobExecutorNotifier{}
	jobsMap := make(map[uint]cron.EntryID)
	var jobMutex sync.Mutex

	executor := NewJobExecutor(dbMock, logger, cronMock, jobsMap, &jobMutex, transferMock, notifierMock)

	return testJobExecutorComponents{
		db:       dbMock,
		logger:   logger,
		logBuf:   logBuf,
		cron:     cronMock,
		transfer: transferMock,
		notifier: notifierMock,
		executor: executor,
		jobsMap:  jobsMap,
		jobMutex: &jobMutex,
	}
}

// --- Tests ---

func TestExecuteJob_Success(t *testing.T) {
	comps := setupTestJobExecutor()
	defer comps.logger.Close()

	testJobID := uint(1)
	testCronEntryID := cron.EntryID(10)
	comps.jobsMap[testJobID] = testCronEntryID // Simulate job being scheduled

	// Configure mocks
	comps.db.GetConfigsForJobFunc = func(jobID uint) ([]db.TransferConfig, error) {
		if jobID != testJobID {
			t.Errorf("GetConfigsForJob called with wrong jobID: got %d, want %d", jobID, testJobID)
		}
		// Return configs in a different order than job.ConfigIDs to test ordering logic
		return []db.TransferConfig{
			{ID: 2, Name: "Config 2"},                    // Corrected initialization
			{ID: 1, Name: "Config 1"},                    // Corrected initialization
			{ID: 3, Name: "Config 3 (Not in Job Order)"}, // Corrected initialization
		}, nil
	}
	// Ensure the job returned by First has the expected ConfigIDs order
	comps.db.FirstFunc = func(dest interface{}, conds ...interface{}) *gorm.DB {
		if job, ok := dest.(*db.Job); ok {
			job.ID = testJobID
			job.Name = "Test Job Success"
			job.ConfigIDs = "1,2" // Explicit order
			enabled := true
			job.Enabled = &enabled
			return &gorm.DB{Error: nil}
		}
		return &gorm.DB{Error: gorm.ErrRecordNotFound}
	}

	// Execute the job
	comps.executor.executeJob(testJobID)

	// Assertions
	// 1. DB calls
	comps.db.mu.Lock()
	if comps.db.firstCalledWithDest == nil {
		t.Error("DB First was not called")
	}
	if comps.db.configsForJobID != testJobID {
		t.Errorf("GetConfigsForJob not called with correct jobID: got %d, want %d", comps.db.configsForJobID, testJobID)
	}
	if comps.db.updatedJobStatus == nil {
		t.Error("DB UpdateJobStatus was not called")
	} else if comps.db.updatedJobStatus.LastRun == nil {
		t.Error("LastRun time was not updated")
	} else if comps.db.updatedJobStatus.NextRun == nil {
		t.Error("NextRun time was not updated")
	}
	if comps.db.createdHistory == nil {
		t.Error("DB CreateJobHistory was not called")
	}
	comps.db.mu.Unlock()

	// 2. Cron calls
	comps.cron.mu.Lock()
	if comps.cron.entryCalledWithID != testCronEntryID {
		t.Errorf("Cron Entry not called with correct entryID: got %d, want %d", comps.cron.entryCalledWithID, testCronEntryID)
	}
	comps.cron.mu.Unlock()

	// 3. Notifier calls (via processConfiguration -> transferExecutor)
	comps.notifier.mu.Lock()
	// Expect one call per configuration processed (1, 2, then 3)
	if len(comps.notifier.sendNotificationsCalls) != 3 { // Expect 3 calls now
		t.Errorf("Expected 3 calls to SendNotifications, got %d", len(comps.notifier.sendNotificationsCalls))
	}
	comps.notifier.mu.Unlock()

	// 4. TransferExecutor calls
	comps.transfer.mu.Lock()
	if len(comps.transfer.executeConfigTransferCalls) != 3 { // Expect 3 calls now
		t.Errorf("Expected 3 calls to executeConfigTransfer, got %d", len(comps.transfer.executeConfigTransferCalls))
	} else {
		// Check order (1, 2, then 3)
		call1 := comps.transfer.executeConfigTransferCalls[0]
		call2 := comps.transfer.executeConfigTransferCalls[1]
		call3 := comps.transfer.executeConfigTransferCalls[2]
		if cfg1, ok := call1["config"].(db.TransferConfig); !ok || cfg1.ID != 1 {
			t.Errorf("Expected first transfer call for config ID 1, got %+v", call1["config"])
		}
		if cfg2, ok := call2["config"].(db.TransferConfig); !ok || cfg2.ID != 2 {
			t.Errorf("Expected second transfer call for config ID 2, got %+v", call2["config"])
		}
		if cfg3, ok := call3["config"].(db.TransferConfig); !ok || cfg3.ID != 3 {
			t.Errorf("Expected third transfer call for config ID 3, got %+v", call3["config"])
		}
	}
	comps.transfer.mu.Unlock()

	// 5. Logs
	logOutput := comps.logBuf.String()
	// Check for specific log messages in order
	expectedLogs := []string{
		fmt.Sprintf("Starting execution of job %d", testJobID),
		"Processing job 1 with 3 configurations in specified order", // Uses total configs found
		"Execution order 1/3: Config ID 1",                          // Uses total configs found
		"Processing configuration 1 (1/3) for job 1",                // Log from processConfiguration
		"Execution order 2/3: Config ID 2",                          // Uses total configs found
		"Processing configuration 2 (2/3) for job 1",                // Log from processConfiguration
		"Execution order 3/3: Config ID 3",                          // Uses total configs found
		"Processing configuration 3 (3/3) for job 1",                // Log from processConfiguration for extra config
		fmt.Sprintf("Next run time for job %d", testJobID),
	}
	for _, expectedLog := range expectedLogs {
		if !strings.Contains(logOutput, expectedLog) {
			t.Errorf("Expected log message containing %q not found in output:\n%s", expectedLog, logOutput)
		}
	}
	// Removed extra closing brace
}

func TestExecuteJob_JobNotFound(t *testing.T) {
	comps := setupTestJobExecutor()
	defer comps.logger.Close()
	testJobID := uint(5)

	// Configure mocks
	comps.db.FirstFunc = func(dest interface{}, conds ...interface{}) *gorm.DB {
		return &gorm.DB{Error: gorm.ErrRecordNotFound} // Simulate job not found
	}

	comps.executor.executeJob(testJobID)

	// Assertions
	logOutput := comps.logBuf.String()
	if !strings.Contains(logOutput, fmt.Sprintf("Error loading job %d: record not found", testJobID)) {
		t.Errorf("Expected 'Error loading job' log message not found in output:\n%s", logOutput)
	}
	// Ensure other dependent functions were not called
	comps.db.mu.Lock()
	if comps.db.configsForJobID != 0 {
		t.Error("GetConfigsForJob should not have been called")
	}
	comps.db.mu.Unlock()
	comps.transfer.mu.Lock()
	if len(comps.transfer.executeConfigTransferCalls) > 0 {
		t.Error("executeConfigTransfer should not have been called")
	}
	comps.transfer.mu.Unlock()
}

func TestExecuteJob_ConfigLoadError(t *testing.T) {
	comps := setupTestJobExecutor()
	defer comps.logger.Close()
	testJobID := uint(6)
	dbErr := errors.New("db connection failed")

	// Configure mocks
	comps.db.GetConfigsForJobFunc = func(jobID uint) ([]db.TransferConfig, error) {
		return nil, dbErr // Simulate error loading configs
	}

	comps.executor.executeJob(testJobID)

	// Assertions
	logOutput := comps.logBuf.String()
	if !strings.Contains(logOutput, fmt.Sprintf("Error loading configurations for job %d: %v", testJobID, dbErr)) {
		t.Errorf("Expected 'Error loading configurations' log message not found in output:\n%s", logOutput)
	}
	comps.transfer.mu.Lock()
	if len(comps.transfer.executeConfigTransferCalls) > 0 {
		t.Error("executeConfigTransfer should not have been called")
	}
	comps.transfer.mu.Unlock()
}

func TestExecuteJob_NoConfigs(t *testing.T) {
	comps := setupTestJobExecutor()
	defer comps.logger.Close()
	testJobID := uint(7)

	// Configure mocks
	comps.db.GetConfigsForJobFunc = func(jobID uint) ([]db.TransferConfig, error) {
		return []db.TransferConfig{}, nil // Simulate empty config list
	}

	comps.executor.executeJob(testJobID)

	// Assertions
	logOutput := comps.logBuf.String()
	if !strings.Contains(logOutput, fmt.Sprintf("Error: job %d has no associated configurations", testJobID)) {
		t.Errorf("Expected 'no associated configurations' log message not found in output:\n%s", logOutput)
	}
	comps.transfer.mu.Lock()
	if len(comps.transfer.executeConfigTransferCalls) > 0 {
		t.Error("executeConfigTransfer should not have been called")
	}
	comps.transfer.mu.Unlock()
}

func TestProcessConfiguration_Success(t *testing.T) {
	comps := setupTestJobExecutor()
	defer comps.logger.Close()

	job := db.Job{ID: 1}
	config := db.TransferConfig{ID: 10, Name: "Process Test"} // Corrected initialization
	index := 1
	totalConfigs := 1

	comps.executor.processConfiguration(&job, &config, index, totalConfigs)

	// Assertions
	// 1. DB CreateJobHistory called
	comps.db.mu.Lock()
	if comps.db.createdHistory == nil {
		t.Fatal("CreateJobHistory was not called")
	}
	if comps.db.createdHistory.JobID != job.ID {
		t.Errorf("CreateJobHistory called with wrong JobID: got %d, want %d", comps.db.createdHistory.JobID, job.ID)
	}
	if comps.db.createdHistory.ConfigID != config.ID {
		t.Errorf("CreateJobHistory called with wrong ConfigID: got %d, want %d", comps.db.createdHistory.ConfigID, config.ID)
	}
	if comps.db.createdHistory.Status != "running" {
		t.Errorf("CreateJobHistory called with wrong Status: got %q, want 'running'", comps.db.createdHistory.Status)
	}
	comps.db.mu.Unlock()

	// 2. Notifier SendNotifications called
	comps.notifier.mu.Lock()
	if len(comps.notifier.sendNotificationsCalls) != 1 {
		t.Fatalf("Expected 1 call to SendNotifications, got %d", len(comps.notifier.sendNotificationsCalls))
	}
	callArgs := comps.notifier.sendNotificationsCalls[0]
	if !reflect.DeepEqual(callArgs["job"], &job) {
		t.Errorf("SendNotifications called with wrong job: got %+v, want %+v", callArgs["job"], &job)
	}
	// Compare history partially as StartTime is dynamic
	if histArg, ok := callArgs["history"].(*db.JobHistory); !ok || histArg.JobID != job.ID || histArg.ConfigID != config.ID || histArg.Status != "running" {
		t.Errorf("SendNotifications called with wrong history: got %+v", callArgs["history"])
	}
	if !reflect.DeepEqual(callArgs["config"], &config) {
		t.Errorf("SendNotifications called with wrong config: got %+v, want %+v", callArgs["config"], &config)
	}
	comps.notifier.mu.Unlock()

	// 3. TransferExecutor executeConfigTransfer called
	comps.transfer.mu.Lock()
	if len(comps.transfer.executeConfigTransferCalls) != 1 {
		t.Fatalf("Expected 1 call to executeConfigTransfer, got %d", len(comps.transfer.executeConfigTransferCalls))
	}
	transferCallArgs := comps.transfer.executeConfigTransferCalls[0]
	// Need to compare job/config by value as they are passed by value to transferExecutor
	if !reflect.DeepEqual(transferCallArgs["job"], job) {
		t.Errorf("executeConfigTransfer called with wrong job: got %+v, want %+v", transferCallArgs["job"], job)
	}
	if !reflect.DeepEqual(transferCallArgs["config"], config) {
		t.Errorf("executeConfigTransfer called with wrong config: got %+v, want %+v", transferCallArgs["config"], config)
	}
	// Compare history partially
	if histArg, ok := transferCallArgs["history"].(*db.JobHistory); !ok || histArg.JobID != job.ID || histArg.ConfigID != config.ID || histArg.Status != "running" {
		t.Errorf("executeConfigTransfer called with wrong history: got %+v", transferCallArgs["history"])
	}
	comps.transfer.mu.Unlock()
}

func TestProcessConfiguration_HistoryError(t *testing.T) {
	comps := setupTestJobExecutor()
	defer comps.logger.Close()

	job := db.Job{ID: 1}
	config := db.TransferConfig{ID: 10, Name: "History Error Test"} // Corrected initialization
	index := 1
	totalConfigs := 1
	dbErr := errors.New("failed to create history")

	// Configure mock
	comps.db.CreateJobHistoryFunc = func(history *db.JobHistory) error {
		return dbErr
	}

	comps.executor.processConfiguration(&job, &config, index, totalConfigs)

	// Assertions
	// 1. Check log for error
	logOutput := comps.logBuf.String()
	if !strings.Contains(logOutput, fmt.Sprintf("Error creating job history for job %d, config %d: %v", job.ID, config.ID, dbErr)) {
		t.Errorf("Expected 'Error creating job history' log message not found in output:\n%s", logOutput)
	}

	// 2. Ensure Notifier and TransferExecutor were NOT called
	comps.notifier.mu.Lock()
	if len(comps.notifier.sendNotificationsCalls) > 0 {
		t.Error("SendNotifications should not have been called after history error")
	}
	comps.notifier.mu.Unlock()

	comps.transfer.mu.Lock()
	if len(comps.transfer.executeConfigTransferCalls) > 0 {
		t.Error("executeConfigTransfer should not have been called after history error")
	}
	comps.transfer.mu.Unlock()
}
