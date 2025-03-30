package db

import (
	"time"
)

// FileMetadata stores information about processed files
type FileMetadata struct {
	ID              uint   `gorm:"primarykey"`
	JobID           uint   `gorm:"not null;index"`
	Job             Job    `gorm:"foreignkey:JobID"`
	ConfigID        uint   `gorm:"default:0"` // The specific config ID this file was processed with
	FileName        string `gorm:"not null"`
	OriginalPath    string `gorm:"not null"`
	FileSize        int64  `gorm:"not null"`
	FileHash        string `gorm:"index"` // MD5 or other hash for file identity
	CreationTime    time.Time
	ModTime         time.Time
	ProcessedTime   time.Time `gorm:"not null"`
	DestinationPath string    `gorm:"not null"`
	Status          string    `gorm:"not null"` // processed, archived, deleted, etc.
	ErrorMessage    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
