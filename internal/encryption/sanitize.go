package encryption

import (
	"regexp"
	"strings"
)

// SanitizeError sanitizes an error message to remove or mask sensitive data like keys
func SanitizeError(errMsg string) string {
	if errMsg == "" {
		return ""
	}

	// Sanitize any hex keys (likely to be encryption keys)
	hexKeyPattern := regexp.MustCompile(`([0-9a-fA-F]{16,})`)
	errMsg = hexKeyPattern.ReplaceAllStringFunc(errMsg, func(match string) string {
		if len(match) > 8 {
			return match[:4] + "..." + match[len(match)-4:]
		}
		return "****"
	})

	// Sanitize any base64 content that might contain keys or encrypted data
	base64Pattern := regexp.MustCompile(`([A-Za-z0-9+/]{16,}={0,2})`)
	errMsg = base64Pattern.ReplaceAllStringFunc(errMsg, func(match string) string {
		if len(match) > 8 {
			return match[:4] + "..." + match[len(match)-4:]
		}
		return "****"
	})

	// Mask content that appears to be formatted like encryption keys
	keyPattern := regexp.MustCompile(`(?i)key[=:][\s]*["']?([^"'\s]+)["']?`)
	errMsg = keyPattern.ReplaceAllString(errMsg, "key=****")

	// Mask any JWT tokens
	jwtPattern := regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`)
	errMsg = jwtPattern.ReplaceAllString(errMsg, "JWT_TOKEN_REDACTED")

	// Mask content that appears to be passwords or secrets
	secretPattern := regexp.MustCompile(`(?i)(password|secret|token|auth)[=:][\s]*["']?([^"'\s]+)["']?`)
	errMsg = secretPattern.ReplaceAllString(errMsg, "$1=****")

	// Remove any content between our encrypted prefix and the end of the word
	encPrefix := EncryptedPrefix
	if encPrefix != "" {
		errMsg = sanitizeEncryptedValues(errMsg, encPrefix)
	}

	return errMsg
}

// sanitizeEncryptedValues replaces encrypted values with a redacted placeholder
func sanitizeEncryptedValues(input, prefix string) string {
	if prefix == "" {
		return input
	}

	// Find all occurrences of the prefix and replace the entire encrypted value
	parts := strings.Split(input, prefix)
	if len(parts) <= 1 {
		return input
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		// Find the end of the encrypted value (usually a space, comma, period, quote, etc.)
		endIdx := strings.IndexAny(part, " \t\n\r.,;:\"')")
		if endIdx == -1 {
			// If no terminating character, take the whole string
			result += prefix + "****"
		} else {
			// Keep the terminating character
			result += prefix + "****" + part[endIdx:]
		}
	}

	return result
}

// SanitizeCredentialData removes or masks a credential for safe logging
// This is a utility function to use in error messages and logs
func SanitizeCredentialData(value string) string {
	if value == "" {
		return ""
	}

	// If already an encrypted value, return just the prefix and a hint of the actual value
	if strings.HasPrefix(value, EncryptedPrefix) {
		encrypted := strings.TrimPrefix(value, EncryptedPrefix)
		if len(encrypted) > 8 {
			return EncryptedPrefix + encrypted[:4] + "..." + encrypted[len(encrypted)-4:]
		}
		return EncryptedPrefix + "..."
	}

	// For plaintext credentials, just mask the value entirely
	if len(value) > 8 {
		return value[:2] + "..." + value[len(value)-2:]
	}
	return "****"
}
