# Encryption Security Framework

The Encryption Security Framework provides a comprehensive solution for secure credential handling, encryption/decryption operations, audit logging, monitoring, and key rotation in the GoMFT application.

## Features

- **Security Auditing**: Detailed logging of encryption and decryption operations
- **Security Monitoring**: Real-time monitoring and alerting for security events
- **Key Rotation**: Safe rotation of encryption keys across database models
- **Performance Benchmarking**: Measure encryption performance impact
- **Security Testing**: Comprehensive testing for encryption implementation
- **Secure Log Handling**: Ensures no sensitive data is exposed in logs

## Architecture

The framework follows a modular design with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────┐
│                 Security Framework                      │
├─────────────┬─────────────┬────────────┬───────────────┤
│  Encryption │   Security  │  Security  │  Key Rotation │
│   Service   │   Auditor   │   Monitor  │    Utility    │
└─────────────┴─────────────┴────────────┴───────────────┘
```

### Core Components

1. **SecurityFramework**: The main facade that ties all components together
2. **SecurityAuditor**: Logs encryption-related events with proper sanitization
3. **SecurityMonitor**: Provides monitoring, alerting, and reporting capabilities
4. **RotationUtility**: Manages the process of rotating encryption keys
5. **SecurityTestingFramework**: Tests and benchmarks encryption implementation

## Usage

### Basic Setup

```go
import (
    "github.com/starfleetcptn/gomft/internal/encryption"
    "github.com/starfleetcptn/gomft/internal/encryptionsecurity"
)

// Create dependencies (implement the FrameworkDependencies interface)
deps := YourDependencyProvider()

// Create encryption service
encryptionService, _ := encryption.NewEncryptionService(keyManager)

// Create security framework
securityFramework, _ := encryptionsecurity.NewSecurityFramework(
    db,
    encryptionService,
    encryptionsecurity.DefaultSecurityFrameworkOptions(),
    deps,
)
```

### Encrypt/Decrypt with Auditing

```go
// Encrypt with auditing
encryptedData, err := securityFramework.EncryptWithAudit(
    data,
    "password",
    "StorageProvider",
    userID,
)

// Decrypt with auditing
decryptedData, err := securityFramework.DecryptWithAudit(
    encryptedData,
    "password",
    "StorageProvider",
    userID,
)
```

### Key Rotation

```go
// Setup old and new encryption services
oldService, _ := encryption.NewEncryptionService(oldKeyManager)
newService, _ := encryption.NewEncryptionService(newKeyManager)

// Models to rotate keys for
models := []interface{}{&StorageProvider{}, &OtherModel{}}

// Execute key rotation
stats, err := securityFramework.RotateEncryptionKeys(
    context.Background(),
    oldService,
    newService,
    models,
    adminUserID,
)
```

### Performance Benchmarking

```go
// Benchmark encryption performance (e.g., with 1KB data for 10 seconds)
metrics, _ := securityFramework.BenchmarkEncryptionPerformance(
    1024,
    10 * time.Second,
)

fmt.Printf("Operations per second: %.2f\n", metrics.OperationsPerSecond)
fmt.Printf("Average latency: %v\n", metrics.AverageLatency)
```

### Security Reports

```go
// Generate a security report for the last 24 hours
startTime := time.Now().Add(-24 * time.Hour)
endTime := time.Now()
reportFile, _ := os.Create("security_report.json")
defer reportFile.Close()

securityFramework.GenerateSecurityReport(startTime, endTime, reportFile)
```

## Implementation Details

### Dependency Injection

The framework uses dependency injection to avoid hard dependencies and facilitate testing:

```go
type FrameworkDependencies struct {
    CreateAuditor           func(logPath string, enableDetailed bool) (SecurityAuditor, error)
    CreateMonitor           func(auditor SecurityAuditor) SecurityMonitor
    CreateAlertHandler      func(logPath string) (AlertHandler, error)
    CreateTestingFramework  func(auditor SecurityAuditor, monitor SecurityMonitor) SecurityTestingFramework
    CreateDummyService      func() (*encryption.EncryptionService, error)
    CreateRotationUtility   func(db *gorm.DB, oldService, newService *encryption.EncryptionService, 
                              auditor SecurityAuditor, monitor SecurityMonitor, 
                              options RotationOptions) (RotationUtility, error)
}
```

### Key Rotation Process

1. **Preparation**: Analyze database models to identify encrypted fields
2. **Batch Processing**: Process records in manageable batches
3. **Decryption/Re-encryption**: Decrypt with old key, re-encrypt with new key
4. **Validation**: Verify data integrity after rotation
5. **Monitoring**: Log all activities and create detailed reports

### Security Best Practices

- **Zero Trust Principle**: Never assume data is safe, always validate
- **Defense in Depth**: Multiple layers of security
- **Least Privilege**: Components only have access to what they need
- **Secure Defaults**: Sensible default settings for security
- **Comprehensive Logging**: All security events are logged
- **Monitored Access**: All access to sensitive data is monitored
- **Fail Securely**: On failures, the system defaults to secure state

## Secure Logging

Special attention is paid to ensure sensitive data is never exposed in logs:

- All error messages are sanitized to remove potential sensitive information
- Key material is never logged in any form
- Timestamps and operation metadata are logged without actual data content
- Access to sensitive data is logged without revealing the actual data

## Performance Considerations

- **Batch Processing**: Key rotation is performed in configurable batches
- **Resource Control**: Memory and CPU usage are optimized for encryption operations
- **Timeouts**: All operations have configurable timeouts
- **Benchmarking**: Performance metrics help identify bottlenecks

## Future Enhancements

- **Distributed Coordination**: Support for coordinated key rotation in distributed systems
- **Real-time Metrics**: Integration with metrics collection systems
- **Anomaly Detection**: Machine learning based detection of unusual encryption patterns
- **Compliance Reporting**: Pre-configured reports for common compliance frameworks

## Additional Resources

- [Encryption Package Documentation](../encryption/README.md)
- [Key Rotation Documentation](../encryption/keyrotation/README.md)
- [Database Integration](../../database/encryption_middleware.md) 