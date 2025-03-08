package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ServerAddress string `json:"server_address"`
	DataDir       string `json:"data_dir"`
	BackupDir     string `json:"backup_dir"`
	JWTSecret     string `json:"jwt_secret"`
	Email         EmailConfig `json:"email"`
	BaseURL       string `json:"base_url"` // Base URL for generating links in emails
}

type EmailConfig struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	FromEmail  string `json:"from_email"`
	FromName   string `json:"from_name"`
	ReplyTo    string `json:"reply_to,omitempty"`
	EnableTLS  bool   `json:"enable_tls"`
	RequireAuth bool  `json:"require_auth"`
}

func Load() (*Config, error) {
	// Default configuration
	cfg := &Config{
		ServerAddress: ":8080",
		DataDir:       filepath.Join("./data", "gomft"),
		BackupDir:     filepath.Join("./data", "gomft", "backups"),
		JWTSecret:     "change_this_to_a_secure_random_string",
		BaseURL:       "http://localhost:8080",
		Email: EmailConfig{
			Enabled:    false,
			Host:       "smtp.example.com",
			Port:       587,
			Username:   "user@example.com",
			Password:   "your-password",
			FromEmail:  "gomft@example.com",
			FromName:   "GoMFT",
			EnableTLS:  true,
			RequireAuth: true,
		},
	}

	// Check if config file exists
	configPath := filepath.Join(cfg.DataDir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		// Read configuration file
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		// Parse configuration
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, err
	}

	// Save configuration if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
