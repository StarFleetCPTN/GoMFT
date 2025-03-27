package db

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// AuditLog represents an audit trail entry in the system
type AuditLog struct {
	gorm.Model
	Action     string          `gorm:"size:50;not null;index"`
	EntityType string          `gorm:"size:50;not null;index"`
	EntityID   uint            `gorm:"not null;index"`
	UserID     uint            `gorm:"not null;index"`
	Details    AuditLogDetails `gorm:"type:json"`
	Timestamp  time.Time       `gorm:"not null;index;default:CURRENT_TIMESTAMP"`
}

// AuditLogDetails is a custom type for storing audit log details as JSON
type AuditLogDetails map[string]interface{}

// Scan implements the sql.Scanner interface
func (d *AuditLogDetails) Scan(value interface{}) error {
	if value == nil {
		*d = make(AuditLogDetails)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, d)
}

// Value implements the driver.Valuer interface
func (d AuditLogDetails) Value() (driver.Value, error) {
	if d == nil {
		return json.Marshal(make(map[string]interface{}))
	}
	return json.Marshal(d)
}
