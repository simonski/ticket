package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
)

// encryptionKey returns the 32-byte AES key from the TICKET_ENCRYPTION_KEY
// environment variable. Returns nil if unset (encryption disabled).
func encryptionKey() []byte {
	raw := strings.TrimSpace(os.Getenv("TICKET_ENCRYPTION_KEY"))
	if raw == "" {
		return nil
	}
	// Pad or truncate to 32 bytes for AES-256
	key := make([]byte, 32)
	copy(key, []byte(raw))
	return key
}

// EncryptEmail encrypts an email address using AES-256-GCM.
// If no encryption key is configured, the plaintext is returned as-is.
func EncryptEmail(plaintext string) (string, error) {
	key := encryptionKey()
	if key == nil {
		return plaintext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptEmail decrypts an email address encrypted with EncryptEmail.
// If the value doesn't have the "enc:" prefix, it's returned as plaintext.
func DecryptEmail(stored string) (string, error) {
	if !strings.HasPrefix(stored, "enc:") {
		return stored, nil
	}
	key := encryptionKey()
	if key == nil {
		return "", errors.New("TICKET_ENCRYPTION_KEY required to decrypt email")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, "enc:"))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
