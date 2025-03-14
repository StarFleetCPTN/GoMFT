package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	"github.com/your-project/db"
)

// handleCreateJob handles the creation of a new job
func (h *Handler) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.Logger.Error("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Get form values
	name := r.FormValue("name")
	schedule := r.FormValue("schedule")
	enabled := r.FormValue("enabled")

	// Get config IDs
	configIDs := r.Form["config_ids[]"]

	// Validate required fields
	if len(configIDs) == 0 {
		http.Error(w, "At least one configuration must be selected", http.StatusBadRequest)
		return
	}

	if schedule == "" {
		http.Error(w, "Schedule is required", http.StatusBadRequest)
		return
	}

	// Parse config IDs and validate they exist
	var configIDsList []uint
	for _, configIDStr := range configIDs {
		cID, err := strconv.ParseUint(configIDStr, 10, 32)
		if err != nil {
			h.Logger.Error("Error parsing config ID: %v", err)
			http.Error(w, "Invalid config ID", http.StatusBadRequest)
			return
		}

		// Validate config exists
		var config db.TransferConfig
		if err := h.DB.First(&config, cID).Error; err != nil {
			h.Logger.Error("Config not found: %v", err)
			http.Error(w, fmt.Sprintf("Config ID %d not found", cID), http.StatusBadRequest)
			return
		}

		configIDsList = append(configIDsList, uint(cID))
	}

	// Create job with parsed values
	job := db.Job{
		Name:     name,
		Schedule: schedule,
		Enabled:  enabled == "true",
	}

	// Set config IDs
	job.SetConfigIDsList(configIDsList)

	// ... existing code ...
}

func (h *Handler) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	// Parse path params
	vars := mux.Vars(r)
	jobID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.Logger.Error("Error parsing job ID: %v", err)
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	// Get existing job
	var job db.Job
	if err := h.DB.First(&job, jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Job not found", http.StatusNotFound)
		} else {
			h.Logger.Error("Error getting job: %v", err)
			http.Error(w, "Error getting job", http.StatusInternalServerError)
		}
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.Logger.Error("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Get form values
	name := r.FormValue("name")
	schedule := r.FormValue("schedule")
	enabled := r.FormValue("enabled")

	// Get config IDs
	configIDs := r.Form["config_ids[]"]

	// Validate required fields
	if len(configIDs) == 0 {
		http.Error(w, "At least one configuration must be selected", http.StatusBadRequest)
		return
	}

	if schedule == "" {
		http.Error(w, "Schedule is required", http.StatusBadRequest)
		return
	}

	// Parse config IDs and validate they exist
	var configIDsList []uint
	for _, configIDStr := range configIDs {
		cID, err := strconv.ParseUint(configIDStr, 10, 32)
		if err != nil {
			h.Logger.Error("Error parsing config ID: %v", err)
			http.Error(w, "Invalid config ID", http.StatusBadRequest)
			return
		}

		// Validate config exists
		var config db.TransferConfig
		if err := h.DB.First(&config, cID).Error; err != nil {
			h.Logger.Error("Config not found: %v", err)
			http.Error(w, fmt.Sprintf("Config ID %d not found", cID), http.StatusBadRequest)
			return
		}

		configIDsList = append(configIDsList, uint(cID))
	}

	// Update job with parsed values
	job.Name = name
	job.Schedule = schedule
	job.Enabled = enabled == "true"

	// Set config IDs
	job.SetConfigIDsList(configIDsList)

	// ... existing code ...
}
