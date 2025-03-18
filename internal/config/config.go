package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddress  string      `json:"server_address"`
	DataDir        string      `json:"data_dir"`
	BackupDir      string      `json:"backup_dir"`
	JWTSecret      string      `json:"jwt_secret"`
	Email          EmailConfig `json:"email"`
	BaseURL        string      `json:"base_url"`         // Base URL for generating links in emails
	TOTPEncryptKey string      `json:"totp_encrypt_key"` // Encryption key for TOTP secrets
}

type EmailConfig struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromEmail   string `json:"from_email"`
	FromName    string `json:"from_name"`
	ReplyTo     string `json:"reply_to,omitempty"`
	EnableTLS   bool   `json:"enable_tls"`
	RequireAuth bool   `json:"require_auth"`
}

func Load() (*Config, error) {
	// Default configuration
	cfg := &Config{
		ServerAddress:  ":8080",
		DataDir:        "./data",
		BackupDir:      "./backups",
		JWTSecret:      "change_this_to_a_secure_random_string",
		BaseURL:        "http://localhost:8080",
		TOTPEncryptKey: "this-is-a-dev-key-not-for-production!", // Default development key
		Email: EmailConfig{
			Enabled:     false,
			Host:        "smtp.example.com",
			Port:        587,
			Username:    "user@example.com",
			Password:    "your-password",
			FromEmail:   "gomft@example.com",
			FromName:    "GoMFT",
			EnableTLS:   true,
			RequireAuth: true,
		},
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, err
	}

	// First try to load .env from the root directory
	envPath := ".env"
	if _, err := os.Stat(envPath); err == nil {
		// Load .env file
		if err := godotenv.Load(envPath); err != nil {
			return nil, err
		}

		// Override configuration with environment variables
		if serverAddr := os.Getenv("SERVER_ADDRESS"); serverAddr != "" {
			cfg.ServerAddress = serverAddr
		}
		if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
			cfg.DataDir = dataDir
		}
		if backupDir := os.Getenv("BACKUP_DIR"); backupDir != "" {
			cfg.BackupDir = backupDir
		}
		if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
			cfg.JWTSecret = jwtSecret
		}
		if baseURL := os.Getenv("BASE_URL"); baseURL != "" {
			cfg.BaseURL = baseURL
		}
		if totpKey := os.Getenv("TOTP_ENCRYPTION_KEY"); totpKey != "" {
			cfg.TOTPEncryptKey = totpKey
		}

		// Email configuration
		if emailEnabled := os.Getenv("EMAIL_ENABLED"); emailEnabled != "" {
			cfg.Email.Enabled = strings.ToLower(emailEnabled) == "true"
		}
		if emailHost := os.Getenv("EMAIL_HOST"); emailHost != "" {
			cfg.Email.Host = emailHost
		}
		if emailPort := os.Getenv("EMAIL_PORT"); emailPort != "" {
			if port, err := strconv.Atoi(emailPort); err == nil {
				cfg.Email.Port = port
			}
		}
		if emailUsername := os.Getenv("EMAIL_USERNAME"); emailUsername != "" {
			cfg.Email.Username = emailUsername
		}
		if emailPassword := os.Getenv("EMAIL_PASSWORD"); emailPassword != "" {
			cfg.Email.Password = emailPassword
		}
		if emailFromEmail := os.Getenv("EMAIL_FROM_EMAIL"); emailFromEmail != "" {
			cfg.Email.FromEmail = emailFromEmail
		}
		if emailFromName := os.Getenv("EMAIL_FROM_NAME"); emailFromName != "" {
			cfg.Email.FromName = emailFromName
		}
		if emailReplyTo := os.Getenv("EMAIL_REPLY_TO"); emailReplyTo != "" {
			cfg.Email.ReplyTo = emailReplyTo
		}
		if emailEnableTLS := os.Getenv("EMAIL_ENABLE_TLS"); emailEnableTLS != "" {
			cfg.Email.EnableTLS = strings.ToLower(emailEnableTLS) == "true"
		}
		if emailRequireAuth := os.Getenv("EMAIL_REQUIRE_AUTH"); emailRequireAuth != "" {
			cfg.Email.RequireAuth = strings.ToLower(emailRequireAuth) == "true"
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	} else {
		// Create default .env file in root directory if it doesn't exist
		envContent := []string{
			"SERVER_ADDRESS=" + cfg.ServerAddress,
			"DATA_DIR=" + cfg.DataDir,
			"BACKUP_DIR=" + cfg.BackupDir,
			"JWT_SECRET=" + cfg.JWTSecret,
			"BASE_URL=" + cfg.BaseURL,
			"",
			"# Google OAuth configuration (optional, for built-in authentication)",
			"GOOGLE_CLIENT_ID=your_google_client_id",
			"GOOGLE_CLIENT_SECRET=your_google_client_secret",
			"",
			"# Two-Factor Authentication configuration",
			"TOTP_ENCRYPTION_KEY=" + cfg.TOTPEncryptKey,
			"",
			"# Email configuration",
			"EMAIL_ENABLED=" + strconv.FormatBool(cfg.Email.Enabled),
			"EMAIL_HOST=" + cfg.Email.Host,
			"EMAIL_PORT=" + strconv.Itoa(cfg.Email.Port),
			"EMAIL_FROM_EMAIL=" + cfg.Email.FromEmail,
			"EMAIL_FROM_NAME=" + cfg.Email.FromName,
			"EMAIL_REPLY_TO=" + cfg.Email.ReplyTo,
			"EMAIL_ENABLE_TLS=" + strconv.FormatBool(cfg.Email.EnableTLS),
			"EMAIL_REQUIRE_AUTH=" + strconv.FormatBool(cfg.Email.RequireAuth),
			"EMAIL_USERNAME=" + cfg.Email.Username,
			"EMAIL_PASSWORD=" + cfg.Email.Password,
		}

		if err := os.WriteFile(envPath, []byte(strings.Join(envContent, "\n")), 0644); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
