package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidate_RejectsShortSecret(t *testing.T) {
	c := Config{
		AppJWTSecret: "too-short",
		AppJWTTTL:    time.Hour,
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for short secret")
	}
	if !strings.Contains(err.Error(), "APP_JWT_SECRET") {
		t.Errorf("error should mention APP_JWT_SECRET, got %v", err)
	}
}

func TestValidate_RejectsShortTTL(t *testing.T) {
	c := Config{
		AppJWTSecret: strings.Repeat("x", 32),
		AppJWTTTL:    10 * time.Second,
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for short TTL")
	}
}

func TestValidate_AcceptsGood(t *testing.T) {
	c := Config{
		AppJWTSecret: strings.Repeat("x", 32),
		AppJWTTTL:    24 * time.Hour,
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
