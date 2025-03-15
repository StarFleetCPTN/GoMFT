# GoMFT Testing Guide

This document outlines the testing strategy and approaches for the GoMFT application.

## Testing Structure

The test suite is organized by components, following Go's standard pattern of placing test files alongside the code they test. For each package, we create corresponding `*_test.go` files.

## Test Types

### 1. Unit Tests

Unit tests focus on testing individual functions and components in isolation. Examples include:

- Configuration loading and validation
- Password hashing and validation
- JWT token generation and validation
- Database operations

### 2. Integration Tests

Integration tests verify that different components work together correctly. Examples include:

- Database operations that span multiple tables
- Authentication flows that involve multiple components
- File transfer operations that involve multiple services

### 3. API Tests

API tests verify HTTP endpoints and request handling. Examples include:

- Authentication endpoints
- CRUD operations on resources
- File transfer management endpoints

### 4. Webhook Tests

Webhook tests verify the correct functioning of the webhook notification system. Examples include:

- Webhook URL validation during job creation/update
- Webhook headers JSON validation
- Webhook delivery when jobs complete successfully
- Webhook delivery when jobs fail
- HMAC-SHA256 signature generation and verification
- Custom HTTP headers inclusion in webhook requests

### 5. Admin Tool Tests

Admin Tool tests verify the functionality of administrative interfaces. Examples include:

- Log Viewer functionality
- Database backup and restore operations
- System statistics reporting
- Maintenance functions (e.g., VACUUM)

## Testing Utilities

A central `testutils` package provides common utilities for testing:

- Database setup with in-memory SQLite
- Test user creation
- JWT token generation
- Configuration setup

## Running Tests

To run all tests:

```bash
go test ./...
```

To run tests for a specific package:

```bash
go test ./internal/db
```

To run a specific test:

```bash
go test ./internal/db -run TestUserCRUD
```

To see test coverage:

```bash
go test ./... -cover
```

For a detailed HTML coverage report:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Mocking

For components that depend on external services or complex dependencies, we use mocking techniques:

- In-memory SQLite for database tests
- Mock schedulers for job scheduling tests
- Mock email services for email tests
- Mock HTTP servers for webhook receiver tests
- Mock file system for Log Viewer tests

### Webhook Testing Mocks

For webhook testing, implement the following mocks:

```go
// Example webhook receiver mock
func setupWebhookMock(t *testing.T) (string, chan []byte, chan http.Header) {
    payloadCh := make(chan []byte, 1)
    headersCh := make(chan http.Header, 1)
    
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        payloadCh <- body
        headersCh <- r.Header.Clone()
        w.WriteHeader(http.StatusOK)
    }))
    
    t.Cleanup(func() {
        server.Close()
    })
    
    return server.URL, payloadCh, headersCh
}
```

### Log Viewer Testing Mocks

For Log Viewer testing, implement file system mocks:

```go
// Example log file system mock
func setupLogFilesMock(t *testing.T) string {
    tempDir := t.TempDir()
    
    // Create sample log files
    for i, content := range []string{
        "INFO: Test log entry 1\nERROR: Test error\n",
        "INFO: Test log entry 2\nWARN: Test warning\n",
    } {
        filename := fmt.Sprintf("test_log_%d.log", i)
        err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
        require.NoError(t, err)
    }
    
    return tempDir
}
```

## Test Data

Test data should be created programmatically rather than relying on existing data in the database. This ensures tests are repeatable and isolated.

## Continuous Integration

Tests are automatically run as part of the CI pipeline to ensure code quality and prevent regressions.

## Example Tests

Here are examples of different types of tests:

### Configuration Test Example

```go
// See internal/config/config_test.go
func TestLoad(t *testing.T) {
    // Test loading configuration from environment variables
}
```

### Database Test Example

```go
// See internal/db/db_test.go
func TestUserCRUD(t *testing.T) {
    // Test creating, reading, updating, and deleting users
}
```

### HTTP Handler Test Example

```go
// See internal/web/handlers/basic_handlers_test.go
func TestHandleHome(t *testing.T) {
    // Test handling home page requests
}
```

### Webhook Test Example

```go
// See internal/web/handlers/webhook_test.go
func TestWebhookValidation(t *testing.T) {
    // Set up test environment
    handlers, router, _, _, config := setupJobsTest(t)

    // Add job create route
    router.POST("/jobs/create", handlers.HandleCreateJob)

    // Create job form data with invalid webhook URL
    formData := url.Values{
        "name":              {"Invalid Webhook Job"},
        "config_ids[]":      {strconv.Itoa(int(config.ID))},
        "schedule":          {"*/15 * * * *"},
        "enabled":           {"true"},
        "webhook_enabled":   {"true"},
        "webhook_url":       {"invalid-url"}, // Invalid URL
        "webhook_secret":    {"test-secret"},
        "notify_on_success": {"true"},
        "notify_on_failure": {"true"},
    }

    // Submit form
    req, _ := http.NewRequest("POST", "/jobs/create", strings.NewReader(formData.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    resp := httptest.NewRecorder()
    router.ServeHTTP(resp, req)

    // Should not create job with invalid webhook URL
    assert.NotEqual(t, http.StatusFound, resp.Code)
    assert.Contains(t, resp.Body.String(), "valid URL")
}
```

### Admin Tools Test Example

```go
// See internal/web/handlers/admin_handlers_test.go
func TestLogViewer(t *testing.T) {
    // Set up test environment with mock log files
    logDir := setupLogFilesMock(t)
    t.Setenv("LOGS_DIR", logDir)
    
    handlers, router, _ := setupAdminTest(t)
    
    // Add log viewer route
    router.GET("/admin/logs/view/:filename", handlers.HandleViewLogFile)
    
    // Create request to view log file
    req, _ := http.NewRequest("GET", "/admin/logs/view/test_log_0.log", nil)
    resp := httptest.NewRecorder()
    
    // Add admin user to context
    ctx, _ := gin.CreateTestContext(resp)
    ctx.Set("userID", uint(1))
    ctx.Set("isAdmin", true)
    req = req.WithContext(ctx)
    
    // Serve request
    router.ServeHTTP(resp, req)
    
    // Verify response contains log content
    assert.Equal(t, http.StatusOK, resp.Code)
    assert.Contains(t, resp.Body.String(), "Test log entry 1")
    assert.Contains(t, resp.Body.String(), "Test error")
}
```

## Test Best Practices

1. **Isolation**: Each test should be independent and not rely on the state of other tests.
2. **Coverage**: Aim for high test coverage, especially for critical components.
3. **Readability**: Tests should be easy to read and understand.
4. **Performance**: Tests should run quickly to enable fast feedback cycles.
5. **Maintainability**: Tests should be easy to maintain and update as the codebase evolves.

## Recent Testing Improvements

### Database Layer Testing

The database layer has seen significant improvements in test coverage. Key improvements include:

- Comprehensive CRUD operation tests
- Error handling tests for edge cases
- Transaction tests
- Tests for database initialization and migration

### Web Handlers Testing

#### File Metadata Handlers

We've implemented comprehensive tests for the file metadata handlers:

- `ListFileMetadata`
- `GetFileMetadataDetails`
- `GetFileMetadataForJob`
- `SearchFileMetadata`
- `DeleteFileMetadata`
- `HandleFileMetadataPartial`
- `HandleFileMetadataSearchPartial`

These tests cover:
- Authentication and authorization
- Pagination
- Filtering
- Error handling
- HTMX integration

#### Testing Challenges and Solutions

When testing web handlers, we encountered several challenges:

1. **Authentication**: Tests needed to simulate authenticated users with proper permissions.
2. **HTMX Integration**: Many handlers expect HTMX headers for proper functioning.
3. **HTML Response Validation**: Validating HTML responses can be brittle.

Solutions implemented:
- Created helper functions to set up authentication context
- Added HTMX headers to test requests
- Focused on verifying database state rather than HTML content

## Next Steps for Testing

### Web Handlers

The overall coverage for the web handlers package needs improvement. To improve this, we should focus on:

1. **Authentication Handlers**: Implement tests for login, logout, and registration handlers.
2. **Job Handlers**: Test job creation, modification, and deletion handlers.
3. **Configuration Handlers**: Test transfer configuration management handlers.
4. **Dashboard Handlers**: Test dashboard data retrieval handlers.

### API Layer

The API layer currently has minimal test coverage. We should implement tests for:

1. **API Authentication**: Test API token generation and validation.
2. **API Endpoints**: Test all REST API endpoints.
3. **Error Handling**: Test API error responses.

### Scheduler

The scheduler component needs tests for:

1. **Job Scheduling**: Test scheduling and execution of jobs.
2. **Error Handling**: Test error handling during job execution.
3. **Concurrency**: Test concurrent job execution.

### Performance Testing

Implement performance tests for critical operations:

1. **File Transfer**: Test large file transfer performance.
2. **Database Operations**: Test database performance under load.
3. **API Endpoints**: Test API endpoint performance.

### Webhook Testing

The webhook functionality requires comprehensive testing:

1. **Validation Tests**:
   - Ensure invalid webhook URLs are rejected during job creation/updates
   - Verify malformed JSON in webhook headers is detected and rejected
   - Test validation edge cases (empty URLs, very long URLs, etc.)

2. **Notification Tests**:
   - Verify webhooks are sent for successful job completion when configured
   - Verify webhooks are sent for failed jobs when configured
   - Confirm webhooks are not sent when the feature is disabled
   - Test the conditional notification settings (notify on success, notify on failure)

3. **Security Tests**:
   - Verify HMAC-SHA256 signatures are correctly generated
   - Test signature verification process
   - Ensure webhook secrets are securely handled

4. **Integration Tests**:
   - Set up a mock webhook receiver to catch and validate payloads
   - Test with various job types and configurations
   - Verify all expected payload fields are present and accurate

### Admin Tools Testing

The Admin Tools interface, particularly the Log Viewer, requires testing:

1. **Log Viewer Tests**:
   - Verify all log files are correctly listed and accessible
   - Test the log file content display functionality
   - Verify log download capability works correctly
   - Test refresh functionality updates the log list and content
   - Verify the viewer works correctly with various log file sizes
   - Test compatibility with log rotation

2. **Database Management Tests**:
   - Verify backup creation and listing functionality
   - Test database restore capability
   - Verify backup download functionality
   - Test database optimization functions

3. **System Statistics Tests**:
   - Verify accurate reporting of system metrics (database size, job counts, etc.)
   - Test uptime calculation and display

## Conclusion

Continued focus on testing will ensure the reliability and maintainability of the GoMFT application. By systematically addressing each component, we can achieve high test coverage and confidence in the codebase.

The recent addition of webhook notification capabilities and admin tools, including the Log Viewer, has expanded the testing requirements. These new features involve various aspects of the system, from HTTP handling to file system operations, and require a comprehensive testing approach that considers:

1. **Functionality Testing**: Ensuring the basic functionality works as expected
2. **Edge Case Testing**: Handling invalid input and extreme conditions
3. **Integration Testing**: Verifying the components work together correctly
4. **Security Testing**: Validating security measures like HMAC signatures

By implementing the testing strategies outlined in this document, we can ensure that all components of the GoMFT system, including these newer features, maintain high quality and reliability. 