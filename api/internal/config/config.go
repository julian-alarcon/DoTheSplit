// Package config loads runtime configuration from the environment.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	HTTPAddr    string `envconfig:"API_HTTP_ADDR"        default:":8080"`
	DatabaseURL string `envconfig:"DATABASE_URL"         required:"true"`
	WebOrigin   string `envconfig:"WEB_ORIGIN"           default:"http://localhost:3000"`
	LogLevel    string `envconfig:"LOG_LEVEL"            default:"info"`

	CookieSecure  bool   `envconfig:"COOKIE_SECURE"         default:"false"`
	CookieDomain  string `envconfig:"SESSION_COOKIE_DOMAIN" default:""`
	SessionTTLDay int    `envconfig:"SESSION_TTL_DAYS"      default:"30"`

	// Base64-encoded 32-byte keys.
	EmailEncKey    []byte `envconfig:"EMAIL_ENC_KEY"   required:"true"`
	EmailHMACKey   []byte `envconfig:"EMAIL_HMAC_KEY"  required:"true"`
	PasswordPepper []byte `envconfig:"PASSWORD_PEPPER" required:"true"`
}

// Load reads config from environment.
func Load() (*Config, error) {
	var raw struct {
		HTTPAddr       string `envconfig:"API_HTTP_ADDR"        default:":8080"`
		DatabaseURL    string `envconfig:"DATABASE_URL"         required:"true"`
		WebOrigin      string `envconfig:"WEB_ORIGIN"           default:"http://localhost:3000"`
		LogLevel       string `envconfig:"LOG_LEVEL"            default:"info"`
		CookieSecure   bool   `envconfig:"COOKIE_SECURE"         default:"false"`
		CookieDomain   string `envconfig:"SESSION_COOKIE_DOMAIN" default:""`
		SessionTTLDay  int    `envconfig:"SESSION_TTL_DAYS"      default:"30"`
		EmailEncKey    string `envconfig:"EMAIL_ENC_KEY"   required:"true"`
		EmailHMACKey   string `envconfig:"EMAIL_HMAC_KEY"  required:"true"`
		PasswordPepper string `envconfig:"PASSWORD_PEPPER" required:"true"`
	}
	if err := envconfig.Process("", &raw); err != nil {
		return nil, fmt.Errorf("envconfig: %w", err)
	}
	if raw.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	enc, err := decodeKey(raw.EmailEncKey, "EMAIL_ENC_KEY")
	if err != nil {
		return nil, err
	}
	mac, err := decodeKey(raw.EmailHMACKey, "EMAIL_HMAC_KEY")
	if err != nil {
		return nil, err
	}
	pep, err := decodeKey(raw.PasswordPepper, "PASSWORD_PEPPER")
	if err != nil {
		return nil, err
	}
	return &Config{
		HTTPAddr:       raw.HTTPAddr,
		DatabaseURL:    raw.DatabaseURL,
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
