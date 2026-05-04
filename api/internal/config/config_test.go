package config

import (
	"encoding/base64"
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
