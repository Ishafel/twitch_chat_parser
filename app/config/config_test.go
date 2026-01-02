package config

import (
	"testing"
	"time"
)

func TestLoadParsesRequiredEnv(t *testing.T) {
	t.Setenv("TWITCH_USERNAME", "bot")
	t.Setenv("TWITCH_OAUTH_TOKEN", "oauth:token")
	t.Setenv("TWITCH_CHANNELS", "#chan1, chan2")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_DB", "db")
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Twitch.Username != "bot" || cfg.Twitch.OAuthToken != "oauth:token" {
		t.Fatalf("unexpected twitch credentials: %+v", cfg.Twitch)
	}

	expectedChannels := []string{"chan1", "chan2"}
	if len(cfg.Twitch.Channels) != len(expectedChannels) {
		t.Fatalf("expected %d channels, got %d", len(expectedChannels), len(cfg.Twitch.Channels))
	}

	for i, ch := range expectedChannels {
		if cfg.Twitch.Channels[i] != ch {
			t.Fatalf("channel %d mismatch: expected %s got %s", i, ch, cfg.Twitch.Channels[i])
		}
	}

	if cfg.Postgres.DSN() != "postgres://user:pass@localhost:5432/db?sslmode=disable" {
		t.Fatalf("unexpected DSN: %s", cfg.Postgres.DSN())
	}

	if cfg.Batch.MaxBatch != 100 || cfg.Batch.FlushEvery != 1500*time.Millisecond {
		t.Fatalf("unexpected batch defaults: %+v", cfg.Batch)
	}
}

func TestLoadValidatesMissingEnv(t *testing.T) {
	if _, err := Load(); err == nil {
		t.Fatalf("expected error when env vars are missing")
	}
}
