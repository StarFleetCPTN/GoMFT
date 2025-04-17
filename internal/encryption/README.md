# Encryption Module

This module provides secure encryption and decryption functionality for sensitive credential fields in GoMFT using AES-256 encryption.

## Key Management

The key management module handles secure retrieval, validation, and management of encryption keys from environment variables or secure storage.

### Setup

1. Set the environment variable `GOMFT_ENCRYPTION_KEY` with a securely generated key:
   ```sh
   # Generate a secure random key and set it as an environment variable
   GOMFT_ENCRYPTION_KEY=$(go run -e 'import "encoding/base64"; import "crypto/rand"; key := make([]byte, 32); rand.Read(key); fmt.Println(base64.StdEncoding.EncodeToString(key))')
   ```

2. Include this key in your `.env` file (for development only):
   ```
   GOMFT_ENCRYPTION_KEY=your-base64-encoded-key
   ```

### Usage

To initialize the key manager:

```go
import "github.com/starfleetcptn/gomft/internal/encryption"

func init() {
    // Initialize with default environment variable (GOMFT_ENCRYPTION_KEY)
    err := encryption.InitializeKeyManager("")
    if err != nil {
        panic("Failed to initialize encryption key: " + err.Error())
    }
}
```

To get the key manager instance:

```go
keyManager := encryption.GetKeyManager()
```

To generate a new random encryption key:

```go
key, err := encryption.GenerateKey(encryption.AES256KeySize)
if err != nil {
    // handle error
}
```

## Security Considerations

- **Never store encryption keys in the database** or expose them in logs
- The key should be at least 32 bytes (256 bits) for AES-256 encryption
- In production, use secure key management solutions (e.g., HashiCorp Vault, AWS KMS) instead of environment variables
- Rotate keys periodically for enhanced security
- Monitor for any unusual encryption/decryption activity

## Testing

The module includes comprehensive unit tests. Run them with:

```sh
go test -v ./internal/encryption/...
``` 