package file_metadata

import "github.com/starfleetcptn/gomft/internal/db"

// FileMetadataFilter represents filter parameters for file metadata queries
type FileMetadataFilter struct {
	Status    string
	JobID     string
	FileName  string
	Hash      string
	StartDate string
	EndDate   string
}

// FileMetadataListData contains data for the file metadata list template
type FileMetadataListData struct {
	Files      []db.FileMetadata
	TotalCount int64
	Page       int
	Limit      int
	TotalPages int
	Job        *db.Job // Optional: if viewing files for a specific job
	Filter     FileMetadataFilter
	SortBy     string // Added for sorting
	SortDir    string // Added for sorting ("asc" or "desc")
}

// FileMetadataDetailsData contains data for the file metadata details template
type FileMetadataDetailsData struct {
	File db.FileMetadata
}

// FileMetadataSearchData contains data for the file metadata search template
type FileMetadataSearchData struct {
	Files      []db.FileMetadata
	TotalCount int64
	Page       int
	Limit      int
	TotalPages int
	Filter     FileMetadataFilter
	SortBy     string // Added for sorting
	SortDir    string // Added for sorting ("asc" or "desc")
}
