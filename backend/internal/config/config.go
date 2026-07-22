// Package config loads environment-based configuration once at startup.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

const minJWTSecretLen = 32 // bytes of entropy (HMAC-SHA256 floor)

type Config struct {
	HTTPAddr string `envconfig:"HTTP_ADDR" default:":8080"`
	TCPAddr  string `envconfig:"TCP_ADDR"  default:":5023"`

	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

	FirebaseCredentialsPath string `envconfig:"FIREBASE_CREDENTIALS_PATH" required:"true"`

	AppJWTSecret string        `envconfig:"APP_JWT_SECRET" required:"true"`
	AppJWTTTL    time.Duration `envconfig:"APP_JWT_TTL"    default:"24h"`

	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`

	// Optional: when true, mount /metrics. Default off in case the deploy is
	// behind a shared LB and the operator wants to keep metrics private.
	MetricsEnabled bool `envconfig:"METRICS_ENABLED" default:"true"`
}

// SlogLevel converts the LogLevel string to a slog.Level, defaulting to Info.
func (c Config) SlogLevel() slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(c.LogLevel))); err != nil {
		return slog.LevelInfo
	}
	return l
}

// Validate enforces invariants that envconfig can't express via struct tags.
func (c Config) Validate() error {
	if len(c.AppJWTSecret) < minJWTSecretLen {
		return fmt.Errorf(
			"APP_JWT_SECRET must be at least %d characters (got %d). "+
				"Generate one with: openssl rand -base64 32",
			minJWTSecretLen, len(c.AppJWTSecret),
		)
	}
	if c.AppJWTTTL < time.Minute {
		return errors.New("APP_JWT_TTL must be ≥ 1m")
	}
	return nil
}

// Load reads environment variables, validates, and returns a populated Config.
func Load() (Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}
