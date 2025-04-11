package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	// "github.com/starfleetcptn/gomft/internal/api"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/config"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/logging"
	"github.com/starfleetcptn/gomft/internal/scheduler"
	"github.com/starfleetcptn/gomft/internal/web"
	"golang.org/x/crypto/bcrypt"
)

//go:embed static
var staticFiles embed.FS

// setCacheHeaders sets appropriate cache headers based on file type
func setCacheHeaders(c *gin.Context, path string) {
	// Set cache headers based on file type
	if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
		c.Header("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	} else if strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".webp") {
		c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	} else {
		c.Header("Cache-Control", "no-cache")
	}
}

// setContentType sets the correct content type based on file extension
func setContentType(c *gin.Context, path string) {
	switch {
	case strings.HasSuffix(path, ".js"):
		c.Header("Content-Type", "application/javascript")
	case strings.HasSuffix(path, ".css"):
		c.Header("Content-Type", "text/css")
	case strings.HasSuffix(path, ".png"):
		c.Header("Content-Type", "image/png")
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		c.Header("Content-Type", "image/jpeg")
	case strings.HasSuffix(path, ".webp"):
		c.Header("Content-Type", "image/webp")
	case strings.HasSuffix(path, ".svg"):
		c.Header("Content-Type", "image/svg+xml")
	case strings.HasSuffix(path, ".woff2"):
		c.Header("Content-Type", "font/woff2")
	case strings.HasSuffix(path, ".woff"):
		c.Header("Content-Type", "font/woff")
	}
}

func main() {
	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Ensure logs directory exists
	logsDir := filepath.Join(cfg.DataDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Printf("Failed to create logs directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with file output and WebSocket broadcasting
	if err := logging.Setup(logsDir, true); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Starting GoMFT server version %s...", components.AppVersion)
	log.Printf("Configuration loaded successfully")

	// Ensure required directories exist
	dirs := []string{
		cfg.DataDir,
		cfg.BackupDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
	log.Printf("Required directories created")

	// Initialize database
	dbPath := filepath.Join(cfg.DataDir, "gomft.db")
	database, err := db.Initialize(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	log.Printf("Database initialized successfully")

	// Create default admin user if no users exist
	var count int64
	database.Model(&db.User{}).Count(&count)
	if count == 0 {
		log.Printf("No users found, creating default admin user")
		// Generate password hash
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}

		// Create admin user
		adminUser := &db.User{
			Email:              "admin@example.com",
			PasswordHash:       string(hashedPassword),
			LastPasswordChange: time.Now(),
		}
		adminUser.SetIsAdmin(true)

		if err := database.CreateUser(adminUser); err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Printf("Default admin user created successfully")

		// Assign admin role to admin user
		if err := database.AssignRoleToUser(adminUser.ID, 1, 1); err != nil {
			log.Fatalf("Failed to assign admin role to admin user: %v", err)
		}
		log.Printf("Admin role assigned to admin user successfully")
	}

	// Initialize scheduler components
	log.Printf("Initializing scheduler components...")
	schedLogger := scheduler.NewLogger()                                    // Create the scheduler's logger
	schedCron := cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger))) // Create cron instance
	schedCron.Start()
	jobsMap := make(map[uint]cron.EntryID)
	var jobMutex sync.Mutex

	// Ensure *db.DB implements the required interfaces.
	// If not, this will cause a compile error later.
	// We assume for now that *db.DB has the necessary methods.
	var dbNotifier scheduler.NotificationDB = database
	var dbMetadata scheduler.MetadataDB = database
	var dbTransfer scheduler.TransferDB = database
	var dbJobExecutor scheduler.JobExecutorDB = database
	var dbScheduler scheduler.SchedulerDB = database

	notifier := scheduler.NewNotifier(dbNotifier, schedLogger, cfg.SkipSSLVerify)
	metadataHandler := scheduler.NewMetadataHandler(dbMetadata, schedLogger)
	transferExecutor := scheduler.NewTransferExecutor(dbTransfer, schedLogger, metadataHandler, notifier)
	jobExecutor := scheduler.NewJobExecutor(dbJobExecutor, schedLogger, schedCron, jobsMap, &jobMutex, transferExecutor, notifier)

	// Initialize scheduler with injected components
	scheduler := scheduler.New(
		dbScheduler,
		schedCron,
		schedLogger,
		jobExecutor,
		jobsMap,
		&jobMutex,
	)
	// Defer Stop using the created scheduler instance
	defer scheduler.Stop()
	log.Printf("Scheduler initialized successfully")

	// Initialize Gin router with custom recovery middleware
	router := gin.New()
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %s\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			param.Path,
			param.ErrorMessage,
		)
	}))
	router.Use(gin.Recovery())

	// Serve embedded static files with proper content types and caching
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem: %v", err)
	}

	// Custom static file handler
	router.GET("/static/*filepath", func(c *gin.Context) {
		path := c.Param("filepath")

		// Set content type and cache headers
		setContentType(c, path)
		setCacheHeaders(c, path)

		// Serve the file
		c.FileFromFS(path, http.FS(staticFS))
	})

	log.Printf("Embedded static files configured for serving")

	// Initialize web handlers
	webHandler, err := web.NewHandler(database, scheduler, cfg.JWTSecret, dbPath, cfg.BackupDir, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize web handlers: %v", err)
	}
	webHandler.InitializeRoutes(router)
	log.Printf("Web handlers initialized successfully")

	// Connect scheduler logger to WebSocket broadcast system
	if handlers, ok := web.GetHandlersInstance(); ok && handlers != nil {
		schedLogger.SetBroadcastFunc(handlers.BroadcastLog)
		log.Printf("Scheduler logger connected to WebSocket broadcast system")
	} else {
		log.Printf("Warning: Could not connect scheduler logger to WebSocket broadcast system - handlers not ready")
	}

	// Initialize API routes
	// Commenting out the API routes initialization to avoid route conflicts
	// api.InitializeRoutes(router, database, scheduler, cfg.JWTSecret)
	// log.Printf("API routes initialized successfully")

	// Add middleware for security headers
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	// Start the server
	log.Printf("Starting server on %s", cfg.ServerAddress)
	if err := router.Run(cfg.ServerAddress); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
