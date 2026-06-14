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
	WebOrigin   string `envconfig:"WEB_ORIGIN"           default:"http://localhost:8080"`
	LogLevel    string `envconfig:"LOG_LEVEL"            default:"info"`

	CookieSecure  bool   `envconfig:"COOKIE_SECURE"         default:"false"`
	CookieDomain  string `envconfig:"SESSION_COOKIE_DOMAIN" default:""`
	SessionTTLDay int    `envconfig:"SESSION_TTL_DAYS"      default:"30"`

	// AccessTokenTTLMin / RefreshTokenTTLDay govern the bearer-token flow used
	// by the SPA / Capacitor clients. Access tokens are short-lived stateless
	// JWTs; refresh tokens are long-lived, rotating, and revocable.
	AccessTokenTTLMin  int `envconfig:"ACCESS_TOKEN_TTL_MINUTES" default:"15"`
	RefreshTokenTTLDay int `envconfig:"REFRESH_TOKEN_TTL_DAYS"   default:"30"`

	// CapacitorOrigins are the extra CORS origins for native WebView clients
	// (iOS `capacitor://localhost`, Android `https://localhost`). They are
	// merged with WebOrigin into the allowed-origin set.
	CapacitorOrigins []string

	// TrustedProxies is the set of proxy IPs/CIDRs whose X-Forwarded-For we
	// honor. Empty (the default) means trust no proxy: the client IP used for
	// rate limiting and audit logs is the direct connection's RemoteAddr, so a
	// forged X-Forwarded-For can't bypass the limiter or poison the audit log.
	TrustedProxies []string

	// Base64-encoded 32-byte keys.
	EmailEncKey    []byte
	EmailHMACKey   []byte
	PasswordPepper []byte
	// JWTSigningKey signs/verifies bearer access tokens (HMAC-SHA256).
	JWTSigningKey []byte
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
		HTTPAddr           string `envconfig:"API_HTTP_ADDR"            default:":8080"`
		WebOrigin          string `envconfig:"WEB_ORIGIN"               default:"http://localhost:8080"`
		LogLevel           string `envconfig:"LOG_LEVEL"                default:"info"`
		CookieSecure       bool   `envconfig:"COOKIE_SECURE"            default:"false"`
		CookieDomain       string `envconfig:"SESSION_COOKIE_DOMAIN"    default:""`
		SessionTTLDay      int    `envconfig:"SESSION_TTL_DAYS"         default:"30"`
		AccessTokenTTLMin  int    `envconfig:"ACCESS_TOKEN_TTL_MINUTES" default:"15"`
		RefreshTokenTTLDay int    `envconfig:"REFRESH_TOKEN_TTL_DAYS"   default:"30"`
		CapacitorOrigins   string `envconfig:"CAPACITOR_ORIGINS"        default:"capacitor://localhost,https://localhost"`
		TrustedProxies     string `envconfig:"TRUSTED_PROXIES"          default:""`
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
	jwtRaw, err := readSecret("JWT_SIGNING_KEY")
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
	jwt, err := decodeKey(jwtRaw, "JWT_SIGNING_KEY")
	if err != nil {
		return nil, err
	}
	return &Config{
		HTTPAddr:           raw.HTTPAddr,
		DatabaseURL:        dbURL,
		WebOrigin:          raw.WebOrigin,
		LogLevel:           raw.LogLevel,
		CookieSecure:       raw.CookieSecure,
		CookieDomain:       raw.CookieDomain,
		SessionTTLDay:      raw.SessionTTLDay,
		AccessTokenTTLMin:  raw.AccessTokenTTLMin,
		RefreshTokenTTLDay: raw.RefreshTokenTTLDay,
		CapacitorOrigins:   splitList(raw.CapacitorOrigins),
		TrustedProxies:     splitList(raw.TrustedProxies),
		EmailEncKey:        enc,
		EmailHMACKey:       mac,
		PasswordPepper:     pep,
		JWTSigningKey:      jwt,
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

// splitList parses a comma-separated env value into a trimmed, non-empty
// slice. Returns nil for an empty input so callers can treat "unset" and
// "empty list" identically.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ErrMissing is returned by callers when a required key is absent.
var ErrMissing = errors.New("missing required config")
