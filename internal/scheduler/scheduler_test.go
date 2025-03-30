package scheduler

import (
	"context"
	"fmt"
	"reflect" // Added import
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/starfleetcptn/gomft/internal/db"
)

// --- Mock Implementations ---

// Mock SchedulerDB
var _ SchedulerDB = (*mockSchedulerDB)(nil)

type mockSchedulerDB struct {
	mu                  sync.Mutex
	GetActiveJobsFunc   func() ([]db.Job, error)
	UpdateJobStatusFunc func(job *db.Job) error

	// Store calls/data
	getActiveJobsCalls int
	updatedJobStatus   *db.Job
}

func (m *mockSchedulerDB) GetActiveJobs() ([]db.Job, error) {
	m.mu.Lock()
	m.getActiveJobsCalls++
	m.mu.Unlock()
	if m.GetActiveJobsFunc != nil {
		return m.GetActiveJobsFunc()
	}
	// Default: return an empty list
	return []db.Job{}, nil
}
func (m *mockSchedulerDB) UpdateJobStatus(job *db.Job) error {
	m.mu.Lock()
	m.updatedJobStatus = job
	m.mu.Unlock()
	if m.UpdateJobStatusFunc != nil {
		return m.UpdateJobStatusFunc(job)
	}
	return nil // Default success
}
func (m *mockSchedulerDB) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getActiveJobsCalls = 0
	m.updatedJobStatus = nil
}

// Mock SchedulerCron
var _ SchedulerCron = (*mockSchedulerCron)(nil)

type mockSchedulerCron struct {
	mu             sync.Mutex
	addFuncMock    func(spec string, cmd func()) (cron.EntryID, error) // Renamed field
	removeFuncMock func(id cron.EntryID)                               // Renamed field
	entryFuncMock  func(id cron.EntryID) cron.Entry                    // Renamed field
	stopFuncMock   func() context.Context                              // Renamed field

	// Store calls/data
	addedJobs   map[string]func() // spec -> cmd
	removedIDs  []cron.EntryID
	entryCalled cron.EntryID
	stopCalled  bool
}

func (m *mockSchedulerCron) AddFunc(spec string, cmd func()) (cron.EntryID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.addedJobs == nil {
		m.addedJobs = make(map[string]func())
	}
	m.addedJobs[spec] = cmd
	// Use reflect to check if the mock function is set, avoiding the warning
	if reflect.ValueOf(m.addFuncMock).IsValid() && !reflect.ValueOf(m.addFuncMock).IsNil() {
		return m.addFuncMock(spec, cmd)
	}
	// Default: return a mock ID
	return cron.EntryID(len(m.addedJobs)), nil
}
func (m *mockSchedulerCron) Remove(id cron.EntryID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removedIDs = append(m.removedIDs, id)
	// Removed unused loop for spec, addedCmd
	if m.removeFuncMock != nil { // Use renamed field
		m.removeFuncMock(id)
	}
}
func (m *mockSchedulerCron) Entry(id cron.EntryID) cron.Entry {
	m.mu.Lock()
	m.entryCalled = id
	m.mu.Unlock()
	if m.entryFuncMock != nil { // Use renamed field
		return m.entryFuncMock(id)
	}
	// Default: return entry with future time
	return cron.Entry{ID: id, Next: time.Now().Add(time.Hour)}
}
func (m *mockSchedulerCron) Stop() context.Context {
	m.mu.Lock()
	m.stopCalled = true
	m.mu.Unlock()
	if m.stopFuncMock != nil { // Use renamed field
		return m.stopFuncMock()
	}
	return context.Background() // Default context
}
func (m *mockSchedulerCron) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addedJobs = nil
	m.removedIDs = nil
	m.entryCalled = 0
	m.stopCalled = false
}

// Mock SchedulerLogger
var _ SchedulerLogger = (*mockSchedulerLogger)(nil)

type mockSchedulerLogger struct {
	mu             sync.Mutex
	LogInfoFunc    func(format string, v ...interface{})
	LogErrorFunc   func(format string, v ...interface{})
	LogDebugFunc   func(format string, v ...interface{})
	CloseFunc      func()
	RotateLogsFunc func() error
	// PrintlnFunc removed

	// Store calls/data
	infoLogs     []string
	errorLogs    []string
	debugLogs    []string
	closeCalled  bool
	rotateCalled bool
	// printlnLogs removed
}

func (m *mockSchedulerLogger) LogInfo(format string, v ...interface{}) {
	m.mu.Lock()
	m.infoLogs = append(m.infoLogs, fmt.Sprintf(format, v...))
	m.mu.Unlock()
	if m.LogInfoFunc != nil {
		m.LogInfoFunc(format, v...)
	}
}
func (m *mockSchedulerLogger) LogError(format string, v ...interface{}) {
	m.mu.Lock()
	m.errorLogs = append(m.errorLogs, fmt.Sprintf(format, v...))
	m.mu.Unlock()
	if m.LogErrorFunc != nil {
		m.LogErrorFunc(format, v...)
	}
}
func (m *mockSchedulerLogger) LogDebug(format string, v ...interface{}) {
	m.mu.Lock()
	m.debugLogs = append(m.debugLogs, fmt.Sprintf(format, v...))
	m.mu.Unlock()
	if m.LogDebugFunc != nil {
		m.LogDebugFunc(format, v...)
	}
}
func (m *mockSchedulerLogger) Close() {
	m.mu.Lock()
	m.closeCalled = true
	m.mu.Unlock()
	if m.CloseFunc != nil {
		m.CloseFunc()
	}
}
func (m *mockSchedulerLogger) RotateLogs() error {
	m.mu.Lock()
	m.rotateCalled = true
	m.mu.Unlock()
	if m.RotateLogsFunc != nil {
		return m.RotateLogsFunc()
	}
	return nil // Default success
}

// Println method removed
func (m *mockSchedulerLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoLogs = nil
	m.errorLogs = nil
	m.debugLogs = nil
	m.closeCalled = false
	m.rotateCalled = false
	// printlnLogs removed from Reset
}

// Mock SchedulerJobExecutor
var _ SchedulerJobExecutor = (*mockSchedulerJobExecutor)(nil)

type mockSchedulerJobExecutor struct {
	mu             sync.Mutex
	ExecuteJobFunc func(jobID uint)

	// Store calls
	executeJobCalls []uint
}

func (m *mockSchedulerJobExecutor) executeJob(jobID uint) {
	m.mu.Lock()
	m.executeJobCalls = append(m.executeJobCalls, jobID)
	m.mu.Unlock()
	if m.ExecuteJobFunc != nil {
		m.ExecuteJobFunc(jobID)
	}
}
func (m *mockSchedulerJobExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executeJobCalls = nil
}

// --- Test Setup ---

type testSchedulerComponents struct {
	db        *mockSchedulerDB
	cron      *mockSchedulerCron
	logger    *mockSchedulerLogger
	executor  *mockSchedulerJobExecutor
	jobsMap   map[uint]cron.EntryID
	jobMutex  *sync.Mutex
	scheduler *Scheduler
}

func setupTestScheduler() testSchedulerComponents {
	dbMock := &mockSchedulerDB{}
	cronMock := &mockSchedulerCron{}
	loggerMock := &mockSchedulerLogger{}
	executorMock := &mockSchedulerJobExecutor{}
	jobsMap := make(map[uint]cron.EntryID)
	var jobMutex sync.Mutex

	// Create scheduler with mocks
	scheduler := New(
		dbMock,
		cronMock,
		loggerMock,
		executorMock,
		jobsMap,
		&jobMutex,
	)

	return testSchedulerComponents{
		db:        dbMock,
		cron:      cronMock,
		logger:    loggerMock,
		executor:  executorMock,
		jobsMap:   jobsMap,
		jobMutex:  &jobMutex,
		scheduler: scheduler,
	}
}

// --- Tests ---

func TestNewScheduler_LoadJobs(t *testing.T) {
	dbMock := &mockSchedulerDB{}
	cronMock := &mockSchedulerCron{}
	loggerMock := &mockSchedulerLogger{}
	executorMock := &mockSchedulerJobExecutor{}
	jobsMap := make(map[uint]cron.EntryID)
	var jobMutex sync.Mutex

	// Configure DB mock to return jobs
	enabled := true
	disabled := false
	dbMock.GetActiveJobsFunc = func() ([]db.Job, error) {
		return []db.Job{
			{ID: 1, Name: "Job 1", Schedule: "* * * * *", Enabled: &enabled},
			{ID: 2, Name: "Job 2", Schedule: "0 * * * *", Enabled: &enabled},
			{ID: 3, Name: "Job 3", Schedule: "*/5 * * * *", Enabled: &disabled}, // Disabled job
		}, nil
	}

	// Create scheduler - this calls loadJobs internally
	_ = New(dbMock, cronMock, loggerMock, executorMock, jobsMap, &jobMutex)

	// Assertions
	// 1. DB GetActiveJobs called once
	dbMock.mu.Lock()
	if dbMock.getActiveJobsCalls != 1 {
		t.Errorf("Expected GetActiveJobs to be called once, got %d", dbMock.getActiveJobsCalls)
	}
	dbMock.mu.Unlock()

	// 2. Cron AddFunc called twice (for enabled jobs)
	cronMock.mu.Lock()
	if len(cronMock.addedJobs) != 2 {
		t.Errorf("Expected 2 jobs to be added to cron, got %d", len(cronMock.addedJobs))
	}
	// Job 1 schedule is 5 fields, should be converted to 6 by ScheduleJob logic.
	if _, ok := cronMock.addedJobs["0 * * * * *"]; !ok { // Check for 6-field version
		t.Errorf("Expected Job 1 schedule '0 * * * * *' to be added, got map: %v", cronMock.addedJobs)
	}
	// Job 2 schedule is 5 fields, should be converted to 6 by ScheduleJob logic.
	if _, ok := cronMock.addedJobs["0 0 * * * *"]; !ok { // Check for 6-field version
		t.Errorf("Expected Job 2 schedule '0 0 * * * *' to be added, got map: %v", cronMock.addedJobs)
	}
	cronMock.mu.Unlock()

	// 3. Check logs
	loggerMock.mu.Lock()
	foundLoadLog := false
	foundDisabledLog := false
	foundLoadedCountLog := false
	for _, log := range loggerMock.infoLogs {
		if strings.Contains(log, "Loading scheduled jobs") {
			foundLoadLog = true
		}
		if strings.Contains(log, "Job 3 (Job 3) is disabled") {
			foundDisabledLog = true
		}
		if strings.Contains(log, "Loaded 2 jobs") {
			foundLoadedCountLog = true
		}
	}
	if !foundLoadLog {
		t.Error("Expected 'Loading scheduled jobs' log")
	}
	if !foundDisabledLog {
		t.Error("Expected 'Job 3 ... disabled' log")
	}
	if !foundLoadedCountLog {
		t.Error("Expected 'Loaded 2 jobs' log")
	}
	loggerMock.mu.Unlock()
}

func TestScheduleJob_Success(t *testing.T) {
	comps := setupTestScheduler()
	enabled := true
	job := db.Job{ID: 5, Name: "Test Sched", Schedule: "10 * * * *", Enabled: &enabled}

	err := comps.scheduler.ScheduleJob(&job)

	if err != nil {
		t.Fatalf("ScheduleJob failed: %v", err)
	}

	// Assertions
	// 1. Cron AddFunc called
	comps.cron.mu.Lock()
	if len(comps.cron.addedJobs) != 1 {
		t.Fatalf("Expected 1 job added to cron, got %d", len(comps.cron.addedJobs))
	}
	// Job schedule is 5 fields, should be converted to 6 by ScheduleJob logic.
	if _, ok := comps.cron.addedJobs["0 10 * * * *"]; !ok { // Check for 6-field version
		t.Errorf("Expected schedule '0 10 * * * *' to be added, got map: %v", comps.cron.addedJobs)
	}
	comps.cron.mu.Unlock()

	// 2. Job map updated
	comps.jobMutex.Lock()
	if _, ok := comps.jobsMap[job.ID]; !ok {
		t.Errorf("Job ID %d not found in scheduler jobs map", job.ID)
	}
	comps.jobMutex.Unlock()

	// 3. DB UpdateJobStatus called with NextRun set
	comps.db.mu.Lock()
	if comps.db.updatedJobStatus == nil {
		t.Error("UpdateJobStatus was not called")
	} else if comps.db.updatedJobStatus.ID != job.ID {
		t.Errorf("UpdateJobStatus called with wrong job ID: got %d, want %d", comps.db.updatedJobStatus.ID, job.ID)
	} else if comps.db.updatedJobStatus.NextRun == nil {
		t.Error("UpdateJobStatus called but NextRun was not set")
	}
	comps.db.mu.Unlock()
}

func TestScheduleJob_Disabled(t *testing.T) {
	comps := setupTestScheduler()
	disabled := false
	job := db.Job{ID: 6, Name: "Disabled Sched", Schedule: "* * * * *", Enabled: &disabled}

	err := comps.scheduler.ScheduleJob(&job)

	if err != nil {
		t.Fatalf("ScheduleJob failed for disabled job: %v", err)
	}

	// Assertions
	comps.cron.mu.Lock()
	if len(comps.cron.addedJobs) != 0 {
		t.Errorf("Expected 0 jobs added to cron for disabled job, got %d", len(comps.cron.addedJobs))
	}
	comps.cron.mu.Unlock()

	comps.jobMutex.Lock()
	if _, ok := comps.jobsMap[job.ID]; ok {
		t.Errorf("Disabled Job ID %d should not be in scheduler jobs map", job.ID)
	}
	comps.jobMutex.Unlock()

	comps.db.mu.Lock()
	if comps.db.updatedJobStatus != nil {
		t.Error("UpdateJobStatus should not be called for disabled job")
	}
	comps.db.mu.Unlock()

	comps.logger.mu.Lock()
	foundDisabledLog := false
	for _, log := range comps.logger.infoLogs {
		if strings.Contains(log, fmt.Sprintf("Job %d is disabled, skipping scheduling", job.ID)) {
			foundDisabledLog = true
			break
		}
	}
	if !foundDisabledLog {
		t.Error("Expected 'disabled, skipping' log message")
	}
	comps.logger.mu.Unlock()
}

func TestScheduleJob_InvalidCron(t *testing.T) {
	comps := setupTestScheduler()
	enabled := true
	job := db.Job{ID: 7, Name: "Invalid Sched", Schedule: "invalid cron string", Enabled: &enabled}

	err := comps.scheduler.ScheduleJob(&job)

	if err == nil {
		t.Fatal("ScheduleJob succeeded with invalid cron, expected error")
	}
	if !strings.Contains(err.Error(), "invalid cron expression") {
		t.Errorf("Expected error containing 'invalid cron expression', got: %v", err)
	}

	// Assertions
	comps.cron.mu.Lock()
	if len(comps.cron.addedJobs) != 0 {
		t.Errorf("Expected 0 jobs added to cron for invalid schedule, got %d", len(comps.cron.addedJobs))
	}
	comps.cron.mu.Unlock()
}

func TestUnscheduleJob(t *testing.T) {
	comps := setupTestScheduler()
	testJobID := uint(8)
	testEntryID := cron.EntryID(88)

	// Pre-populate the map
	comps.jobsMap[testJobID] = testEntryID

	comps.scheduler.UnscheduleJob(testJobID)

	// Assertions
	// 1. Cron Remove called
	comps.cron.mu.Lock()
	foundRemoved := false
	for _, id := range comps.cron.removedIDs {
		if id == testEntryID {
			foundRemoved = true
			break
		}
	}
	if !foundRemoved {
		t.Errorf("Expected cron Remove to be called with EntryID %d", testEntryID)
	}
	comps.cron.mu.Unlock()

	// 2. Job removed from map
	comps.jobMutex.Lock()
	if _, ok := comps.jobsMap[testJobID]; ok {
		t.Errorf("Job ID %d should have been removed from scheduler jobs map", testJobID)
	}
	comps.jobMutex.Unlock()
}

func TestStop(t *testing.T) {
	comps := setupTestScheduler()
	comps.scheduler.Stop()

	// Assertions
	comps.cron.mu.Lock()
	if !comps.cron.stopCalled {
		t.Error("Expected cron Stop to be called")
	}
	comps.cron.mu.Unlock()

	comps.logger.mu.Lock()
	if !comps.logger.closeCalled {
		t.Error("Expected logger Close to be called")
	}
	comps.logger.mu.Unlock()
}

func TestRunJobNow(t *testing.T) {
	comps := setupTestScheduler()
	testJobID := uint(9)

	err := comps.scheduler.RunJobNow(testJobID)
	if err != nil {
		t.Fatalf("RunJobNow failed: %v", err)
	}

	// Allow time for goroutine to potentially start
	time.Sleep(10 * time.Millisecond)

	// Assertions
	comps.executor.mu.Lock()
	foundCall := false
	for _, id := range comps.executor.executeJobCalls {
		if id == testJobID {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Errorf("Expected executor executeJob to be called with JobID %d", testJobID)
	}
	comps.executor.mu.Unlock()
}
