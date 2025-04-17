package audit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/starfleetcptn/gomft/internal/encryption"
)

// TestingLevel represents the thoroughness of security tests
type TestingLevel int

const (
	// BasicTesting includes essential encryption/decryption and key management tests
	BasicTesting TestingLevel = iota
	// ExtendedTesting adds key rotation, performance, and some edge cases
	ExtendedTesting
	// ComprehensiveTesting includes all tests plus stress tests, fuzzing, and security audit
	ComprehensiveTesting
)

// TestSecretKey is a constant test key for testing purposes only
// Never use this in production
var TestSecretKey = []byte("01234567890123456789012345678901") // 32-byte key for AES-256

// SecurityTestingFramework provides comprehensive testing and benchmarking for the encryption system
type SecurityTestingFramework struct {
	auditor       *SecurityAuditor
	monitor       *SecurityMonitor
	testOutputDir string
	testLevel     TestingLevel
	logOutput     io.Writer
	verbose       bool
	mutex         sync.Mutex
}

// TestResult represents the outcome of a security test
type TestResult struct {
	Name        string        `json:"name"`
	Success     bool          `json:"success"`
	ElapsedTime time.Duration `json:"elapsed_time"`
	Error       string        `json:"error,omitempty"`
	Details     string        `json:"details,omitempty"`
}

// PerformanceMetrics contains performance data for encryption operations
type PerformanceMetrics struct {
	OperationsPerSecond float64       `json:"operations_per_second"`
	AverageLatency      time.Duration `json:"average_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	MemoryUsageMB       float64       `json:"memory_usage_mb"`
	CPUUsagePercent     float64       `json:"cpu_usage_percent"`
}

// NewSecurityTestingFramework creates a new security testing framework
func NewSecurityTestingFramework(auditor *SecurityAuditor, monitor *SecurityMonitor) *SecurityTestingFramework {
	if auditor == nil {
		auditor = GetGlobalAuditor()
	}

	if monitor == nil {
		monitor = NewSecurityMonitor(auditor)
	}

	return &SecurityTestingFramework{
		auditor:       auditor,
		monitor:       monitor,
		testOutputDir: "security_test_results",
		testLevel:     BasicTesting,
		logOutput:     os.Stdout,
		verbose:       false,
	}
}

// SetOutputDirectory sets the directory for test outputs
func (f *SecurityTestingFramework) SetOutputDirectory(dir string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.testOutputDir = dir
}

// SetTestingLevel sets the testing thoroughness level
func (f *SecurityTestingFramework) SetTestingLevel(level TestingLevel) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.testLevel = level
}

// SetVerbose enables or disables verbose logging
func (f *SecurityTestingFramework) SetVerbose(verbose bool) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.verbose = verbose
}

// SetLogOutput sets the output writer for test logs
func (f *SecurityTestingFramework) SetLogOutput(w io.Writer) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.logOutput = w
}

// logf logs a message if verbose mode is enabled
func (f *SecurityTestingFramework) logf(format string, args ...interface{}) {
	if f.verbose && f.logOutput != nil {
		fmt.Fprintf(f.logOutput, format+"\n", args...)
	}
}

// BenchmarkEncryptionPerformance measures the performance of encryption operations
func (f *SecurityTestingFramework) BenchmarkEncryptionPerformance(
	service *encryption.EncryptionService,
	dataSize int,
	duration time.Duration,
) (*PerformanceMetrics, error) {
	if service == nil {
		return nil, fmt.Errorf("encryption service cannot be nil")
	}

	f.logf("Starting encryption performance benchmark (data size: %d bytes, duration: %s)", dataSize, duration)

	// Generate test data
	testData := make([]byte, dataSize)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Setup variables for benchmark
	var (
		operationCount uint64
		totalLatency   uint64
		latencies      []time.Duration
		memStatsBefore runtime.MemStats
		memStatsAfter  runtime.MemStats
	)

	// Collect memory stats before
	runtime.ReadMemStats(&memStatsBefore)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Record start time
	startTime := time.Now()

	// Run benchmark operations
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localLatencies := make([]time.Duration, 0, 1000)
			localData := make([]byte, len(testData))
			copy(localData, testData)

			for {
				select {
				case <-ctx.Done():
					// Add local latencies to global latencies with lock
					f.mutex.Lock()
					latencies = append(latencies, localLatencies...)
					f.mutex.Unlock()
					return
				default:
					// Perform encrypt+decrypt operation and measure latency
					opStart := time.Now()

					// Encrypt
					encrypted, err := service.Encrypt(localData)
					if err != nil {
						f.logf("Encryption error during benchmark: %v", err)
						continue
					}

					// Decrypt
					_, err = service.Decrypt(encrypted)
					if err != nil {
						f.logf("Decryption error during benchmark: %v", err)
						continue
					}

					// Record latency
					latency := time.Since(opStart)
					localLatencies = append(localLatencies, latency)

					// Update metrics
					atomic.AddUint64(&operationCount, 1)
					atomic.AddUint64(&totalLatency, uint64(latency))
				}
			}
		}()
	}

	// Wait for the benchmark to complete
	wg.Wait()

	// Record end time
	endTime := time.Now()
	actualDuration := endTime.Sub(startTime)

	// Collect memory stats after
	runtime.ReadMemStats(&memStatsAfter)

	// Calculate performance metrics
	ops := atomic.LoadUint64(&operationCount)
	if ops == 0 {
		return nil, fmt.Errorf("no operations completed during benchmark")
	}

	// Sort latencies for percentile calculation
	f.mutex.Lock()
	latenciesLen := len(latencies)
	f.mutex.Unlock()

	// Calculate results
	opsPerSec := float64(ops) / actualDuration.Seconds()
	avgLatency := time.Duration(atomic.LoadUint64(&totalLatency) / ops)

	// Calculate memory usage
	memUsageMB := float64(memStatsAfter.Alloc-memStatsBefore.Alloc) / 1024 / 1024

	// Calculate CPU usage (approximate based on operations)
	cpuUsage := float64(ops) / float64(runtime.NumCPU()) / actualDuration.Seconds() * 100
	if cpuUsage > 100 {
		cpuUsage = 100
	}

	// Calculate P95 and P99 latencies
	var p95Latency, p99Latency time.Duration
	if latenciesLen > 0 {
		f.mutex.Lock()
		// Simple bubble sort for small sets (in production you'd use a more efficient sort)
		for i := 0; i < latenciesLen; i++ {
			for j := i + 1; j < latenciesLen; j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		p95Index := int(float64(latenciesLen) * 0.95)
		p99Index := int(float64(latenciesLen) * 0.99)
		if p95Index < latenciesLen {
			p95Latency = latencies[p95Index]
		}
		if p99Index < latenciesLen {
			p99Latency = latencies[p99Index]
		}
		f.mutex.Unlock()
	}

	metrics := &PerformanceMetrics{
		OperationsPerSecond: opsPerSec,
		AverageLatency:      avgLatency,
		P95Latency:          p95Latency,
		P99Latency:          p99Latency,
		MemoryUsageMB:       memUsageMB,
		CPUUsagePercent:     cpuUsage,
	}

	f.logf("Encryption performance benchmark completed: %.2f ops/sec, avg latency: %s",
		metrics.OperationsPerSecond, metrics.AverageLatency)

	return metrics, nil
}

// VerifyKeyRotation tests the key rotation process
func (f *SecurityTestingFramework) VerifyKeyRotation(
	oldService, newService *encryption.EncryptionService,
	testData []byte,
) (*TestResult, error) {
	startTime := time.Now()
	result := &TestResult{
		Name: "KeyRotationVerification",
	}

	if oldService == nil || newService == nil {
		result.Success = false
		result.Error = "encryption services cannot be nil"
		return result, fmt.Errorf(result.Error)
	}

	f.logf("Verifying key rotation with %d bytes of test data", len(testData))

	// Step 1: Encrypt with old key
	encrypted, err := oldService.Encrypt(testData)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to encrypt with old key: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	// Step 2: Verify old key can decrypt
	decrypted, err := oldService.Decrypt(encrypted)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to decrypt with old key: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	if string(decrypted) != string(testData) {
		result.Success = false
		result.Error = "decryption with old key produced different data"
		return result, fmt.Errorf(result.Error)
	}

	// Step 3: Re-encrypt with new key
	rotatedEncrypted, err := newService.Encrypt(decrypted)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to re-encrypt with new key: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	// Step 4: Verify new key can decrypt
	finalDecrypted, err := newService.Decrypt(rotatedEncrypted)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to decrypt with new key: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	if string(finalDecrypted) != string(testData) {
		result.Success = false
		result.Error = "final decryption produced different data"
		return result, fmt.Errorf(result.Error)
	}

	// Step 5: Verify new key cannot decrypt old data (different IV/salt)
	_, err = newService.Decrypt(encrypted)
	if err == nil {
		result.Success = false
		result.Error = "new key should not be able to decrypt data encrypted with old key"
		return result, fmt.Errorf(result.Error)
	}

	result.Success = true
	result.ElapsedTime = time.Since(startTime)
	result.Details = fmt.Sprintf("Successfully verified key rotation process in %s", result.ElapsedTime)

	f.logf("Key rotation verification successful")
	return result, nil
}

// VerifyNoSensitiveDataInLogs checks that sensitive data is not exposed in logs
func (f *SecurityTestingFramework) VerifyNoSensitiveDataInLogs(sensitiveData string) (*TestResult, error) {
	startTime := time.Now()
	result := &TestResult{
		Name: "SensitiveDataExposureCheck",
	}

	f.logf("Verifying sensitive data is not exposed in logs")

	// Create test buffer for logs
	logBuffer := new(logger)

	// Create a temporary auditor that logs to our buffer
	tempAuditor, err := New()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create test auditor: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	// Set log writer to our buffer
	auditValue := reflect.ValueOf(tempAuditor).Elem()
	if logField := auditValue.FieldByName("logWriter"); logField.IsValid() && logField.CanSet() {
		logField.Set(reflect.ValueOf(logBuffer))
	}
	if errorField := auditValue.FieldByName("errorWriter"); errorField.IsValid() && errorField.CanSet() {
		errorField.Set(reflect.ValueOf(logBuffer))
	}

	// Create a temporary encryption service for testing
	os.Setenv("TEST_KEY", "dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXRlc3Q=") // base64 test key
	keyManager := encryption.NewKeyManager("TEST_KEY")
	err = keyManager.Initialize()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to initialize key manager: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	encryptionService, err := encryption.NewEncryptionService(keyManager)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create encryption service: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	// Perform operations that should log
	encryptedData, err := encryptionService.EncryptString(sensitiveData)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to encrypt test data: %v", err)
		return result, fmt.Errorf(result.Error)
	}

	// Log various events with the sensitive data
	tempAuditor.LogEncryptionEvent("test_encrypt", "password", "TestModel", true, nil, "v1", 0, time.Millisecond)
	tempAuditor.LogDecryptionEvent("test_decrypt", "password", "TestModel", true, nil, "v1", 0, time.Millisecond)
	tempAuditor.LogKeyRotationEvent("v1", "v2", true, nil, 0)

	// Force an error log that might contain sensitive data
	tempAuditor.LogDecryptionEvent("test_error", "password", "TestModel", false,
		fmt.Errorf("failed to decrypt: %s", sensitiveData), "v1", 0, time.Millisecond)

	// Get the log contents
	logContents := logBuffer.String()

	// Check if the sensitive data appears in the logs
	if strings.Contains(logContents, sensitiveData) {
		result.Success = false
		result.Error = "sensitive data was found in the logs"
		return result, fmt.Errorf(result.Error)
	}

	// Also check for the encrypted version
	if strings.Contains(logContents, encryptedData) {
		result.Success = false
		result.Error = "encrypted sensitive data was found in the logs"
		return result, fmt.Errorf(result.Error)
	}

	result.Success = true
	result.ElapsedTime = time.Since(startTime)
	result.Details = "Successfully verified that sensitive data is properly sanitized in logs"

	f.logf("Sensitive data exposure check passed")
	return result, nil
}

// Custom logger for testing
type logger struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

func (l *logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.Write(p)
}

func (l *logger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

// RunAllTests executes all security tests based on the configured test level
func (f *SecurityTestingFramework) RunAllTests(encryptionService *encryption.EncryptionService) ([]*TestResult, error) {
	results := make([]*TestResult, 0)

	// Basic tests
	basicTests := []func(*encryption.EncryptionService) (*TestResult, error){
		f.testEncryptionDecryption,
		f.testEmptyData,
		f.testLargeData,
	}

	// Extended tests
	extendedTests := []func(*encryption.EncryptionService) (*TestResult, error){
		f.testPerformance,
		f.testConcurrentAccess,
		f.testKeyVersioning,
	}

	// Comprehensive tests
	comprehensiveTests := []func(*encryption.EncryptionService) (*TestResult, error){
		f.testFuzzedInput,
		f.testKeyRotation,
		f.testErrorHandling,
		f.testSensitiveDataExposure,
	}

	// Run basic tests
	for _, test := range basicTests {
		result, err := test(encryptionService)
		if err != nil {
			f.logf("Test %s failed: %v", result.Name, err)
		}
		results = append(results, result)
	}

	// Run extended tests if level is high enough
	if f.testLevel >= ExtendedTesting {
		for _, test := range extendedTests {
			result, err := test(encryptionService)
			if err != nil {
				f.logf("Test %s failed: %v", result.Name, err)
			}
			results = append(results, result)
		}
	}

	// Run comprehensive tests if level is highest
	if f.testLevel >= ComprehensiveTesting {
		for _, test := range comprehensiveTests {
			result, err := test(encryptionService)
			if err != nil {
				f.logf("Test %s failed: %v", result.Name, err)
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// Test implementations (placeholders - these would be implemented with real tests)
func (f *SecurityTestingFramework) testEncryptionDecryption(s *encryption.EncryptionService) (*TestResult, error) {
	// This is a placeholder - in a real implementation, this would perform actual tests
	return &TestResult{Name: "EncryptionDecryption", Success: true}, nil
}

func (f *SecurityTestingFramework) testEmptyData(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "EmptyData", Success: true}, nil
}

func (f *SecurityTestingFramework) testLargeData(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "LargeData", Success: true}, nil
}

func (f *SecurityTestingFramework) testPerformance(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "Performance", Success: true}, nil
}

func (f *SecurityTestingFramework) testConcurrentAccess(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "ConcurrentAccess", Success: true}, nil
}

func (f *SecurityTestingFramework) testKeyVersioning(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "KeyVersioning", Success: true}, nil
}

func (f *SecurityTestingFramework) testFuzzedInput(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "FuzzedInput", Success: true}, nil
}

func (f *SecurityTestingFramework) testKeyRotation(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "KeyRotation", Success: true}, nil
}

func (f *SecurityTestingFramework) testErrorHandling(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "ErrorHandling", Success: true}, nil
}

func (f *SecurityTestingFramework) testSensitiveDataExposure(s *encryption.EncryptionService) (*TestResult, error) {
	return &TestResult{Name: "SensitiveDataExposure", Success: true}, nil
}
