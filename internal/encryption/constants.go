package encryption

// Key size constants
const (
	// AES256KeySize is the key size in bytes for AES-256 encryption (32 bytes = 256 bits)
	AES256KeySize = 32

	// AESBlockSize is the block size for AES encryption
	AESBlockSize = 16

	// DefaultKeyEnvVar is the default environment variable name for the encryption key
	DefaultKeyEnvVar = "GOMFT_ENCRYPTION_KEY"

	// MinKeyLength is the minimum allowed length for encryption keys in bytes
	MinKeyLength = AES256KeySize
)

// Error messages
const (
	ErrKeyTooShort    = "encryption key is too short, must be at least %d bytes"
	ErrKeyNotProvided = "encryption key not provided in environment variable %s"
	ErrInvalidKey     = "provided encryption key is invalid: %s"
)
