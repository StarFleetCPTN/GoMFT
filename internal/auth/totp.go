package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"strings"

	// "github.com/pquerna/otp/base32"
	"github.com/pquerna/otp/totp"
	"github.com/starfleetcptn/gomft/internal/config"
	"golang.org/x/crypto/bcrypt"
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

// EncryptTOTPSecret encrypts the TOTP secret with AES-256-GCM
func EncryptTOTPSecret(secret string) (string, error) {
	// Get encryption key from config or environment variable
	var key []byte
	appConfig, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %v", err)
	}

	// Use the configured encryption key
	key = []byte(appConfig.TOTPEncryptKey)

	// If empty for some reason, log warning and use development key
	if len(key) == 0 {
		fmt.Println("WARNING: Using development encryption key for TOTP. Set TOTP_ENCRYPTION_KEY for production.")
		key = []byte("this-is-a-dev-key-not-for-production!")
	}

	// Ensure key is exactly 32 bytes (AES-256)
	if len(key) < 32 {
		// If key is too short, pad it to 32 bytes
		paddedKey := make([]byte, 32)
		copy(paddedKey, key)
		for i := len(key); i < 32; i++ {
			paddedKey[i] = byte(i % 256) // Simple padding pattern
		}
		key = paddedKey
		fmt.Println("WARNING: TOTP encryption key was padded to 32 bytes. This is insecure.")
	} else if len(key) > 32 {
		// If key is too long, truncate to 32 bytes
		key = key[:32]
		fmt.Println("WARNING: TOTP encryption key was truncated to 32 bytes.")
	}

	// Create a new cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Create a nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Encrypt the data
	ciphertext := aesGCM.Seal(nil, nonce, []byte(secret), nil)

	// Combine nonce and ciphertext and encode as base64
	result := base64.StdEncoding.EncodeToString(append(nonce, ciphertext...))
	return result, nil
}

// DecryptTOTPSecret decrypts the TOTP secret with AES-256-GCM
func DecryptTOTPSecret(encryptedSecret string) (string, error) {
	// Get encryption key from config or environment variable
	var key []byte
	appConfig, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %v", err)
	}

	// Use the configured encryption key
	key = []byte(appConfig.TOTPEncryptKey)

	// If empty for some reason, log warning and use development key
	if len(key) == 0 {
		fmt.Println("WARNING: Using development encryption key for TOTP. Set TOTP_ENCRYPTION_KEY for production.")
		key = []byte("this-is-a-dev-key-not-for-production!")
	}

	// Ensure key is exactly 32 bytes (AES-256)
	if len(key) < 32 {
		// If key is too short, pad it to 32 bytes
		paddedKey := make([]byte, 32)
		copy(paddedKey, key)
		for i := len(key); i < 32; i++ {
			paddedKey[i] = byte(i % 256) // Simple padding pattern
		}
		key = paddedKey
		fmt.Println("WARNING: TOTP encryption key was padded to 32 bytes. This is insecure.")
	} else if len(key) > 32 {
		// If key is too long, truncate to 32 bytes
		key = key[:32]
		fmt.Println("WARNING: TOTP encryption key was truncated to 32 bytes.")
	}

	// Decode the base64 string
	decoded, err := base64.StdEncoding.DecodeString(encryptedSecret)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 secret: %v", err)
	}

	// Create a new cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Get the nonce size
	nonceSize := aesGCM.NonceSize()
	if len(decoded) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := decoded[:nonceSize], decoded[nonceSize:]

	// Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt data: %v", err)
	}

	return string(plaintext), nil
}

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

// ValidateTOTPCode validates a TOTP code against an encrypted secret
func ValidateTOTPCode(encryptedSecret string, code string) bool {
	// Remove any spaces from the code
	code = strings.ReplaceAll(code, " ", "")

	// Decrypt the secret
	secret, err := DecryptTOTPSecret(encryptedSecret)
	if err != nil {
		// Log the error but fail silently to the user
		fmt.Printf("Error decrypting TOTP secret: %v\n", err)
		return false
	}

	// Use the library's Validate function
	return totp.Validate(code, secret)
}

// BackupCodePair represents a backup code and its hash
type BackupCodePair struct {
	PlainCode  string
	HashedCode string
}

// GenerateBackupCodes generates a set of backup codes
// Returns both plaintext codes (to show to user) and hashed codes (to store in DB)
func GenerateBackupCodes() ([]string, string, error) {
	plainCodes := make([]string, BackupCodeCount)
	hashedCodes := make([]string, BackupCodeCount)

	for i := 0; i < BackupCodeCount; i++ {
		// Generate random bytes
		bytes := make([]byte, BackupCodeLength/2)
		_, err := rand.Read(bytes)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate backup code: %v", err)
		}

		// Convert to hex string
		plainCodes[i] = fmt.Sprintf("%x", bytes)

		// Hash the code for storage
		hash, err := bcrypt.GenerateFromPassword([]byte(plainCodes[i]), bcrypt.DefaultCost)
		if err != nil {
			return nil, "", fmt.Errorf("failed to hash backup code: %v", err)
		}

		// Store the hashed version
		hashedCodes[i] = string(hash)
	}

	// Return plaintext codes for display and hashed codes for storage
	return plainCodes, strings.Join(hashedCodes, ","), nil
}

// ValidateBackupCode validates a backup code against a list of hashed codes
func ValidateBackupCode(providedCode string, storedHashedCodes string) bool {
	if storedHashedCodes == "" {
		return false
	}

	// Remove any spaces and convert to lowercase
	providedCode = strings.ToLower(strings.ReplaceAll(providedCode, " ", ""))

	// Split stored hashed codes
	hashedCodes := strings.Split(storedHashedCodes, ",")

	// Check if the provided code matches any stored hashed code
	for _, hashedCode := range hashedCodes {
		if err := bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(providedCode)); err == nil {
			// If the code matches (no error from bcrypt), return true
			return true
		}
	}

	return false
}

// RemoveBackupCode removes a used backup code from the list
func RemoveBackupCode(usedCode string, storedHashedCodes string) string {
	if storedHashedCodes == "" {
		return ""
	}

	usedCode = strings.ToLower(strings.ReplaceAll(usedCode, " ", ""))
	hashedCodes := strings.Split(storedHashedCodes, ",")

	var remainingHashedCodes []string
	for _, hashedCode := range hashedCodes {
		// Only add the code back to the list if it doesn't match the used code
		if err := bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(usedCode)); err != nil {
			// If there's an error, this isn't the used code, so keep it
			remainingHashedCodes = append(remainingHashedCodes, hashedCode)
		}
	}

	return strings.Join(remainingHashedCodes, ",")
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
