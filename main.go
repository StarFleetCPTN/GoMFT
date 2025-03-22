package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	// "github.com/starfleetcptn/gomft/internal/api"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/config"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/scheduler"
	"github.com/starfleetcptn/gomft/internal/web"
	"golang.org/x/crypto/bcrypt"
)

//go:embed static
var staticFiles embed.FS

func main() {
	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting GoMFT server version %s...", components.AppVersion)

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
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

	// Initialize scheduler
	scheduler := scheduler.New(database)
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

	// Serve embedded static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem: %v", err)
	}
	router.StaticFS("/static", http.FS(staticFS))
	log.Printf("Embedded static files configured for serving")

	// Initialize web handlers
	webHandler, err := web.NewHandler(database, scheduler, cfg.JWTSecret, dbPath, cfg.BackupDir, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize web handlers: %v", err)
	}
	webHandler.InitializeRoutes(router)
	log.Printf("Web handlers initialized successfully")

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
