package db

import (
	"time"
)

// ConnectorError represents different types of connection errors
type ConnectorError struct {
	Code    string
	Message string
	Err     error
}

// ConnectionResult contains the result of a connection test
type ConnectionResult struct {
	Success   bool
	Message   string
	Error     *ConnectorError
	Timestamp time.Time
}

// Common error codes
const (
	ErrorCodeUnknown          = "unknown"
	ErrorCodeTimeout          = "timeout"
	ErrorCodeAuthentication   = "authentication"
	ErrorCodeConnection       = "connection"
	ErrorCodeResourceNotFound = "resource_not_found"
	ErrorCodeInvalidParams    = "invalid_params"
	ErrorCodePermission       = "permission"
	ErrorCodeNetwork          = "network"
)

// Error returns the error message
func (e *ConnectorError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error
func (e *ConnectorError) Unwrap() error {
	return e.Err
}
