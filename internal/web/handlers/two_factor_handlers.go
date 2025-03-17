package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

	// Generate backup codes
	backupCodes, err := auth.GenerateBackupCodes()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate backup codes")
		return
	}

	// Store secret and backup codes in session temporarily
	c.SetCookie("2fa_setup_secret", secret, 3600, "/", "", false, true)
	c.SetCookie("2fa_setup_backup_codes", strings.Join(backupCodes, ","), 3600, "/", "", false, true)

	// Render setup page
	data := components.TwoFactorSetupData{
		QRCodeURL:    qrCodeURL,
		Secret:       secret,
		BackupCodes:  backupCodes,
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

	// Get backup codes from session
	backupCodes, err := c.Cookie("2fa_setup_backup_codes")
	if err != nil {
		c.String(http.StatusBadRequest, "Setup session expired")
		return
	}

	// Verify the code
	code := c.PostForm("code")
	if !auth.ValidateTOTPCode(secret, code) {
		// Regenerate QR code URL using the existing secret
		qrCodeURL, err := auth.GenerateQRCodeURL(secret, user.Email)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to generate QR code")
			return
		}

		data := components.TwoFactorSetupData{
			QRCodeURL:    qrCodeURL,
			Secret:       secret,
			BackupCodes:  strings.Split(backupCodes, ","),
			ErrorMessage: "Invalid verification code. Please try again.",
		}
		components.TwoFactorSetup(c.Request.Context(), data).Render(c, c.Writer)
		return
	}

	// Update user with 2FA settings
	if err := h.DB.Table("users").Where("id = ?", userID).Updates(map[string]interface{}{
		"two_factor_secret":  secret,
		"two_factor_enabled": true,
		"backup_codes":       backupCodes,
	}).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to enable 2FA")
		return
	}

	// Clear setup cookies
	c.SetCookie("2fa_setup_secret", "", -1, "/", "", false, true)
	c.SetCookie("2fa_setup_backup_codes", "", -1, "/", "", false, true)

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
		if err := h.DB.Model("users").Where("id = ?", userID).Update("backup_codes", newBackupCodes).Error; err != nil {
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
