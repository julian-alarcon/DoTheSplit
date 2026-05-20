// Package config loads runtime configuration from the environment.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	HTTPAddr    string `envconfig:"API_HTTP_ADDR"        default:":8080"`
	DatabaseURL string `envconfig:"DATABASE_URL"`
	WebOrigin   string `envconfig:"WEB_ORIGIN"           default:"http://localhost:3000"`
	LogLevel    string `envconfig:"LOG_LEVEL"            default:"info"`

	CookieSecure  bool   `envconfig:"COOKIE_SECURE"         default:"false"`
	CookieDomain  string `envconfig:"SESSION_COOKIE_DOMAIN" default:""`
	SessionTTLDay int    `envconfig:"SESSION_TTL_DAYS"      default:"30"`

	// Base64-encoded 32-byte keys.
	EmailEncKey    []byte
	EmailHMACKey   []byte
	PasswordPepper []byte
}

// Load reads config from environment.
//
// Sensitive values (DATABASE_URL, EMAIL_ENC_KEY, EMAIL_HMAC_KEY,
// PASSWORD_PEPPER) support the `*_FILE` convention from the Postgres official
// image: if `FOO_FILE` is set and points to a readable file, its trimmed
// contents are used as the value of `FOO`. The `_FILE` form takes precedence
// when both are set so a Docker secret always wins over an inherited env var.
func Load() (*Config, error) {
	var raw struct {
		HTTPAddr      string `envconfig:"API_HTTP_ADDR"        default:":8080"`
		WebOrigin     string `envconfig:"WEB_ORIGIN"           default:"http://localhost:3000"`
		LogLevel      string `envconfig:"LOG_LEVEL"            default:"info"`
		CookieSecure  bool   `envconfig:"COOKIE_SECURE"         default:"false"`
		CookieDomain  string `envconfig:"SESSION_COOKIE_DOMAIN" default:""`
		SessionTTLDay int    `envconfig:"SESSION_TTL_DAYS"      default:"30"`
	}
	if err := envconfig.Process("", &raw); err != nil {
		return nil, fmt.Errorf("envconfig: %w", err)
	}

	dbURL, err := readSecret("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	encRaw, err := readSecret("EMAIL_ENC_KEY")
	if err != nil {
		return nil, err
	}
	macRaw, err := readSecret("EMAIL_HMAC_KEY")
	if err != nil {
		return nil, err
	}
	pepRaw, err := readSecret("PASSWORD_PEPPER")
	if err != nil {
		return nil, err
	}

	enc, err := decodeKey(encRaw, "EMAIL_ENC_KEY")
	if err != nil {
		return nil, err
	}
	mac, err := decodeKey(macRaw, "EMAIL_HMAC_KEY")
	if err != nil {
		return nil, err
	}
	pep, err := decodeKey(pepRaw, "PASSWORD_PEPPER")
	if err != nil {
		return nil, err
	}
	return &Config{
		HTTPAddr:       raw.HTTPAddr,
		DatabaseURL:    dbURL,
		WebOrigin:      raw.WebOrigin,
		LogLevel:       raw.LogLevel,
		CookieSecure:   raw.CookieSecure,
		CookieDomain:   raw.CookieDomain,
		SessionTTLDay:  raw.SessionTTLDay,
		EmailEncKey:    enc,
		EmailHMACKey:   mac,
		PasswordPepper: pep,
	}, nil
}

// readSecret returns the value of `name`, preferring `name+"_FILE"` when set.
// Trailing newlines from the secret file are trimmed (heredocs and `echo`
// almost always append one).
func readSecret(name string) (string, error) {
	if path := os.Getenv(name + "_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("%s_FILE: %w", name, err)
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	return os.Getenv(name), nil
}

func decodeKey(s, name string) ([]byte, error) {
	if s == "" {
		return nil, fmt.Errorf("%s is empty", name)
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%s: base64 decode: %w", name, err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("%s: must decode to 32 bytes, got %d", name, len(b))
	}
	return b, nil
}

// ErrMissing is returned by callers when a required key is absent.
var ErrMissing = errors.New("missing required config")
