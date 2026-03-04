package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// Provider encrypts and decrypts data (e.g. envelope encryption for DB columns).
type Provider interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

const gcmNonceSize = 12

// AESGCMProvider uses AES-256-GCM with a 32-byte key. Ciphertext format: nonce (12 bytes) || sealed.
type AESGCMProvider struct {
	aead cipher.AEAD
}

// NewAESGCMProvider creates a provider from a 64-character hex string (32 bytes). Returns an error if key is invalid.
func NewAESGCMProvider(hexKey string) (*AESGCMProvider, error) {
	if len(hexKey) != 64 {
		return nil, errors.New("kms: key must be 64 hex characters (32 bytes)")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("kms: key must be 32 bytes after decode")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCMProvider{aead: aead}, nil
}

// Encrypt generates a random 12-byte nonce, encrypts plaintext with AES-GCM, and prepends the nonce to the result.
func (p *AESGCMProvider) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := p.aead.Seal(nil, nonce, plaintext, nil)
	return append(nonce, sealed...), nil
}

// Decrypt extracts the 12-byte nonce from the start of ciphertext and decrypts the remainder.
func (p *AESGCMProvider) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < gcmNonceSize {
		return nil, errors.New("kms: ciphertext too short")
	}
	nonce := ciphertext[:gcmNonceSize]
	payload := ciphertext[gcmNonceSize:]
	return p.aead.Open(nil, nonce, payload, nil)
}
