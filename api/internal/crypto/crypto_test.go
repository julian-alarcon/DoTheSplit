package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return k
}

func TestPasswordRoundTrip(t *testing.T) {
	pepper := randomKey(t)
	hash, err := HashPassword("correct horse battery staple", pepper)
	require.NoError(t, err)

	ok, err := VerifyPassword(hash, "correct horse battery staple", pepper)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = VerifyPassword(hash, "wrong password", pepper)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestEmailHashDeterministic(t *testing.T) {
	c, err := NewEmailCipher(randomKey(t), randomKey(t))
	require.NoError(t, err)
	require.True(t, bytes.Equal(
		c.HashEmail("Alice@Example.com "),
		c.HashEmail("alice@example.com"),
	))
}

func TestEmailEncryptRoundTrip(t *testing.T) {
	c, err := NewEmailCipher(randomKey(t), randomKey(t))
	require.NoError(t, err)
	blob, err := c.Encrypt("alice@example.com")
	require.NoError(t, err)
	got, err := c.Decrypt(blob)
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", got)
}
