package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
)

// EmailCipher encrypts and decrypts email addresses with versioned AES-GCM keys,
// and computes a deterministic HMAC-SHA256 for lookup.
type EmailCipher struct {
	hmacKey []byte
	// keys[id] = aead; activeID is used for new encryptions.
	keys     map[byte]cipher.AEAD
	activeID byte
}

// NewEmailCipher builds a cipher with a single active key (id=1). Pass additional
// (id,key) pairs via AddKey to support rotation.
func NewEmailCipher(activeKey, hmacKey []byte) (*EmailCipher, error) {
	if len(hmacKey) != 32 {
		return nil, errors.New("hmac key must be 32 bytes")
	}
	aead, err := newAEAD(activeKey)
	if err != nil {
		return nil, err
	}
	return &EmailCipher{
		hmacKey:  hmacKey,
		keys:     map[byte]cipher.AEAD{1: aead},
		activeID: 1,
	}, nil
}

// AddKey registers an additional decrypt key under id. It does not change the active id.
func (c *EmailCipher) AddKey(id byte, key []byte) error {
	if id == 0 {
		return errors.New("id 0 is reserved")
	}
	aead, err := newAEAD(key)
	if err != nil {
		return err
	}
	c.keys[id] = aead
	return nil
}

// HashEmail returns the HMAC-SHA256 of the normalized email. Deterministic.
func (c *EmailCipher) HashEmail(email string) []byte {
	h := hmac.New(sha256.New, c.hmacKey)
	h.Write([]byte(normalizeEmail(email)))
	return h.Sum(nil)
}

// Encrypt produces [key_id(1) || nonce(12) || ciphertext || tag].
func (c *EmailCipher) Encrypt(email string) ([]byte, error) {
	aead := c.keys[c.activeID]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	plaintext := []byte(email)
	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+aead.Overhead())
	out = append(out, c.activeID)
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plaintext, nil), nil
}

// Decrypt reverses Encrypt.
func (c *EmailCipher) Decrypt(blob []byte) (string, error) {
	if len(blob) < 1 {
		return "", errors.New("ciphertext too short")
	}
	keyID := blob[0]
	aead, ok := c.keys[keyID]
	if !ok {
		return "", fmt.Errorf("unknown key id: %d", keyID)
	}
	ns := aead.NonceSize()
	if len(blob) < 1+ns+aead.Overhead() {
		return "", errors.New("ciphertext too short")
	}
	nonce := blob[1 : 1+ns]
	ct := blob[1+ns:]
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("aead open: %w", err)
	}
	return string(pt), nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("aes key must be 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return aead, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
