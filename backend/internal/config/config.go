// Package config loads environment-based configuration once at startup.
package config

import (
	"log/slog"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	HTTPAddr string `envconfig:"HTTP_ADDR" default:":8080"`
	TCPAddr  string `envconfig:"TCP_ADDR"  default:":5023"`

	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

	FirebaseCredentialsPath string `envconfig:"FIREBASE_CREDENTIALS_PATH" required:"true"`

	AppJWTSecret string        `envconfig:"APP_JWT_SECRET" required:"true"`
	AppJWTTTL    time.Duration `envconfig:"APP_JWT_TTL"    default:"24h"`

	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}

// SlogLevel converts the LogLevel string to a slog.Level, defaulting to Info.
func (c Config) SlogLevel() slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(c.LogLevel))); err != nil {
		return slog.LevelInfo
	}
	return l
}

// Load reads environment variables and returns a populated Config.
func Load() (Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return Config{}, err
	}
	return c, nil
}
