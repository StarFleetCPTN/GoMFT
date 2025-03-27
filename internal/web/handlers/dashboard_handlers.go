package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleDashboard handles the GET /dashboard route
func (h *Handlers) HandleDashboard(c *gin.Context) {

	// Get recent job history
	var recentHistory []db.JobHistory
	h.DB.Preload("Job.Config").Order("start_time DESC").Limit(5).Find(&recentHistory)

	// Get job statistics
	var totalJobs int64
	h.DB.Model(&db.JobHistory{}).Where("job_histories.status = 'running' AND job_histories.end_time IS NULL").Count(&totalJobs)

	var completedJobs int64
	h.DB.Model(&db.JobHistory{}).Where("status = ? AND start_time >= ?", "completed", time.Now().AddDate(0, 0, -1)).Count(&completedJobs)

	var failedJobs int64
	h.DB.Model(&db.JobHistory{}).Where("status = ?", "failed").Count(&failedJobs)

	// Create a map to hold all relevant config IDs
	configIDs := make(map[uint]bool)

	// Collect all config IDs from recent history entries
	for _, h := range recentHistory {
		// Add the specific config ID used for this history entry if it exists
		if h.ConfigID > 0 {
			configIDs[h.ConfigID] = true
		}

		// Add the job's default config ID as a fallback
		if h.Job.ConfigID > 0 {
			configIDs[h.Job.ConfigID] = true
		}
	}

	// Create a map to store all configs by their ID
	configsMap := make(map[uint]db.TransferConfig)

	// Load all necessary configurations
	if len(configIDs) > 0 {
		var configsList []db.TransferConfig
		configIDsList := make([]uint, 0, len(configIDs))

		// Extract config IDs from the map
		for id := range configIDs {
			configIDsList = append(configIDsList, id)
		}

		// Load all configurations in one query
		if err := h.DB.Where("id IN ?", configIDsList).Find(&configsList).Error; err == nil {
			// Create the lookup map
			for _, config := range configsList {
				configsMap[config.ID] = config
			}
		}
	}

	// Get the rclone version
	rcloneVersion := components.GetRcloneVersion()

	data := components.DashboardData{
		RecentJobs:      recentHistory,
		ActiveTransfers: int(totalJobs),
		CompletedToday:  int(completedJobs),
		FailedTransfers: int(failedJobs),
		Configs:         configsMap,
		RcloneVersion:   rcloneVersion,
	}

	components.Dashboard(components.CreateTemplateContext(c), data).Render(c, c.Writer)
}

// HandleHistory handles the GET /history route
func (h *Handlers) HandleHistory(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get pagination parameters
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil {
		pageSize = 10
	}
	// Limit page size options
	if pageSize != 10 && pageSize != 25 && pageSize != 50 && pageSize != 100 {
		pageSize = 10
	}

	// Get search term
	searchTerm := c.Query("search")

	// Build the query
	query := h.DB.Model(&db.JobHistory{}).
		Joins("JOIN jobs ON jobs.id = job_histories.job_id").
		Joins("JOIN transfer_configs ON transfer_configs.id = jobs.config_id").
		Where("jobs.created_by = ?", userID)

	// Apply search if provided
	if searchTerm != "" {
		query = query.Where("transfer_configs.name LIKE ? OR job_histories.status LIKE ?",
			"%"+searchTerm+"%", "%"+searchTerm+"%")
	}

	// Count total matching records for pagination
	var total int64
	query.Count(&total)

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	// Ensure page is within bounds
	if page > totalPages {
		page = totalPages
	}

	// Get paginated results
	var history []db.JobHistory
	offset := (page - 1) * pageSize

	query.Offset(offset).
		Limit(pageSize).
		Preload("Job.Config").
		Order("start_time desc").
		Find(&history)

	// If we got no results and we're not on page 1, redirect to page 1
	// Only do this for non-HTMX requests to avoid navigation issues
	isHtmxRequest := c.GetHeader("HX-Request") == "true"
	if len(history) == 0 && page > 1 && total > 0 && !isHtmxRequest {
		redirectURL := fmt.Sprintf("/history?page=1&pageSize=%d", pageSize)
		if searchTerm != "" {
			redirectURL += fmt.Sprintf("&search=%s", url.QueryEscape(searchTerm))
		}
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	// Create a map to hold all relevant config IDs
	configIDs := make(map[uint]bool)

	// Collect all config IDs from history entries
	for _, h := range history {
		// Add the specific config ID used for this history entry if it exists
		if h.ConfigID > 0 {
			configIDs[h.ConfigID] = true
		}

		// Add the job's default config ID as a fallback
		if h.Job.ConfigID > 0 {
			configIDs[h.Job.ConfigID] = true
		}
	}

	// Create a map to store all configs by their ID
	configsMap := make(map[uint]db.TransferConfig)

	// Load all necessary configurations
	if len(configIDs) > 0 {
		var configsList []db.TransferConfig
		configIDsList := make([]uint, 0, len(configIDs))

		// Extract config IDs from the map
		for id := range configIDs {
			configIDsList = append(configIDsList, id)
		}

		// Load all configurations in one query
		if err := h.DB.Where("id IN ?", configIDsList).Find(&configsList).Error; err == nil {
			// Create the lookup map
			for _, config := range configsList {
				configsMap[config.ID] = config
			}
		}
	}

	data := components.HistoryData{
		History:     history,
		CurrentPage: page,
		TotalPages:  totalPages,
		SearchTerm:  searchTerm,
		PageSize:    pageSize,
		Total:       int(total),
		Configs:     configsMap,
	}

	// If this is an HTMX request, only render the history content component
	if isHtmxRequest {
		components.HistoryContent(c, data).Render(c, c.Writer)
	} else {
		components.History(c, data).Render(c, c.Writer)
	}
}

// HandleDashboardData handles the GET /dashboard/data route
func (h *Handlers) HandleDashboardData(c *gin.Context) {
	// Get recent job runs
	var recentRuns []db.JobHistory
	if err := h.DB.Preload("Job").Order("start_time desc").Limit(5).Find(&recentRuns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve recent runs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recent_runs": recentRuns,
	})
}

// HandleDashboardJobsData handles the GET /dashboard/jobs route
func (h *Handlers) HandleDashboardJobsData(c *gin.Context) {
	// Get active jobs
	var activeJobs []db.Job
	if err := h.DB.Where("enabled = ?", true).Find(&activeJobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve active jobs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active_jobs": activeJobs,
	})
}

// HandleDashboardHistoryData handles the GET /dashboard/history route
func (h *Handlers) HandleDashboardHistoryData(c *gin.Context) {
	// Get job history stats
	var successCount int64
	var failureCount int64
	var pendingCount int64

	h.DB.Model(&db.JobHistory{}).Where("status = ?", "success").Count(&successCount)
	h.DB.Model(&db.JobHistory{}).Where("status = ?", "failure").Count(&failureCount)
	h.DB.Model(&db.JobHistory{}).Where("status = ?", "pending").Count(&pendingCount)

	c.JSON(http.StatusOK, gin.H{
		"success_count": successCount,
		"failure_count": failureCount,
		"pending_count": pendingCount,
	})
}
