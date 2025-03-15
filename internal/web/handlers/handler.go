package handlers

import (
	"time"

	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/email"
	"github.com/starfleetcptn/gomft/internal/scheduler"
)

// Handlers contains all the dependencies needed by the handlers
type Handlers struct {
	DB        *db.DB
	Scheduler scheduler.SchedulerInterface
	JWTSecret string
	StartTime time.Time
	DBPath    string
	BackupDir string
	LogsDir   string
	Email     *email.Service
}

// NewHandlers creates a new Handlers instance
func NewHandlers(database *db.DB, scheduler scheduler.SchedulerInterface, jwtSecret string, dbPath string, backupDir string, logsDir string, emailService *email.Service) *Handlers {
	return &Handlers{
		DB:        database,
		Scheduler: scheduler,
		JWTSecret: jwtSecret,
		StartTime: time.Now(),
		DBPath:    dbPath,
		BackupDir: backupDir,
		LogsDir:   logsDir,
		Email:     emailService,
	}
}
