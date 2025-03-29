package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/components/file_metadata"
	"github.com/starfleetcptn/gomft/components/file_metadata/details"
	"github.com/starfleetcptn/gomft/components/file_metadata/list"
	"github.com/starfleetcptn/gomft/components/file_metadata/search"
	"github.com/starfleetcptn/gomft/internal/db"
)

// FileMetadataHandler handles displaying and searching file metadata
type FileMetadataHandler struct {
	DB *db.DB
}

type UserIDKey string

const userIDKey UserIDKey = "userID"

// ListFileMetadata displays a list of file metadata with pagination and filtering options
func (h *FileMetadataHandler) ListFileMetadata(c *gin.Context) {
	userID := c.GetUint("userID")

	// Query parameters for pagination and filtering
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}

	status := c.Query("status")
	jobIDStr := c.Query("job_id")
	fileName := c.Query("filename")
	sortBy := c.DefaultQuery("sort_by", "processed_time")
	sortDir := c.DefaultQuery("sort_dir", "desc")

	// Validate sort parameters
	allowedSortColumns := map[string]string{
		"id":             "file_metadata.id",
		"filename":       "file_metadata.file_name",
		"size":           "file_metadata.file_size",
		"processed_time": "file_metadata.processed_time",
		"status":         "file_metadata.status",
	}
	dbColumn, ok := allowedSortColumns[sortBy]
	if !ok {
		sortBy = "processed_time" // Default sort column
		dbColumn = allowedSortColumns[sortBy]
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc" // Default sort direction
	}
	orderClause := fmt.Sprintf("%s %s", dbColumn, sortDir)

	// Base query
	query := h.DB.DB.Model(&db.FileMetadata{}).Joins("JOIN jobs ON file_metadata.job_id = jobs.id")

	// Apply filters
	if jobIDStr != "" {
		jobID, _ := strconv.ParseUint(jobIDStr, 10, 64)
		query = query.Where("file_metadata.job_id = ?", jobID)
	} else {
		// Only show files from jobs created by the current user
		query = query.Where("jobs.created_by = ?", userID)
	}

	if status != "" {
		query = query.Where("file_metadata.status = ?", status)
	}

	if fileName != "" {
		query = query.Where("file_metadata.file_name LIKE ?", "%"+fileName+"%")
	}

	// Count total records for pagination
	var totalCount int64
	query.Count(&totalCount)

	// Retrieve file metadata with pagination
	var fileMetadata []db.FileMetadata
	offset := (page - 1) * limit
	err := query.Preload("Job").Preload("Job.Config").
		Order(orderClause). // Use dynamic order clause
		Offset(offset).Limit(limit).
		Find(&fileMetadata).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file metadata"})
		return
	}

	// Create context for template
	ctx := components.CreateTemplateContext(c)

	// Render the file metadata list template
	data := file_metadata.FileMetadataListData{
		Files:      fileMetadata,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: int(totalCount) / limit,
		Filter: file_metadata.FileMetadataFilter{
			Status:   status,
			JobID:    jobIDStr,
			FileName: fileName,
		},
		SortBy:  sortBy,  // Pass sorting info
		SortDir: sortDir, // Pass sorting info
	}

	// If total count is not exactly divisible by limit, add one more page
	if int(totalCount)%limit > 0 {
		data.TotalPages++
	}

	// Check if this is an HTMX request
	isHtmxRequest := c.GetHeader("HX-Request") == "true" || c.Query("htmx") == "true"

	c.Header("Content-Type", "text/html")

	if isHtmxRequest {
		// For HTMX requests, render just the partial template
		list.FileMetadataListPartial(ctx, data, "/files/partial", "#file-list-container").Render(ctx, c.Writer)
	} else {
		// For full page requests, render the complete template
		list.FileMetadataList(ctx, data).Render(ctx, c.Writer)
	}
}

// GetFileMetadataDetails displays detailed information about a specific file
func (h *FileMetadataHandler) GetFileMetadataDetails(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get file ID from URL parameter
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	// Retrieve file metadata
	var fileMetadata db.FileMetadata
	err = h.DB.DB.Preload("Job").Preload("Job.Config").First(&fileMetadata, fileID).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Check if the user has access to this file (file must belong to a job created by the user)
	var jobCreator uint
	err = h.DB.DB.Model(&db.Job{}).Where("id = ?", fileMetadata.JobID).Pluck("created_by", &jobCreator).Error
	if err != nil || jobCreator != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view this file"})
		return
	}

	// Create context for template
	ctx := components.CreateTemplateContext(c)

	// Render the file metadata details template
	data := file_metadata.FileMetadataDetailsData{
		File: fileMetadata,
	}

	c.Header("Content-Type", "text/html")
	details.FileMetadataDetails(ctx, data).Render(ctx, c.Writer)
}

// GetFileMetadataForJob displays file metadata for a specific job
func (h *FileMetadataHandler) GetFileMetadataForJob(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get job ID from URL parameter
	jobID, err := strconv.ParseUint(c.Param("job_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	// Check if the user has access to this job
	var job db.Job
	err = h.DB.DB.Where("id = ?", jobID).First(&job).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	if job.CreatedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view this job's files"})
		return
	}

	// Query parameters for pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}

	status := c.Query("status")
	fileName := c.Query("filename")
	sortBy := c.DefaultQuery("sort_by", "processed_time")
	sortDir := c.DefaultQuery("sort_dir", "desc")

	// Validate sort parameters
	allowedSortColumns := map[string]string{
		"id":             "file_metadata.id",
		"filename":       "file_metadata.file_name",
		"size":           "file_metadata.file_size",
		"processed_time": "file_metadata.processed_time",
		"status":         "file_metadata.status",
	}
	dbColumn, ok := allowedSortColumns[sortBy]
	if !ok {
		sortBy = "processed_time" // Default sort column
		dbColumn = allowedSortColumns[sortBy]
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc" // Default sort direction
	}
	orderClause := fmt.Sprintf("%s %s", dbColumn, sortDir)

	// Base query
	query := h.DB.DB.Model(&db.FileMetadata{}).Where("job_id = ?", jobID)

	// Apply filters
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if fileName != "" {
		query = query.Where("file_name LIKE ?", "%"+fileName+"%")
	}

	// Count total records for pagination
	var totalCount int64
	query.Count(&totalCount)

	// Retrieve file metadata with pagination
	var fileMetadata []db.FileMetadata
	offset := (page - 1) * limit
	err = query.Preload("Job").Preload("Job.Config").
		Order(orderClause). // Use dynamic order clause
		Offset(offset).Limit(limit).
		Find(&fileMetadata).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file metadata"})
		return
	}

	// Create context for template
	ctx := components.CreateTemplateContext(c)

	// Render the file metadata list template
	data := file_metadata.FileMetadataListData{
		Files:      fileMetadata,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: int(totalCount) / limit,
		Job:        &job,
		Filter: file_metadata.FileMetadataFilter{
			Status:   status,
			JobID:    strconv.FormatUint(uint64(job.ID), 10),
			FileName: fileName,
		},
		SortBy:  sortBy,  // Pass sorting info
		SortDir: sortDir, // Pass sorting info
	}

	// If total count is not exactly divisible by limit, add one more page
	if int(totalCount)%limit > 0 {
		data.TotalPages++
	}

	// Check if this is an HTMX request
	isHtmxRequest := c.GetHeader("HX-Request") == "true" || c.Query("htmx") == "true"

	c.Header("Content-Type", "text/html")

	if isHtmxRequest {
		// For HTMX requests, render just the partial template
		list.FileMetadataListPartial(ctx, data, "/files/partial", "#file-list-container").Render(ctx, c.Writer)
	} else {
		// For full page requests, render the complete template
		list.FileMetadataList(ctx, data).Render(ctx, c.Writer)
	}
}

// SearchFileMetadata searches file metadata based on various criteria
func (h *FileMetadataHandler) SearchFileMetadata(c *gin.Context) {
	userID := c.GetUint("userID")

	// Query parameters for search and pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}

	status := c.Query("status")
	jobIDStr := c.Query("job_id")
	fileName := c.Query("filename")
	hash := c.Query("hash")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	sortBy := c.DefaultQuery("sort_by", "processed_time")
	sortDir := c.DefaultQuery("sort_dir", "desc")

	// Validate sort parameters
	allowedSortColumns := map[string]string{
		"id":             "file_metadata.id",
		"filename":       "file_metadata.file_name",
		"size":           "file_metadata.file_size",
		"processed_time": "file_metadata.processed_time",
		"status":         "file_metadata.status",
	}
	dbColumn, ok := allowedSortColumns[sortBy]
	if !ok {
		sortBy = "processed_time" // Default sort column
		dbColumn = allowedSortColumns[sortBy]
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc" // Default sort direction
	}
	orderClause := fmt.Sprintf("%s %s", dbColumn, sortDir)

	// Base query
	query := h.DB.DB.Model(&db.FileMetadata{}).Joins("JOIN jobs ON file_metadata.job_id = jobs.id")

	// Apply filters
	if jobIDStr != "" {
		jobID, _ := strconv.ParseUint(jobIDStr, 10, 64)
		query = query.Where("file_metadata.job_id = ?", jobID)
	} else {
		// Only show files from jobs created by the current user
		query = query.Where("jobs.created_by = ?", userID)
	}

	if status != "" {
		query = query.Where("file_metadata.status = ?", status)
	}

	if fileName != "" {
		query = query.Where("file_metadata.file_name LIKE ?", "%"+fileName+"%")
	}

	if hash != "" {
		query = query.Where("file_metadata.file_hash = ?", hash)
	}

	if startDate != "" {
		query = query.Where("file_metadata.processed_time >= ?", startDate)
	}

	if endDate != "" {
		query = query.Where("file_metadata.processed_time <= ?", endDate+" 23:59:59")
	}

	// Count total records for pagination
	var totalCount int64
	query.Count(&totalCount)

	// Retrieve file metadata with pagination
	var fileMetadata []db.FileMetadata
	offset := (page - 1) * limit
	err := query.Preload("Job").Preload("Job.Config").
		Order(orderClause). // Use dynamic order clause
		Offset(offset).Limit(limit).
		Find(&fileMetadata).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file metadata"})
		return
	}

	// Render the file metadata search template
	data := file_metadata.FileMetadataSearchData{
		Files:      fileMetadata,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: int(totalCount) / limit,
		Filter: file_metadata.FileMetadataFilter{
			Status:    status,
			JobID:     jobIDStr,
			FileName:  fileName,
			Hash:      hash,
			StartDate: startDate,
			EndDate:   endDate,
		},
		SortBy:  sortBy,  // Pass sorting info
		SortDir: sortDir, // Pass sorting info
	}

	// If total count is not exactly divisible by limit, add one more page
	if int(totalCount)%limit > 0 {
		data.TotalPages++
	}

	// Create template context using the helper function
	ctx := components.CreateTemplateContext(c)

	// Check if this is an HTMX request
	isHtmxRequest := c.GetHeader("HX-Request") == "true" || c.Query("htmx") == "true"

	c.Header("Content-Type", "text/html")

	if isHtmxRequest {
		// For HTMX requests, render just the partial template
		search.FileMetadataSearchContent(ctx, data).Render(ctx, c.Writer)
	} else {
		// For full page requests, render the complete template
		search.FileMetadataSearch(ctx, data).Render(ctx, c.Writer)
	}
}

// DeleteFileMetadata deletes a file metadata record
func (h *FileMetadataHandler) DeleteFileMetadata(c *gin.Context) {
	userID := c.GetUint("userID")

	fmt.Println("Deleting file metadata")

	// Get file ID from URL parameter
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	// Check if the user has access to this file
	var fileMetadata db.FileMetadata
	err = h.DB.DB.Preload("Job").First(&fileMetadata, fileID).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	var jobCreator uint
	err = h.DB.DB.Model(&db.Job{}).Where("id = ?", fileMetadata.JobID).Pluck("created_by", &jobCreator).Error
	if err != nil || jobCreator != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to delete this file"})
		return
	}

	// Delete the file metadata
	err = h.DB.DeleteFileMetadata(uint(fileID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file metadata"})
		return
	}

	fmt.Println("File deleted successfully")

	// Check if this is an HTMX request
	isHtmxRequest := c.GetHeader("HX-Request") == "true"

	if isHtmxRequest {
		// For HTMX requests, just return a 200 status - client will handle UI updates
		c.Status(http.StatusOK)
	} else {
		// For regular browser requests, redirect to the file list
		c.Redirect(http.StatusFound, "/files")
	}
}

// HandleFileMetadataPartial handles rendering just the partial template for file metadata
func (h *FileMetadataHandler) HandleFileMetadataPartial(c *gin.Context) {
	// Check if this is an HTMX request or a direct browser request
	isHtmxRequest := c.GetHeader("HX-Request") == "true"

	// If it's a direct browser request (not from HTMX), redirect to the full page
	if !isHtmxRequest {
		// Get all query parameters
		query := c.Request.URL.Query()

		// Rebuild query string for the redirect
		c.Redirect(http.StatusFound, "/files?"+query.Encode())
		return
	}

	userID := c.GetUint("userID")

	// Query parameters for filtering and pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}

	status := c.Query("status")
	jobIDStr := c.Query("job_id")
	fileName := c.Query("filename")
	sortBy := c.DefaultQuery("sort_by", "processed_time")
	sortDir := c.DefaultQuery("sort_dir", "desc")

	// Validate sort parameters
	allowedSortColumns := map[string]string{
		"id":             "file_metadata.id",
		"filename":       "file_metadata.file_name",
		"size":           "file_metadata.file_size",
		"processed_time": "file_metadata.processed_time",
		"status":         "file_metadata.status",
	}
	dbColumn, ok := allowedSortColumns[sortBy]
	if !ok {
		sortBy = "processed_time" // Default sort column
		dbColumn = allowedSortColumns[sortBy]
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc" // Default sort direction
	}
	orderClause := fmt.Sprintf("%s %s", dbColumn, sortDir)

	// Base query
	query := h.DB.DB.Model(&db.FileMetadata{}).Joins("JOIN jobs ON file_metadata.job_id = jobs.id")

	// Apply filters
	if jobIDStr != "" {
		jobID, _ := strconv.ParseUint(jobIDStr, 10, 64)
		query = query.Where("file_metadata.job_id = ?", jobID)
	} else {
		// Only show files from jobs created by the current user
		query = query.Where("jobs.created_by = ?", userID)
	}

	if status != "" {
		query = query.Where("file_metadata.status = ?", status)
	}

	if fileName != "" {
		query = query.Where("file_metadata.file_name LIKE ?", "%"+fileName+"%")
	}

	// Count total records for pagination
	var totalCount int64
	query.Count(&totalCount)

	// Retrieve file metadata with pagination
	var fileMetadata []db.FileMetadata
	offset := (page - 1) * limit
	err := query.Preload("Job").Preload("Job.Config").
		Order(orderClause). // Use dynamic order clause
		Offset(offset).Limit(limit).
		Find(&fileMetadata).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file metadata"})
		return
	}

	// Create context for template
	ctx := components.CreateTemplateContext(c)

	// Prepare job pointer if needed
	var job *db.Job
	if jobIDStr != "" {
		jobID, _ := strconv.ParseUint(jobIDStr, 10, 64)
		var jobRecord db.Job
		if err := h.DB.DB.First(&jobRecord, jobID).Error; err == nil {
			job = &jobRecord
		}
	}

	// Render the file metadata list template
	data := file_metadata.FileMetadataListData{
		Files:      fileMetadata,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: int(totalCount) / limit,
		Job:        job,
		Filter: file_metadata.FileMetadataFilter{
			Status:   status,
			JobID:    jobIDStr,
			FileName: fileName,
		},
		SortBy:  sortBy,  // Pass sorting info
		SortDir: sortDir, // Pass sorting info
	}

	// If total count is not exactly divisible by limit, add one more page
	if int(totalCount)%limit > 0 {
		data.TotalPages++
	}

	c.Header("Content-Type", "text/html")
	list.FileMetadataListPartial(ctx, data, "/files/partial", "#file-list-container").Render(ctx, c.Writer)
}

// HandleFileMetadataSearchPartial handles partial updates for search results
func (h *FileMetadataHandler) HandleFileMetadataSearchPartial(c *gin.Context) {
	// Check if this is an HTMX request or a direct browser request
	isHtmxRequest := c.GetHeader("HX-Request") == "true"

	// If it's a direct browser request (not from HTMX), redirect to the full page
	if !isHtmxRequest {
		// Get all query parameters
		query := c.Request.URL.Query()

		// Rebuild query string for the redirect
		c.Redirect(http.StatusFound, "/files/search?"+query.Encode())
		return
	}

	userID := c.GetUint("userID")

	// Query parameters for search and pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}

	status := c.Query("status")
	jobIDStr := c.Query("job_id")
	fileName := c.Query("filename")
	hash := c.Query("hash")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	sortBy := c.DefaultQuery("sort_by", "processed_time")
	sortDir := c.DefaultQuery("sort_dir", "desc")

	// Validate sort parameters
	allowedSortColumns := map[string]string{
		"id":             "file_metadata.id",
		"filename":       "file_metadata.file_name",
		"size":           "file_metadata.file_size",
		"processed_time": "file_metadata.processed_time",
		"status":         "file_metadata.status",
	}
	dbColumn, ok := allowedSortColumns[sortBy]
	if !ok {
		sortBy = "processed_time" // Default sort column
		dbColumn = allowedSortColumns[sortBy]
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc" // Default sort direction
	}
	orderClause := fmt.Sprintf("%s %s", dbColumn, sortDir)

	// Execute the search query
	query := h.DB.DB.Model(&db.FileMetadata{}).Joins("JOIN jobs ON file_metadata.job_id = jobs.id")

	// Apply filters
	if jobIDStr != "" {
		jobID, _ := strconv.ParseUint(jobIDStr, 10, 64)
		query = query.Where("file_metadata.job_id = ?", jobID)
	} else {
		// Only show files from jobs created by the current user
		query = query.Where("jobs.created_by = ?", userID)
	}

	if status != "" {
		query = query.Where("file_metadata.status = ?", status)
	}

	if fileName != "" {
		query = query.Where("file_metadata.file_name LIKE ?", "%"+fileName+"%")
	}

	if hash != "" {
		query = query.Where("file_metadata.file_hash = ?", hash)
	}

	if startDate != "" {
		query = query.Where("file_metadata.processed_time >= ?", startDate)
	}

	if endDate != "" {
		query = query.Where("file_metadata.processed_time <= ?", endDate+" 23:59:59")
	}

	// Count total results
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count files"})
		return
	}

	// Order and paginate the results
	var files []db.FileMetadata
	if err := query.
		Preload("Job").
		Order(orderClause). // Use dynamic order clause
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search files"})
		return
	}

	// Build the template data
	data := file_metadata.FileMetadataSearchData{
		Files:      files,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: int(totalCount) / limit,
		Filter: file_metadata.FileMetadataFilter{
			Status:    status,
			JobID:     jobIDStr,
			FileName:  fileName,
			Hash:      hash,
			StartDate: startDate,
			EndDate:   endDate,
		},
		SortBy:  sortBy,  // Pass sorting info
		SortDir: sortDir, // Pass sorting info
	}

	if int(totalCount)%limit > 0 {
		data.TotalPages++
	}

	// Create template context using the helper function
	ctx := components.CreateTemplateContext(c)

	c.Header("Content-Type", "text/html")
	search.FileMetadataSearchContent(ctx, data).Render(ctx, c.Writer)
}
