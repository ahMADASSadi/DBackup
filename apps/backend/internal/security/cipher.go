package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Cipher encrypts secrets stored in the metadata database.
type Cipher struct {
	key []byte
}

// NewCipher requires a 32-byte AES-256 key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes")
	}
	copied := make([]byte, 32)
	copy(copied, key)
	return &Cipher{key: copied}, nil
}

// Encrypt seals plaintext with AES-256-GCM. aad binds the ciphertext to a record.
// An empty plaintext returns an empty string.
func (c *Cipher) Encrypt(aad, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(c.key)
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
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), []byte(aad))
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt opens a value produced by Encrypt.
func (c *Cipher) Decrypt(aad, encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode secret")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("decode secret")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, []byte(aad))
	if err != nil {
		return "", fmt.Errorf("decrypt secret")
	}
	return string(plain), nil
}
