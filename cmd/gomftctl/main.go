package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/starfleetcptn/gomft/internal/config"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/encryption"
	"github.com/starfleetcptn/gomft/internal/encryption/keyrotation"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Create root command
	rootCmd := &cobra.Command{
		Use:   "gomftctl",
		Short: "GoMFT Control Tool - Command line utilities for GoMFT",
		Long: `GoMFT Control Tool (gomftctl) provides command line utilities for managing 
your GoMFT installation, including database migrations, security key rotation,
and other administrative functions.`,
	}

	// Add commands
	rootCmd.AddCommand(createMigrateCmd())
	rootCmd.AddCommand(createKeyRotationCmd())
	rootCmd.AddCommand(createVersionCmd())
	rootCmd.AddCommand(createBackupCmd())
	rootCmd.AddCommand(createUserCmd())
	rootCmd.AddCommand(createEncryptionKeyRotationCmd())

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// createMigrateCmd creates the migrate command for provider data migration
func createMigrateCmd() *cobra.Command {
	var dryRun, validationOnly, force, debugMode, autoFill bool
	var backupDir string

	migrateCmd := &cobra.Command{
		Use:   "migrate-providers",
		Short: "Migrate provider data to the new storage provider model",
		Long: `Migrate provider data extracts unique provider configurations from existing 
transfer configs and creates dedicated storage provider records.

This command should be run when upgrading from older versions of GoMFT that 
stored provider configuration directly in transfer configs.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}

			// Set backup directory if not provided
			if backupDir == "" {
				backupDir = cfg.BackupDir
			}

			// Initialize database
			dbPath := filepath.Join(cfg.DataDir, "gomft.db")
			database, err := db.Initialize(dbPath)
			if err != nil {
				log.Fatalf("Failed to initialize database: %v", err)
			}
			defer database.Close()

			// Create migration options
			options := db.MigrateProviderDataOptions{
				DryRun:         dryRun,
				ValidationOnly: validationOnly,
				Force:          force,
				BackupDir:      backupDir,
				DebugMode:      debugMode,
				AutoFill:       autoFill,
			}

			// Run migration
			fmt.Println("Starting provider data migration...")
			stats, err := database.MigrateProviderData(options)
			if err != nil {
				fmt.Println("\nMigration failed with error:")
				fmt.Printf("Error: %v\n", err)
				
				// Add more detailed error information
				fmt.Println("\nDetailed error information:")
				fmt.Println("===========================")
				
				// Unwrap nested errors if possible
				var currentErr error = err
				depth := 1
				for currentErr != nil {
					fmt.Printf("%d. %v\n", depth, currentErr)
					if unwrapped, ok := currentErr.(interface{ Unwrap() error }); ok {
						currentErr = unwrapped.Unwrap()
						depth++
					} else {
						break
					}
				}
				
				// Print database connection information (without sensitive details)
				fmt.Println("\nDatabase information:")
				fmt.Printf("- Database path: %s\n", dbPath)
				fmt.Printf("- Migration options: dryRun=%v, validationOnly=%v, force=%v\n", 
					options.DryRun, options.ValidationOnly, options.Force)
				
				log.Fatalf("Migration failed. See details above.")
			}

			// Print report
			fmt.Println(db.FormatMigrationReport(stats))
		},
	}

	// Add flags
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate migration without making changes")
	migrateCmd.Flags().BoolVar(&validationOnly, "validate-only", false, "Only validate if migration is possible without making changes")
	migrateCmd.Flags().BoolVar(&force, "force", false, "Force migration even if validation fails")
	migrateCmd.Flags().StringVar(&backupDir, "backup-dir", "", "Directory to store backup data (defaults to config backup_dir)")
	migrateCmd.Flags().BoolVar(&debugMode, "debug", false, "Enable debug mode with more detailed error messages")
	migrateCmd.Flags().BoolVar(&autoFill, "auto-fill", false, "Automatically fill missing required fields with placeholder values")

	return migrateCmd
}

// createKeyRotationCmd creates the key rotation command
func createKeyRotationCmd() *cobra.Command {
	var keyType string
	var writeToEnv bool

	keyRotationCmd := &cobra.Command{
		Use:   "rotate-key",
		Short: "Rotate security keys used by GoMFT",
		Long: `Rotate security keys generates new cryptographic keys for GoMFT.

Available key types:
- jwt: JSON Web Token signing key
- totp: TOTP encryption key
- encryption: General encryption key used for sensitive data

This command will generate a new key and provide instructions for updating
your configuration. The application must be restarted for changes to take effect.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Validate key type
			validTypes := map[string]string{
				"jwt":        "JWT_SECRET",
				"totp":       "TOTP_ENCRYPTION_KEY",
				"encryption": "GOMFT_ENCRYPTION_KEY",
			}

			envVar, valid := validTypes[keyType]
			if !valid {
				log.Fatalf("Invalid key type: %s. Valid types are: jwt, totp, encryption", keyType)
			}

			// Load configuration
			_, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}

			// Generate a new key
			newKey, err := generateSecureKey()
			if err != nil {
				log.Fatalf("Failed to generate secure key: %v", err)
			}

			fmt.Printf("Generated new %s key: %s\n\n", keyType, newKey)

			if writeToEnv {
				// Read current .env file
				envPath := ".env"
				envContent, err := os.ReadFile(envPath)
				if err != nil {
					log.Fatalf("Failed to read .env file: %v", err)
				}

				// Update .env file with new key
				updatedEnv, updated := updateEnvVar(string(envContent), envVar, newKey)
				if !updated {
					// If the variable wasn't found, append it
					updatedEnv = updatedEnv + fmt.Sprintf("\n%s=%s\n", envVar, newKey)
				}

				// Write updated content back to .env file
				if err := os.WriteFile(envPath, []byte(updatedEnv), 0644); err != nil {
					log.Fatalf("Failed to write updated .env file: %v", err)
				}

				fmt.Printf("Updated %s in .env file\n", envVar)
				fmt.Println("Please restart the GoMFT application for changes to take effect.")
			} else {
				// Print instructions for manual update
				fmt.Println("To use this key, update your .env file with:")
				fmt.Printf("%s=%s\n\n", envVar, newKey)
				fmt.Println("Then restart the GoMFT application for changes to take effect.")
			}
		},
	}

	// Add flags
	keyRotationCmd.Flags().StringVar(&keyType, "type", "", "Type of key to rotate (jwt, totp, encryption)")
	keyRotationCmd.Flags().BoolVar(&writeToEnv, "write", false, "Write the new key directly to .env file")
	keyRotationCmd.MarkFlagRequired("type")

	return keyRotationCmd
}

// createVersionCmd creates the version command
func createVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run: func(cmd *cobra.Command, args []string) {
			// Import the version from the components package
			fmt.Println("GoMFT Control Tool")
			fmt.Println("Version: Same as GoMFT application")
			fmt.Println("Visit https://github.com/starfleetcptn/gomft for more information")
		},
	}
}

// createBackupCmd creates the backup command
func createBackupCmd() *cobra.Command {
	var outputDir string

	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a backup of the GoMFT database",
		Long: `Create a backup of the GoMFT database and configuration.
The backup includes the SQLite database file and the .env configuration file.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}

			// Set output directory if not provided
			if outputDir == "" {
				outputDir = cfg.BackupDir
			}

			// Ensure output directory exists
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				log.Fatalf("Failed to create backup directory: %v", err)
			}

			// Create timestamp for backup filename
			timestamp := fmt.Sprintf("%s", filepath.Base(os.Args[0]))

			// Create backup
			dbPath := filepath.Join(cfg.DataDir, "gomft.db")
			backupPath := filepath.Join(outputDir, fmt.Sprintf("gomft-backup-%s.db", timestamp))

			// Copy database file
			if err := copyFile(dbPath, backupPath); err != nil {
				log.Fatalf("Failed to create database backup: %v", err)
			}

			// Copy .env file if it exists
			envPath := ".env"
			backupEnvPath := filepath.Join(outputDir, fmt.Sprintf("gomft-env-backup-%s.env", timestamp))
			if _, err := os.Stat(envPath); err == nil {
				if err := copyFile(envPath, backupEnvPath); err != nil {
					log.Fatalf("Failed to backup .env file: %v", err)
				}
				fmt.Printf("Configuration backed up to: %s\n", backupEnvPath)
			}

			fmt.Printf("Database backed up to: %s\n", backupPath)
		},
	}

	// Add flags
	backupCmd.Flags().StringVar(&outputDir, "output-dir", "", "Directory to store backup files (defaults to config backup_dir)")

	return backupCmd
}

// createUserCmd creates the user management command
func createUserCmd() *cobra.Command {
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
		Long:  `Commands for managing GoMFT users, including creating, updating, and listing users.`,
	}

	// Add subcommands
	userCmd.AddCommand(createUserCreateCmd())
	userCmd.AddCommand(createUserResetPasswordCmd())
	userCmd.AddCommand(createUserListCmd())

	return userCmd
}

// createUserCreateCmd creates the user create command
func createUserCreateCmd() *cobra.Command {
	var email, password string
	var isAdmin bool

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long:  `Create a new user with the specified email and password.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}

			// Initialize database
			dbPath := filepath.Join(cfg.DataDir, "gomft.db")
			database, err := db.Initialize(dbPath)
			if err != nil {
				log.Fatalf("Failed to initialize database: %v", err)
			}
			defer database.Close()

			// Create user by first generating password hash
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("Failed to hash password: %v", err)
			}

			// Create user object
			user := &db.User{
				Email:              email,
				PasswordHash:       string(hashedPassword),
				LastPasswordChange: time.Now(),
			}

			// Set admin status if requested
			if isAdmin {
				user.SetIsAdmin(true)
			}

			// Save user to database
			if err := database.CreateUser(user); err != nil {
				log.Fatalf("Failed to create user: %v", err)
			}

			fmt.Printf("User created successfully:\n")
			fmt.Printf("  ID: %d\n", user.ID)
			fmt.Printf("  Email: %s\n", user.Email)
			fmt.Printf("  Admin: %t\n", user.GetIsAdmin())
		},
	}

	// Add flags
	createCmd.Flags().StringVar(&email, "email", "", "User email address")
	createCmd.Flags().StringVar(&password, "password", "", "User password")
	createCmd.Flags().BoolVar(&isAdmin, "admin", false, "Grant admin privileges to the user")
	createCmd.MarkFlagRequired("email")
	createCmd.MarkFlagRequired("password")

	return createCmd
}

// createUserResetPasswordCmd creates the user reset-password command
func createUserResetPasswordCmd() *cobra.Command {
	var email, newPassword string

	resetCmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Reset a user's password",
		Long:  `Reset the password for a user with the specified email address.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}

			// Initialize database
			dbPath := filepath.Join(cfg.DataDir, "gomft.db")
			database, err := db.Initialize(dbPath)
			if err != nil {
				log.Fatalf("Failed to initialize database: %v", err)
			}
			defer database.Close()

			// Find user by email
			var user db.User
			if err := database.Where("email = ?", email).First(&user).Error; err != nil {
				log.Fatalf("Failed to find user with email %s: %v", email, err)
			}

			// Generate new password hash
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("Failed to hash password: %v", err)
			}

			// Update user password
			user.PasswordHash = string(hashedPassword)
			user.LastPasswordChange = time.Now()

			// Save user to database
			if err := database.Save(&user).Error; err != nil {
				log.Fatalf("Failed to update user: %v", err)
			}

			fmt.Printf("Password reset successfully for user: %s\n", email)
		},
	}

	// Add flags
	resetCmd.Flags().StringVar(&email, "email", "", "User email address")
	resetCmd.Flags().StringVar(&newPassword, "password", "", "New password")
	resetCmd.MarkFlagRequired("email")
	resetCmd.MarkFlagRequired("password")

	return resetCmd
}

// createUserListCmd creates the user list command
func createUserListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		Long:  `List all users in the GoMFT system.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}

			// Initialize database
			dbPath := filepath.Join(cfg.DataDir, "gomft.db")
			database, err := db.Initialize(dbPath)
			if err != nil {
				log.Fatalf("Failed to initialize database: %v", err)
			}
			defer database.Close()

			// Get all users
			var users []db.User
			if err := database.Find(&users).Error; err != nil {
				log.Fatalf("Failed to get users: %v", err)
			}

			// Print users
			fmt.Println("GoMFT Users:")
			fmt.Println("ID\tEmail\tAdmin\tLast Updated")
			fmt.Println("--------------------------------------------------")
			for _, user := range users {
				lastUpdated := "Never"
				if !user.UpdatedAt.IsZero() {
					lastUpdated = user.UpdatedAt.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("%d\t%s\t%t\t%s\n", user.ID, user.Email, user.GetIsAdmin(), lastUpdated)
			}
		},
	}
}

// createEncryptionKeyRotationCmd creates the encryption key rotation command
func createEncryptionKeyRotationCmd() *cobra.Command {
	var dryRun bool
	var batchSize, maxErrors int
	var backupDir string
	var skipBackup bool
	var oldKeyEnvVar string
	var modelsFlag string

	rotateCmd := &cobra.Command{
		Use:   "rotate-encryption-key",
		Short: "Rotate encryption keys for sensitive data",
		Long: `Rotate encryption keys for sensitive data stored in the database.

This command will:
1. Create a backup of your database (unless --skip-backup is specified)
2. Re-encrypt all sensitive data with a new encryption key
3. Provide instructions for updating your configuration

The application must be stopped before running this command to prevent data corruption.
`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load configuration: %v", err)
			}

			// Set backup directory if not provided
			if backupDir == "" {
				backupDir = cfg.BackupDir
			}

			// Create backup if needed
			if !skipBackup {
				dbPath := filepath.Join(cfg.DataDir, "gomft.db")
				backupPath := filepath.Join(backupDir, fmt.Sprintf("gomft_backup_before_key_rotation_%s.db",
					time.Now().Format("20060102_150405")))

				fmt.Printf("Creating database backup at %s...\n", backupPath)
				if err := copyFile(dbPath, backupPath); err != nil {
					log.Fatalf("Failed to create backup: %v", err)
				}
				fmt.Println("Backup created successfully.")
			} else {
				fmt.Println("Skipping database backup as requested.")
			}

			// Initialize database
			dbPath := filepath.Join(cfg.DataDir, "gomft.db")
			database, err := db.Initialize(dbPath)
			if err != nil {
				log.Fatalf("Failed to initialize database: %v", err)
			}
			defer database.Close()

			// Setup old encryption service
			if oldKeyEnvVar == "" {
				oldKeyEnvVar = encryption.DefaultKeyEnvVar
			}

			// Get the current encryption service
			oldService, err := encryption.GetGlobalEncryptionService()
			if err != nil {
				log.Fatalf("Failed to get current encryption service: %v", err)
			}

			// Generate new key
			newKey, err := encryption.GenerateKey(encryption.AES256KeySize)
			if err != nil {
				log.Fatalf("Failed to generate new encryption key: %v", err)
			}

			// Create new key manager for the new key
			newKeyManager := &keyManager{key: newKey}

			// Setup new encryption service with the new key
			newService, err := encryption.NewEncryptionService(newKeyManager)
			if err != nil {
				log.Fatalf("Failed to create new encryption service: %v", err)
			}

			// Create rotation options
			options := keyrotation.RotationOptions{
				DryRun:    dryRun,
				BatchSize: batchSize,
				MaxErrors: maxErrors,
				Timeout:   24 * time.Hour,
				ProgressCallback: func(modelName string, processed, total int) {
					fmt.Printf("\rProcessing %s: %d/%d records (%.1f%%)",
						modelName, processed, total, float64(processed)/float64(total)*100)
				},
			}

			// Create rotation utility
			rotationUtil, err := keyrotation.NewRotationUtility(
				database.DB, // Use the underlying gorm.DB
				oldService,
				newService,
				nil, // No auditor needed, keyrotation will use the global one
				options,
			)
			if err != nil {
				log.Fatalf("Failed to create rotation utility: %v", err)
			}

			// Find models with encrypted fields
			var models []interface{}
			if modelsFlag == "auto" {
				fmt.Println("Automatically detecting models with encrypted fields...")
				models, err = rotationUtil.FindModelsWithEncryptedFields()
				if err != nil {
					log.Fatalf("Failed to find models with encrypted fields: %v", err)
				}
				if len(models) == 0 {
					log.Fatalf("No models with encrypted fields found")
				}
			} else if modelsFlag != "" {
				// TODO: Support manual model specification
				log.Fatalf("Manual model specification not yet implemented, use --models=auto")
			} else {
				log.Fatalf("No models specified, use --models=auto to automatically detect models")
			}

			// Create migration plan
			fmt.Println("Creating encryption migration plan...")
			plan, err := rotationUtil.CreateEncryptionMigrationPlan(models)
			if err != nil {
				log.Fatalf("Failed to create migration plan: %v", err)
			}

			// Print plan
			fmt.Println("\nEncryption Migration Plan:")
			fmt.Printf("Total models: %d\n", len(plan.ModelPlans))
			fmt.Printf("Total records: %d\n", plan.EstimatedRecords)
			fmt.Printf("Estimated duration: %s\n", plan.EstimatedDuration.Round(time.Second))
			fmt.Println("\nModels to process:")
			for name, modelPlan := range plan.ModelPlans {
				fmt.Printf("- %s: %d records, %d encrypted fields\n",
					name, modelPlan.RecordCount, len(modelPlan.EncryptedFields))
			}

			// Confirm if not in dry run mode
			if !dryRun {
				fmt.Println("\nWARNING: This operation will re-encrypt all sensitive data with a new key.")
				fmt.Println("Make sure the application is stopped before proceeding.")
				fmt.Print("\nDo you want to continue? [y/N]: ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("Operation cancelled.")
					return
				}
			}

			// Perform key rotation
			fmt.Println("\nStarting key rotation...")
			startTime := time.Now()
			stats, err := rotationUtil.RotateKeysForModels(context.Background(), models)
			if err != nil {
				fmt.Println("\nKey rotation failed with error:")
				fmt.Printf("Error: %v\n", err)
				
				// Add more detailed error information
				fmt.Println("\nDetailed error information:")
				fmt.Println("===========================")
				
				// Unwrap nested errors if possible
				var currentErr error = err
				depth := 1
				for currentErr != nil {
					fmt.Printf("%d. %v\n", depth, currentErr)
					if unwrapped, ok := currentErr.(interface{ Unwrap() error }); ok {
						currentErr = unwrapped.Unwrap()
						depth++
					} else {
						break
					}
				}
				
				// Print rotation configuration details
				fmt.Println("\nRotation configuration:")
				fmt.Printf("- Dry run: %v\n", dryRun)
				fmt.Printf("- Batch size: %d\n", batchSize)
				fmt.Printf("- Max errors: %d\n", maxErrors)
				fmt.Printf("- Models: %s\n", modelsFlag)
				fmt.Printf("- Old key env var: %s\n", oldKeyEnvVar)
				
				log.Fatalf("Key rotation failed. See details above.")
			}
			duration := time.Since(startTime).Round(time.Second)

			// Print results
			fmt.Println("\nKey rotation completed successfully!")
			fmt.Printf("Total records processed: %d/%d\n", stats.ProcessedRecords, stats.TotalRecords)
			fmt.Printf("Failed records: %d\n", stats.FailedRecords)
			fmt.Printf("Duration: %s\n", duration)

			if len(stats.Errors) > 0 {
				fmt.Printf("\nErrors (%d):\n", len(stats.Errors))
				for i, err := range stats.Errors {
					if i >= 10 {
						fmt.Printf("... and %d more errors\n", len(stats.Errors)-10)
						break
					}
					fmt.Printf("- %s\n", err)
				}
			}

			// Print next steps
			if !dryRun {
				fmt.Println("\nNext steps:")
				fmt.Println("1. Update your environment variable or .env file with the new encryption key:")
				fmt.Printf("   %s=%s\n", oldKeyEnvVar, base64.StdEncoding.EncodeToString(newKey))
				fmt.Println("2. Restart your GoMFT application")
				fmt.Println("\nIMPORTANT: Keep a backup of both the old and new keys until you verify everything works correctly.")
			} else {
				fmt.Println("\nDry run completed. No changes were made to the database.")
				fmt.Println("Run without --dry-run to perform the actual key rotation.")
			}
		},
	}

	// Add flags
	rotateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate key rotation without making changes")
	rotateCmd.Flags().IntVar(&batchSize, "batch-size", 100, "Number of records to process in each batch")
	rotateCmd.Flags().IntVar(&maxErrors, "max-errors", 50, "Maximum number of errors before aborting")
	rotateCmd.Flags().StringVar(&backupDir, "backup-dir", "", "Directory to store backup data (defaults to config backup_dir)")
	rotateCmd.Flags().BoolVar(&skipBackup, "skip-backup", false, "Skip database backup (not recommended)")
	rotateCmd.Flags().StringVar(&oldKeyEnvVar, "old-key-env", "", "Environment variable containing the old encryption key (defaults to GOMFT_ENCRYPTION_KEY)")
	rotateCmd.Flags().StringVar(&modelsFlag, "models", "auto", "Models to process (use 'auto' for automatic detection)")

	return rotateCmd
}

// keyManager is a simple implementation of the encryption.KeyManager interface
// that uses a fixed key for the new encryption service
type keyManager struct {
	key []byte
}

func (km *keyManager) Initialize() error {
	// Already initialized with the key
	return nil
}

func (km *keyManager) GetPrimaryKey() ([]byte, error) {
	return km.key, nil
}

func (km *keyManager) GetEnvironmentVariableName() string {
	return "TEMP_KEY_MANAGER"
}

func (km *keyManager) StoreKeyEnvironment(key []byte) error {
	// Not needed for this implementation
	return nil
}

// Helper functions

// generateSecureKey creates a cryptographically secure random key encoded as base64
func generateSecureKey() (string, error) {
	return config.GenerateSecureKey()
}

// updateEnvVar updates an environment variable in the .env file content
func updateEnvVar(content, key, value string) (string, bool) {
	lines := strings.Split(content, "\n")
	prefix := key + "="
	updated := false

	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + value
			updated = true
			break
		}
	}

	return strings.Join(lines, "\n"), updated
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
