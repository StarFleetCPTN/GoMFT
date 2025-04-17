package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Standard errors for encryption operations
var (
	ErrEncryptionFailed  = errors.New("encryption failed")
	ErrDecryptionFailed  = errors.New("decryption failed")
	ErrInvalidBlockSize  = errors.New("invalid block size")
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrInvalidKeySize    = errors.New("invalid key size")
	ErrEmptyPlaintext    = errors.New("plaintext is empty")
	ErrEmptyCiphertext   = errors.New("ciphertext is empty")
	ErrMissingIV         = errors.New("initialization vector missing")
)

// EncryptionService provides methods to encrypt and decrypt data
type EncryptionService struct {
	keyManager KeyManager
}

// NewEncryptionService creates a new encryption service using the provided key manager
func NewEncryptionService(km KeyManager) (*EncryptionService, error) {
	if km == nil {
		return nil, errors.New("key manager is required")
	}
	return &EncryptionService{keyManager: km}, nil
}

// Encrypt encrypts the plaintext using AES-256-CBC with PKCS7 padding
// It returns a base64-encoded string of the IV + ciphertext
func (s *EncryptionService) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", ErrEmptyPlaintext
	}

	// Get the encryption key
	key, err := s.keyManager.GetPrimaryKey()
	if err != nil {
		return "", fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Create a new AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	// Pad the plaintext to be a multiple of the block size
	paddedPlaintext := pkcs7Pad(plaintext, block.BlockSize())

	// Generate a random IV
	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(SecureRandomReader, iv); err != nil {
		return "", fmt.Errorf("%w: failed to generate IV: %v", ErrEncryptionFailed, err)
	}

	// Create CBC encrypter
	mode := cipher.NewCBCEncrypter(block, iv)

	// Encrypt the data
	ciphertext := make([]byte, len(paddedPlaintext))
	mode.CryptBlocks(ciphertext, paddedPlaintext)

	// Prepend IV to ciphertext
	combined := append(iv, ciphertext...)

	// Encode with base64
	encoded := base64.StdEncoding.EncodeToString(combined)

	return encoded, nil
}

// Decrypt decrypts the base64-encoded ciphertext using AES-256-CBC with PKCS7 padding
// It expects the ciphertext to be a base64-encoded string of the IV + actual ciphertext
func (s *EncryptionService) Decrypt(encodedCiphertext string) ([]byte, error) {
	if encodedCiphertext == "" {
		return nil, ErrEmptyCiphertext
	}

	// Decode the base64 encoded data
	combined, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64 encoding: %v", ErrDecryptionFailed, err)
	}

	// Get the encryption key
	key, err := s.keyManager.GetPrimaryKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Create a new AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	// Extract IV and ciphertext
	blockSize := block.BlockSize()
	if len(combined) < blockSize {
		return nil, ErrMissingIV
	}
	iv := combined[:blockSize]
	ciphertext := combined[blockSize:]

	// Verify ciphertext length
	if len(ciphertext) == 0 {
		return nil, ErrEmptyCiphertext
	}
	if len(ciphertext)%blockSize != 0 {
		return nil, ErrInvalidBlockSize
	}

	// Create CBC decrypter
	mode := cipher.NewCBCDecrypter(block, iv)

	// Decrypt the data
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	// Remove padding
	unpadded, err := pkcs7Unpad(decrypted, blockSize)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return unpadded, nil
}

// EncryptString encrypts a string and returns a base64-encoded result
func (s *EncryptionService) EncryptString(plaintext string) (string, error) {
	return s.Encrypt([]byte(plaintext))
}

// DecryptString decrypts a base64-encoded ciphertext and returns the plaintext string
func (s *EncryptionService) DecryptString(encodedCiphertext string) (string, error) {
	plaintext, err := s.Decrypt(encodedCiphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// pkcs7Pad adds PKCS#7 padding to the data to make it a multiple of the block size
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS#7 padding from the data
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, ErrInvalidBlockSize
	}

	padding := int(data[len(data)-1])
	if padding <= 0 || padding > blockSize {
		return nil, errors.New("invalid padding value")
	}

	// Validate that all padding bytes have the correct value
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}

// GetGlobalEncryptionService creates an EncryptionService using the global key manager
// It initializes the key manager if it hasn't been initialized yet
func GetGlobalEncryptionService() (*EncryptionService, error) {
	// Make sure key manager is initialized
	if GetKeyManager() == nil {
		if err := InitializeKeyManager(""); err != nil {
			return nil, fmt.Errorf("failed to initialize key manager: %w", err)
		}
	}

	return NewEncryptionService(GetKeyManager())
}
