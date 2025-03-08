package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// HandleDashboard handles the GET /dashboard route
func (h *Handlers) HandleDashboard(c *gin.Context) {
	
	// Get recent job history
	var recentHistory []db.JobHistory
	h.DB.Order("start_time DESC").Limit(5).Find(&recentHistory)
	
	// Get job statistics
	var totalJobs int64
	h.DB.Model(&db.JobHistory{}).Where("job_histories.status = 'running' AND job_histories.end_time IS NULL").Count(&totalJobs)
	
	var completedJobs int64
	h.DB.Model(&db.JobHistory{}).Where("status = ?", "completed").Count(&completedJobs)
	
	var failedJobs int64
	h.DB.Model(&db.JobHistory{}).Where("status = ?", "failed").Count(&failedJobs)
	
	data := components.DashboardData{
		RecentJobs:      recentHistory,
		ActiveTransfers: int(totalJobs),
		CompletedToday:  int(completedJobs),
		FailedTransfers: int(failedJobs),
	}
	
	components.Dashboard(components.CreateTemplateContext(c), data).Render(c, c.Writer)
}

// HandleDashboardStats handles the dashboard stats API request
func (h *Handlers) HandleDashboardStats(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get job statistics
	var activeJobCount int64
	var completedJobCount int64
	var failedJobCount int64

	h.DB.Model(&db.Job{}).Where("created_by = ? AND status = ?", userID, "running").Count(&activeJobCount)
	h.DB.Model(&db.Job{}).Where("created_by = ? AND status = ?", userID, "completed").Count(&completedJobCount)
	h.DB.Model(&db.Job{}).Where("created_by = ? AND status = ?", userID, "failed").Count(&failedJobCount)

	// Get transfer statistics for the last 7 days
	var dailyStats []struct {
		Date      string `json:"date"`
		Completed int64  `json:"completed"`
		Failed    int64  `json:"failed"`
	}

	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
		endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, time.Local)

		var completed int64
		var failed int64

		h.DB.Model(&db.Job{}).
			Where("created_by = ? AND status = ? AND last_run BETWEEN ? AND ?", userID, "completed", startOfDay, endOfDay).
			Count(&completed)

		h.DB.Model(&db.Job{}).
			Where("created_by = ? AND status = ? AND last_run BETWEEN ? AND ?", userID, "failed", startOfDay, endOfDay).
			Count(&failed)

		dailyStats = append(dailyStats, struct {
			Date      string `json:"date"`
			Completed int64  `json:"completed"`
			Failed    int64  `json:"failed"`
		}{
			Date:      startOfDay.Format("2006-01-02"),
			Completed: completed,
			Failed:    failed,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"activeJobs":     activeJobCount,
		"completedJobs":  completedJobCount,
		"failedJobs":     failedJobCount,
		"dailyStats":     dailyStats,
		"uptime":         time.Since(h.StartTime).String(),
		"uptimeSeconds":  int64(time.Since(h.StartTime).Seconds()),
	})
}

// HandleRecentJobs handles the recent jobs API request
func (h *Handlers) HandleRecentJobs(c *gin.Context) {
	userID := c.GetUint("userID")

	var recentJobs []db.Job
	h.DB.Where("created_by = ?", userID).Order("created_at DESC").Limit(5).Find(&recentJobs)

	c.JSON(http.StatusOK, gin.H{
		"recentJobs": recentJobs,
	})
}
