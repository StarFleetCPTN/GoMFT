package audit

import (
	"bytes"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestFramework(t *testing.T) (*SecurityTestingFramework, *bytes.Buffer) {
	// Create audit log buffer
	logBuffer := new(bytes.Buffer)

	// Create auditor
	auditor, err := New()
	require.NoError(t, err)

	// Set auditor to use buffer
	auditValue := reflect.ValueOf(auditor).Elem()
	if logField := auditValue.FieldByName("logWriter"); logField.IsValid() && logField.CanSet() {
		logField.Set(reflect.ValueOf(logBuffer))
	}
	if errorField := auditValue.FieldByName("errorWriter"); errorField.IsValid() && errorField.CanSet() {
		errorField.Set(reflect.ValueOf(logBuffer))
	}

	// Create monitor
	monitor := NewSecurityMonitor(auditor)

	// Create framework
	framework := NewSecurityTestingFramework(auditor, monitor)
	framework.SetVerbose(true)

	return framework, logBuffer
}

func setupTestEncryptionService(t *testing.T) *encryption.EncryptionService {
	// Setup test key
	os.Setenv("TEST_ENCRYPTION_KEY", "dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXRlc3Q=") // base64 test key

	t.Cleanup(func() {
		os.Unsetenv("TEST_ENCRYPTION_KEY")
	})

	// Create key manager
	keyManager := encryption.NewKeyManager("TEST_ENCRYPTION_KEY")
	err := keyManager.Initialize()
	require.NoError(t, err)

	// Create encryption service
	service, err := encryption.NewEncryptionService(keyManager)
	require.NoError(t, err)

	return service
}

func TestNewSecurityTestingFramework(t *testing.T) {
	auditor, err := New()
	require.NoError(t, err)

	monitor := NewSecurityMonitor(auditor)

	framework := NewSecurityTestingFramework(auditor, monitor)

	assert.Equal(t, auditor, framework.auditor)
	assert.Equal(t, monitor, framework.monitor)
	assert.Equal(t, "security_test_results", framework.testOutputDir)
	assert.Equal(t, BasicTesting, framework.testLevel)
	assert.Equal(t, os.Stdout, framework.logOutput)
	assert.False(t, framework.verbose)
}

func TestSecurityTestingFramework_SetMethods(t *testing.T) {
	framework, _ := setupTestFramework(t)

	// Test SetOutputDirectory
	framework.SetOutputDirectory("test_dir")
	assert.Equal(t, "test_dir", framework.testOutputDir)

	// Test SetTestingLevel
	framework.SetTestingLevel(ComprehensiveTesting)
	assert.Equal(t, ComprehensiveTesting, framework.testLevel)

	// Test SetVerbose
	framework.SetVerbose(true)
	assert.True(t, framework.verbose)

	// Test SetLogOutput
	buffer := new(bytes.Buffer)
	framework.SetLogOutput(buffer)
	assert.Equal(t, buffer, framework.logOutput)
}

func TestSecurityTestingFramework_BenchmarkEncryptionPerformance(t *testing.T) {
	framework, _ := setupTestFramework(t)
	service := setupTestEncryptionService(t)

	// Run a very short benchmark
	metrics, err := framework.BenchmarkEncryptionPerformance(service, 1024, 100*time.Millisecond)
	require.NoError(t, err)

	// Verify metrics are populated
	assert.True(t, metrics.OperationsPerSecond > 0)
	assert.True(t, metrics.AverageLatency > 0)
	assert.True(t, metrics.MemoryUsageMB >= 0)
	assert.True(t, metrics.CPUUsagePercent >= 0)
}

func TestSecurityTestingFramework_VerifyKeyRotation(t *testing.T) {
	framework, _ := setupTestFramework(t)

	// Setup two different encryption services with different keys
	oldKeyEnv := "TEST_OLD_KEY"
	newKeyEnv := "TEST_NEW_KEY"

	os.Setenv(oldKeyEnv, "b2xka2V5b2xka2V5b2xka2V5b2xka2V5b2xka2V5b2xk")
	os.Setenv(newKeyEnv, "bmV3a2V5bmV3a2V5bmV3a2V5bmV3a2V5bmV3a2V5bmV3")

	t.Cleanup(func() {
		os.Unsetenv(oldKeyEnv)
		os.Unsetenv(newKeyEnv)
	})

	// Create old key manager and service
	oldKeyManager := encryption.NewKeyManager(oldKeyEnv)
	err := oldKeyManager.Initialize()
	require.NoError(t, err)

	oldService, err := encryption.NewEncryptionService(oldKeyManager)
	require.NoError(t, err)

	// Create new key manager and service
	newKeyManager := encryption.NewKeyManager(newKeyEnv)
	err = newKeyManager.Initialize()
	require.NoError(t, err)

	newService, err := encryption.NewEncryptionService(newKeyManager)
	require.NoError(t, err)

	// Test data
	testData := []byte("This is some test data for key rotation verification")

	// Run verification
	result, err := framework.VerifyKeyRotation(oldService, newService, testData)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Details, "Successfully verified key rotation")
}

func TestSecurityTestingFramework_VerifyNoSensitiveDataInLogs(t *testing.T) {
	framework, _ := setupTestFramework(t)

	// Sensitive data to check
	sensitiveData := "very_sensitive_password_123!"

	// Run verification
	result, err := framework.VerifyNoSensitiveDataInLogs(sensitiveData)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Details, "Successfully verified that sensitive data is properly sanitized")
}

func TestSecurityTestingFramework_RunAllTests(t *testing.T) {
	framework, _ := setupTestFramework(t)
	service := setupTestEncryptionService(t)

	// Run tests at basic level
	results, err := framework.RunAllTests(service)
	require.NoError(t, err)

	// Should have 3 basic tests
	assert.Equal(t, 3, len(results))

	// Set to extended level and run again
	framework.SetTestingLevel(ExtendedTesting)
	results, err = framework.RunAllTests(service)
	require.NoError(t, err)

	// Should have 3 basic + 3 extended tests
	assert.Equal(t, 6, len(results))

	// Set to comprehensive level and run again
	framework.SetTestingLevel(ComprehensiveTesting)
	results, err = framework.RunAllTests(service)
	require.NoError(t, err)

	// Should have 3 basic + 3 extended + 4 comprehensive tests
	assert.Equal(t, 10, len(results))
}
