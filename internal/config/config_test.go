package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gomft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up test environment variables
	testEnvVars := map[string]string{
		"SERVER_ADDRESS": ":9090",
		"DATA_DIR":       filepath.Join(tempDir, "data"),
		"BACKUP_DIR":     filepath.Join(tempDir, "backups"),
		"JWT_SECRET":     "test-jwt-secret",
		"BASE_URL":       "http://test.example.com",
		"EMAIL_ENABLED":  "true",
		"EMAIL_HOST":     "smtp.test.com",
		"EMAIL_PORT":     "2525",
		"EMAIL_USERNAME": "test@example.com",
		"EMAIL_PASSWORD": "test-password",
	}

	// Create a temporary .env file
	envContent := ""
	for key, value := range testEnvVars {
		envContent += key + "=" + value + "\n"
		os.Setenv(key, value)
	}

	// Save temporary .env file
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	// Create a symlink to the temp .env file from the project root
	// This is a hack for testing, as the Load() function looks for .env in the root
	currentEnv := ".env"
	// Backup existing .env if it exists
	if _, err := os.Stat(currentEnv); err == nil {
		if err := os.Rename(currentEnv, currentEnv+".bak"); err != nil {
			t.Fatalf("Failed to backup existing .env file: %v", err)
		}
		defer os.Rename(currentEnv+".bak", currentEnv)
	}

	// Create temporary .env for test
	if err := os.WriteFile(currentEnv, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}
	defer os.Remove(currentEnv)

	// Load configuration
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Verify loaded configuration matches expected values
	if cfg.ServerAddress != testEnvVars["SERVER_ADDRESS"] {
		t.Errorf("Expected ServerAddress to be %s, got %s", testEnvVars["SERVER_ADDRESS"], cfg.ServerAddress)
	}
	if cfg.DataDir != testEnvVars["DATA_DIR"] {
		t.Errorf("Expected DataDir to be %s, got %s", testEnvVars["DATA_DIR"], cfg.DataDir)
	}
	if cfg.BackupDir != testEnvVars["BACKUP_DIR"] {
		t.Errorf("Expected BackupDir to be %s, got %s", testEnvVars["BACKUP_DIR"], cfg.BackupDir)
	}
	if cfg.JWTSecret != testEnvVars["JWT_SECRET"] {
		t.Errorf("Expected JWTSecret to be %s, got %s", testEnvVars["JWT_SECRET"], cfg.JWTSecret)
	}
	if cfg.BaseURL != testEnvVars["BASE_URL"] {
		t.Errorf("Expected BaseURL to be %s, got %s", testEnvVars["BASE_URL"], cfg.BaseURL)
	}
	if !cfg.Email.Enabled {
		t.Errorf("Expected Email.Enabled to be true")
	}
	if cfg.Email.Host != testEnvVars["EMAIL_HOST"] {
		t.Errorf("Expected Email.Host to be %s, got %s", testEnvVars["EMAIL_HOST"], cfg.Email.Host)
	}
}
