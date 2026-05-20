package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// setEnv sets a full, valid set of env vars for Load(). Any overrides passed
// in the map replace (or clear, when empty string) the defaults.
func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	defaults := map[string]string{
		"DATABASE_URL":    "postgres://u:p@localhost/db?sslmode=disable",
		"EMAIL_ENC_KEY":   key,
		"EMAIL_HMAC_KEY":  key,
		"PASSWORD_PEPPER": key,
	}
	for k, v := range overrides {
		defaults[k] = v
	}
	for k, v := range defaults {
		if v == "" {
			t.Setenv(k, "")
		} else {
			t.Setenv(k, v)
		}
	}
}

func TestLoadValid(t *testing.T) {
	setEnv(t, nil)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.HTTPAddr)
	require.Equal(t, 30, cfg.SessionTTLDay)
	require.False(t, cfg.CookieSecure)
	require.Len(t, cfg.EmailEncKey, 32)
	require.Len(t, cfg.EmailHMACKey, 32)
	require.Len(t, cfg.PasswordPepper, 32)
}

func TestLoadMissingDatabaseURL(t *testing.T) {
	setEnv(t, nil)
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoadMissingKey(t *testing.T) {
	setEnv(t, nil)
	t.Setenv("EMAIL_ENC_KEY", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoadRejectsWrongSizedKey(t *testing.T) {
	setEnv(t, map[string]string{
		// 16 random bytes base64 -> decodes to 16 bytes, not 32.
		"EMAIL_ENC_KEY": base64.StdEncoding.EncodeToString(make([]byte, 16)),
	})
	_, err := Load()
	require.Error(t, err)
}

func TestLoadRejectsNonBase64Key(t *testing.T) {
	setEnv(t, map[string]string{"PASSWORD_PEPPER": "not-base64!!"})
	_, err := Load()
	require.Error(t, err)
}

// TestLoadReadsSecretFromFile covers the `*_FILE` Docker-secrets convention:
// when EMAIL_ENC_KEY_FILE points at a readable file, its contents replace the
// EMAIL_ENC_KEY env var (whose value is intentionally cleared here to prove
// the file path is what wins).
func TestLoadReadsSecretFromFile(t *testing.T) {
	setEnv(t, nil)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "enc.key")
	keyB64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	require.NoError(t, os.WriteFile(keyPath, []byte(keyB64+"\n"), 0o600))
	t.Setenv("EMAIL_ENC_KEY", "")
	t.Setenv("EMAIL_ENC_KEY_FILE", keyPath)
	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.EmailEncKey, 32)
}

// TestLoadFilePrefersOverEnv ensures that when both EMAIL_HMAC_KEY and
// EMAIL_HMAC_KEY_FILE are set, the file content wins (Docker secrets must
// override any inherited env var).
func TestLoadFilePrefersOverEnv(t *testing.T) {
	setEnv(t, nil)
	bad := base64.StdEncoding.EncodeToString(make([]byte, 16)) // not 32 bytes
	good := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("EMAIL_HMAC_KEY", bad)
	dir := t.TempDir()
	p := filepath.Join(dir, "hmac.key")
	require.NoError(t, os.WriteFile(p, []byte(good), 0o600))
	t.Setenv("EMAIL_HMAC_KEY_FILE", p)
	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.EmailHMACKey, 32)
}
