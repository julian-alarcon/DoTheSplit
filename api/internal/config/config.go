// Package config loads runtime configuration from the environment.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr string `envconfig:"API_HTTP_ADDR"        default:":8080"`
	// DatabaseDriver selects the persistence engine: "sqlite" (default) or
	// "postgres". For sqlite, DatabaseURL is a file DSN (defaults to
	// file:./dts.db) or ":memory:"; migrations are applied in-process at boot and
	// there is no separate migrate container. Postgres deployments must set
	// DATABASE_DRIVER=postgres and a DATABASE_URL.
	DatabaseDriver string `envconfig:"DATABASE_DRIVER"      default:"sqlite"`
	DatabaseURL    string `envconfig:"DATABASE_URL"`
	// WorkerMode controls the recurring/outbox worker topology: "external"
	// (default) runs it as a separate process/container; "embedded" runs it as
	// a goroutine inside the api binary. SQLite forces "embedded" (single-writer
	// file + in-process realtime hub), regardless of this value.
	WorkerMode string `envconfig:"WORKER_MODE"          default:"external"`
	WebOrigin  string `envconfig:"WEB_ORIGIN"           default:"http://localhost:8080"`
	LogLevel   string `envconfig:"LOG_LEVEL"            default:"info"`

	CookieSecure bool   `envconfig:"COOKIE_SECURE"         default:"false"`
	CookieDomain string `envconfig:"COOKIE_DOMAIN"         default:""`

	// AccessTokenTTLMin / RefreshTokenTTLDay govern the bearer-token flow used
	// by the SPA / Capacitor clients. Access tokens are short-lived stateless
	// JWTs; refresh tokens are long-lived, rotating, and revocable.
	AccessTokenTTLMin  int `envconfig:"ACCESS_TOKEN_TTL_MINUTES" default:"15"`
	RefreshTokenTTLDay int `envconfig:"REFRESH_TOKEN_TTL_DAYS"   default:"30"`

	// AuthRateLimitPerMin caps requests/min/IP on the auth endpoints
	// (/auth/register, /auth/token, /auth/verify*, /auth/password-reset*).
	AuthRateLimitPerMin int `envconfig:"AUTH_RATE_LIMIT_PER_MIN"  default:"10"`

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
	accessTTL, err := envInt("ACCESS_TOKEN_TTL_MINUTES", 15)
	if err != nil {
		return nil, err
	}
	refreshTTL, err := envInt("REFRESH_TOKEN_TTL_DAYS", 30)
	if err != nil {
		return nil, err
	}
	authRate, err := envInt("AUTH_RATE_LIMIT_PER_MIN", 10)
	if err != nil {
		return nil, err
	}
	cookieSecure, err := envBool("COOKIE_SECURE", false)
	if err != nil {
		return nil, err
	}

	driver := strings.ToLower(strings.TrimSpace(envStr("DATABASE_DRIVER", "sqlite")))
	switch driver {
	case "postgres", "sqlite":
	default:
		return nil, fmt.Errorf("DATABASE_DRIVER: must be 'postgres' or 'sqlite', got %q", driver)
	}

	workerMode := strings.ToLower(strings.TrimSpace(envStr("WORKER_MODE", "external")))
	switch workerMode {
	case "external", "embedded":
	default:
		return nil, fmt.Errorf("WORKER_MODE: must be 'external' or 'embedded', got %q", workerMode)
	}
	// SQLite is single-node by construction: a separate worker process can't
	// reach the api's in-memory realtime hub and would contend with it for the
	// single SQLite writer. Force the embedded worker.
	if driver == "sqlite" {
		workerMode = "embedded"
	}

	dbURL, err := readSecret("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	if dbURL == "" {
		// SQLite works out of the box with a local file; Postgres always needs
		// an explicit connection string.
		if driver == "sqlite" {
			dbURL = "file:./dts.db"
		} else {
			return nil, errors.New("DATABASE_URL is required")
		}
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
		HTTPAddr:            envStr("API_HTTP_ADDR", ":8080"),
		DatabaseDriver:      driver,
		DatabaseURL:         dbURL,
		WorkerMode:          workerMode,
		WebOrigin:           envStr("WEB_ORIGIN", "http://localhost:8080"),
		LogLevel:            envStr("LOG_LEVEL", "info"),
		CookieSecure:        cookieSecure,
		CookieDomain:        envStr("COOKIE_DOMAIN", ""),
		AccessTokenTTLMin:   accessTTL,
		RefreshTokenTTLDay:  refreshTTL,
		AuthRateLimitPerMin: authRate,
		CapacitorOrigins:    splitList(envStr("CAPACITOR_ORIGINS", "capacitor://localhost,https://localhost")),
		TrustedProxies:      splitList(envStr("TRUSTED_PROXIES", "")),
		EmailEncKey:         enc,
		EmailHMACKey:        mac,
		PasswordPepper:      pep,
		JWTSigningKey:       jwt,
	}, nil
}

// envStr returns the env var value, or def when unset. (An explicitly empty
// value is returned as-is, matching envconfig's behavior.)
func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// envInt parses an integer env var, falling back to def when unset/empty.
func envInt(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: must be an integer: %w", key, err)
	}
	return n, nil
}

// envBool parses a boolean env var, falling back to def when unset/empty.
// Accepts the same forms as strconv.ParseBool (1/t/true/0/f/false, any case).
func envBool(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: must be a boolean: %w", key, err)
	}
	return b, nil
}

// SlogLevel maps the LOG_LEVEL string to a slog.Level, defaulting to Info for
// empty or unrecognised values.
func (c *Config) SlogLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
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
