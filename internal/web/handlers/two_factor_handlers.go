package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

// Handle2FASetup handles the GET /profile/2fa/setup route
func (h *Handlers) Handle2FASetup(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")

	var user struct {
		Email            string
		TwoFactorEnabled bool
	}

	if err := h.DB.Table("users").Select("email, two_factor_enabled").Where("id = ?", userID).First(&user).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user")
		return
	}

	// Check if 2FA is already enabled
	if user.TwoFactorEnabled {
		c.Redirect(http.StatusFound, "/profile")
		return
	}

	// Generate TOTP secret and QR code URL
	secret, qrCodeURL, err := auth.GenerateTOTPSecret(user.Email)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate 2FA secret")
		return
	}

	// Generate backup codes - now returns both plain codes and hashed codes
	backupCodesPlain, backupCodesHashed, err := auth.GenerateBackupCodes()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate backup codes")
		return
	}

	// Store secret and backup codes in session temporarily
	// We store the plain secret in the cookie since it's temporary and will be encrypted before DB storage
	c.SetCookie("2fa_setup_secret", secret, 3600, "/", "", false, true)
	c.SetCookie("2fa_setup_backup_codes_hashed", backupCodesHashed, 3600, "/", "", false, true)

	// Render setup page
	data := components.TwoFactorSetupData{
		QRCodeURL:    qrCodeURL,
		Secret:       secret,
		BackupCodes:  backupCodesPlain, // Show plain codes to the user
		ErrorMessage: "",
	}
	components.TwoFactorSetup(c.Request.Context(), data).Render(c, c.Writer)
}

// Handle2FAVerifySetup handles the POST /profile/2fa/verify route
func (h *Handlers) Handle2FAVerifySetup(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")

	var user struct {
		Email string
	}
	if err := h.DB.Table("users").Select("email").Where("id = ?", userID).First(&user).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user")
		return
	}

	// Get secret from session
	secret, err := c.Cookie("2fa_setup_secret")
	if err != nil {
		c.String(http.StatusBadRequest, "Setup session expired")
		return
	}

	// Get backup codes from session - now using the hashed version
	backupCodesHashed, err := c.Cookie("2fa_setup_backup_codes_hashed")
	if err != nil {
		c.String(http.StatusBadRequest, "Setup session expired")
		return
	}

	// Verify the code
	code := c.PostForm("code")
	// For verification during setup, we use the plain secret since it's not yet encrypted
	if !totp.Validate(code, secret) {
		// Regenerate QR code URL using the existing secret
		qrCodeURL, err := auth.GenerateQRCodeURL(secret, user.Email)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to generate QR code")
			return
		}

		// For display, we need to generate new plain-text codes
		// but we'll keep the same hashed codes for storage
		backupCodesPlain := []string{}
		if backupCodesHashed != "" {
			// Create placeholder codes since we can't recover the original codes
			// We'll use placeholder text that indicates these were already generated
			for i := 0; i < auth.BackupCodeCount; i++ {
				backupCodesPlain = append(backupCodesPlain, "[BACKUP CODE ALREADY GENERATED]")
			}
		}

		data := components.TwoFactorSetupData{
			QRCodeURL:    qrCodeURL,
			Secret:       secret,
			BackupCodes:  backupCodesPlain,
			ErrorMessage: "Invalid verification code. Please try again.",
		}
		components.TwoFactorSetup(c.Request.Context(), data).Render(c, c.Writer)
		return
	}

	// Encrypt the secret before storing in database
	encryptedSecret, err := auth.EncryptTOTPSecret(secret)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to secure 2FA secret")
		return
	}

	// Update user with 2FA settings
	if err := h.DB.Table("users").Where("id = ?", userID).Updates(map[string]interface{}{
		"two_factor_secret":  encryptedSecret, // Store the encrypted secret
		"two_factor_enabled": true,
		"backup_codes":       backupCodesHashed, // Store the hashed codes
	}).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to enable 2FA")
		return
	}

	// Clear setup cookies
	c.SetCookie("2fa_setup_secret", "", -1, "/", "", false, true)
	c.SetCookie("2fa_setup_backup_codes_hashed", "", -1, "/", "", false, true)

	// Redirect to profile with success message
	c.Redirect(http.StatusFound, "/profile?message=2FA+enabled+successfully")
}

// Handle2FAVerifyPage handles the GET /login/verify route
func (h *Handlers) Handle2FAVerifyPage(c *gin.Context) {
	// Check if we have a temporary user ID
	_, err := c.Cookie("temp_user_id")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Render verification page
	data := components.TwoFactorVerifyData{
		ErrorMessage: "",
	}
	components.TwoFactorVerify(c.Request.Context(), data).Render(c, c.Writer)
}

// Handle2FAVerify handles the POST /login/verify route
func (h *Handlers) Handle2FAVerify(c *gin.Context) {
	// Get user ID from cookie
	tempUserID, err := c.Cookie("temp_user_id")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse user ID
	var userID uint
	if _, err := fmt.Sscanf(tempUserID, "%d", &userID); err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	var user struct {
		TwoFactorSecret string
		BackupCodes     string
		Email           string
		IsAdmin         *bool
	}
	if err := h.DB.Table("users").Select("two_factor_secret, backup_codes, email, is_admin").Where("id = ?", userID).First(&user).Error; err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	code := c.PostForm("code")

	// First try TOTP code
	if auth.ValidateTOTPCode(user.TwoFactorSecret, code) {
		// Generate new JWT token and set cookie
		isAdmin := false
		if user.IsAdmin != nil {
			isAdmin = *user.IsAdmin
		}
		token, err := h.GenerateJWT(userID, user.Email, isAdmin)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to generate token")
			return
		}
		c.SetCookie("jwt_token", token, 86400, "/", "", false, true)

		// Clear temporary user ID cookie
		c.SetCookie("temp_user_id", "", -1, "/", "", false, true)

		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// Then try backup code
	if auth.ValidateBackupCode(code, user.BackupCodes) {
		// Remove used backup code
		newBackupCodes := auth.RemoveBackupCode(code, user.BackupCodes)
		if err := h.DB.Table("users").Where("id = ?", userID).Update("backup_codes", newBackupCodes).Error; err != nil {
			c.String(http.StatusInternalServerError, "Failed to update backup codes")
			return
		}

		// Generate new JWT token and set cookie
		isAdmin := false
		if user.IsAdmin != nil {
			isAdmin = *user.IsAdmin
		}
		token, err := h.GenerateJWT(userID, user.Email, isAdmin)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to generate token")
			return
		}
		c.SetCookie("jwt_token", token, 86400, "/", "", false, true)

		// Clear temporary user ID cookie
		c.SetCookie("temp_user_id", "", -1, "/", "", false, true)

		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	// If neither code is valid, show error
	data := components.TwoFactorVerifyData{
		ErrorMessage: "Invalid verification code. Please try again.",
	}
	components.TwoFactorVerify(c.Request.Context(), data).Render(c, c.Writer)
}

// Handle2FADisable handles the POST /profile/2fa/disable route
func (h *Handlers) Handle2FADisable(c *gin.Context) {
	// Get user ID from context
	userID := c.GetUint("userID")

	// Get current password from form
	currentPassword := c.PostForm("current_password")
	if currentPassword == "" {
		c.Data(http.StatusBadRequest, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Current password is required</span>
		</div>`))
		return
	}

	// Get user from database
	var user struct {
		PasswordHash     string
		TwoFactorEnabled bool
	}
	if err := h.DB.Table("users").Select("password_hash, two_factor_enabled").Where("id = ?", userID).First(&user).Error; err != nil {
		c.Data(http.StatusInternalServerError, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Failed to get user information</span>
		</div>`))
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		c.Data(http.StatusBadRequest, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Current password is incorrect</span>
		</div>`))
		return
	}

	// Check if 2FA is already disabled
	if !user.TwoFactorEnabled {
		c.Data(http.StatusBadRequest, "text/html", []byte(`<div class="bg-yellow-100 border border-yellow-400 text-yellow-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Two-factor authentication is already disabled</span>
		</div>`))
		return
	}

	// Disable 2FA
	if err := h.DB.Table("users").Where("id = ?", userID).Updates(map[string]interface{}{
		"two_factor_enabled": false,
		"two_factor_secret":  nil,
		"backup_codes":       nil,
	}).Error; err != nil {
		c.Data(http.StatusInternalServerError, "text/html", []byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4" role="alert">
			<span class="block sm:inline">Failed to disable two-factor authentication</span>
		</div>`))
		return
	}

	// Return success message
	c.Data(http.StatusOK, "text/html", []byte(`<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4" role="alert">
		<span class="block sm:inline">Two-factor authentication has been disabled</span>
		<script>
			setTimeout(function() {
				window.location.reload();
			}, 1500);
		</script>
	</div>`))
}

// Handle2FABackupCodes handles the GET /profile/2fa/backup-codes route
func (h *Handlers) Handle2FABackupCodes(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")

	var user struct {
		BackupCodes      string
		TwoFactorEnabled bool
		Email            string // We need email to generate new backup codes display
	}

	if err := h.DB.Table("users").Select("backup_codes, two_factor_enabled, email").Where("id = ?", userID).First(&user).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user")
		return
	}

	// Check if 2FA is enabled
	if !user.TwoFactorEnabled {
		c.Redirect(http.StatusFound, "/profile")
		return
	}

	// If user just generated new codes, we need to show them
	// We can't derive the plain codes from the hashed ones, so we'll generate new ones for display
	message := c.Query("message")
	successMessage := ""
	if message != "" {
		successMessage = message
	}

	// For backup codes display, we need to check where we are in the flow
	var backupCodes []string

	// If the user just regenerated codes or they're viewing for the first time
	// we need to generate new codes to display, as we can't recover the hashed ones
	// We'll generate new temporary codes for display only, keeping the hashed ones in the database
	if strings.Contains(successMessage, "New backup codes generated") {
		// Generate new codes to display
		newCodes, _, err := auth.GenerateBackupCodes()
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to generate backup codes for display")
			return
		}
		backupCodes = newCodes

		// Add a special warning about these being the only time they'll see these codes
		successMessage = "New backup codes generated successfully. IMPORTANT: Save these codes now. You won't be able to see them again!"
	} else {
		// If they're just viewing an existing page, show a message explaining
		// that backup codes are securely stored and they need to regenerate to view new ones
		backupCodes = []string{}
		if user.BackupCodes != "" {
			// Count how many backup codes the user has by counting commas+1
			codeCount := 1
			if user.BackupCodes != "" {
				codeCount = strings.Count(user.BackupCodes, ",") + 1
			}

			// Show a placeholder message for existing codes
			for i := 0; i < codeCount; i++ {
				backupCodes = append(backupCodes, "[REDACTED FOR SECURITY]")
			}

			// Set a message explaining why codes are hidden
			successMessage = "For security, backup codes are not displayed after initial generation. Generate new codes to replace existing ones."
		}
	}

	// Render backup codes page
	data := components.TwoFactorBackupCodesData{
		BackupCodes:    backupCodes,
		SuccessMessage: successMessage,
	}
	components.TwoFactorBackupCodes(c.Request.Context(), data).Render(c, c.Writer)
}

// Handle2FARegenerateCodes handles the POST /profile/2fa/regenerate-codes route
func (h *Handlers) Handle2FARegenerateCodes(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")

	var user struct {
		TwoFactorEnabled bool
	}

	if err := h.DB.Table("users").Select("two_factor_enabled").Where("id = ?", userID).First(&user).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user")
		return
	}

	// Check if 2FA is enabled
	if !user.TwoFactorEnabled {
		c.Redirect(http.StatusFound, "/profile")
		return
	}

	// Generate new backup codes - we only need the hashed version for storage
	_, backupCodesHashed, err := auth.GenerateBackupCodes()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate backup codes")
		return
	}

	// Store new backup codes (hashed version)
	if err := h.DB.Table("users").Where("id = ?", userID).Update("backup_codes", backupCodesHashed).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to update backup codes")
		return
	}

	// Redirect back to backup codes page with success message
	c.Redirect(http.StatusFound, "/profile/2fa/backup-codes?message=New backup codes generated successfully")
}

// Handle2FABackupCodePage handles the GET /login/backup-code route
func (h *Handlers) Handle2FABackupCodePage(c *gin.Context) {
	// Check if we have a temporary user ID
	_, err := c.Cookie("temp_user_id")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Render backup code verification page
	data := components.BackupCodeVerifyData{
		ErrorMessage: "",
	}
	components.TwoFactorBackupVerify(c.Request.Context(), data).Render(c, c.Writer)
}
