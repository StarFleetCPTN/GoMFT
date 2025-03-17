package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"image/png"
	"strings"

	// "github.com/pquerna/otp/base32"
	"github.com/pquerna/otp/totp"
)

const (
	// IssuerName is the name of the issuer that appears in authenticator apps
	IssuerName = "GoMFT"
	// SecretSize is the size of the TOTP secret in bytes
	SecretSize = 20
	// BackupCodeCount is the number of backup codes to generate
	BackupCodeCount = 8
	// BackupCodeLength is the length of each backup code
	BackupCodeLength = 8
)

// GenerateTOTPSecret generates a new TOTP secret for a user
func GenerateTOTPSecret(email string) (string, string, error) {
	// Generate TOTP key using the library
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      IssuerName,
		AccountName: email,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate TOTP key: %v", err)
	}

	// Generate QR code image
	var buf bytes.Buffer
	img, err := key.Image(256, 256)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate QR code image: %v", err)
	}

	// Encode image as PNG and convert to base64
	err = png.Encode(&buf, img)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode QR code image: %v", err)
	}

	// Create data URL
	dataURL := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes()))

	return key.Secret(), dataURL, nil
}

// ValidateTOTPCode validates a TOTP code against a secret
func ValidateTOTPCode(secret string, code string) bool {
	// Remove any spaces from the code
	code = strings.ReplaceAll(code, " ", "")

	// Use the library's Validate function
	return totp.Validate(code, secret)
}

// GenerateBackupCodes generates a set of backup codes
func GenerateBackupCodes() ([]string, error) {
	codes := make([]string, BackupCodeCount)
	for i := 0; i < BackupCodeCount; i++ {
		// Generate random bytes
		bytes := make([]byte, BackupCodeLength/2)
		_, err := rand.Read(bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %v", err)
		}

		// Convert to hex string
		codes[i] = fmt.Sprintf("%x", bytes)
	}
	return codes, nil
}

// ValidateBackupCode validates a backup code against a list of codes
func ValidateBackupCode(providedCode string, storedCodes string) bool {
	if storedCodes == "" {
		return false
	}

	// Remove any spaces and convert to lowercase
	providedCode = strings.ToLower(strings.ReplaceAll(providedCode, " ", ""))

	// Split stored codes
	codes := strings.Split(storedCodes, ",")

	// Check if the provided code matches any stored code
	for _, code := range codes {
		if code == providedCode {
			return true
		}
	}

	return false
}

// RemoveBackupCode removes a used backup code from the list
func RemoveBackupCode(usedCode string, storedCodes string) string {
	if storedCodes == "" {
		return ""
	}

	usedCode = strings.ToLower(strings.ReplaceAll(usedCode, " ", ""))
	codes := strings.Split(storedCodes, ",")

	var newCodes []string
	for _, code := range codes {
		if code != usedCode {
			newCodes = append(newCodes, code)
		}
	}

	return strings.Join(newCodes, ",")
}

// GenerateQRCodeURL generates a QR code URL for an existing secret
func GenerateQRCodeURL(secret string, email string) (string, error) {
	// Decode the base32 secret
	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret: %v", err)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      IssuerName,
		AccountName: email,
		Secret:      secretBytes,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP key: %v", err)
	}

	// Generate QR code image
	var buf bytes.Buffer
	img, err := key.Image(256, 256)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code image: %v", err)
	}

	// Encode image as PNG and convert to base64
	err = png.Encode(&buf, img)
	if err != nil {
		return "", fmt.Errorf("failed to encode QR code image: %v", err)
	}

	// Create data URL
	dataURL := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes()))

	return dataURL, nil
}
