package web

import (
	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/internal/config"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/email"
	"github.com/starfleetcptn/gomft/internal/scheduler"
	"github.com/starfleetcptn/gomft/internal/web/handlers"
)

// Handler is a wrapper around the handlers package
type Handler struct {
	handlers *handlers.Handlers
}

// NewHandler creates a new Handler instance that delegates to the handlers package
func NewHandler(database *db.DB, scheduler *scheduler.Scheduler, jwtSecret string, dbPath string, backupDir string, cfg *config.Config) (*Handler, error) {
	// Create email service instance
	emailService := email.NewService(cfg)

	// Create handlers instance
	handlersInstance := handlers.NewHandlers(database, scheduler, jwtSecret, dbPath, backupDir, "./logs", emailService)

	return &Handler{
		handlers: handlersInstance,
	}, nil
}

// InitializeRoutes delegates route registration to the handlers package
func (h *Handler) InitializeRoutes(router *gin.Engine) {
	// Register all routes through the handlers package
	h.handlers.RegisterRoutes(router)
}
