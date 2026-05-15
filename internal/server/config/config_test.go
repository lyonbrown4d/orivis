package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	unset(t, "ORIVIS_APP_ENV", "ORIVIS_HTTP_ADDR", "ORIVIS_LOG_LEVEL", "ORIVIS_DB_DRIVER", "ORIVIS_DB_DSN")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected defaults to load: %v", err)
	}

	if cfg.App.Env != "development" {
		t.Fatalf("expected default environment, got %q", cfg.App.Env)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("expected default HTTP address, got %q", cfg.HTTP.Addr)
	}
	if cfg.DB.Driver != "sqlite" {
		t.Fatalf("expected default DB driver, got %q", cfg.DB.Driver)
	}
	if cfg.DB.DSN != "file:orivis.db" {
		t.Fatalf("expected default DB DSN, got %q", cfg.DB.DSN)
	}
}

func unset(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("ORIVIS_APP_ENV", "test")
	t.Setenv("ORIVIS_HTTP_ADDR", ":9090")
	t.Setenv("ORIVIS_LOG_LEVEL", "debug")
	t.Setenv("ORIVIS_DB_DRIVER", "postgres")
	t.Setenv("ORIVIS_DB_DSN", "postgres://example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if cfg.App.Env != "test" {
		t.Fatalf("expected environment from environment, got %q", cfg.App.Env)
	}
	if cfg.HTTP.Addr != ":9090" {
		t.Fatalf("expected HTTP address from environment, got %q", cfg.HTTP.Addr)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("expected log level from environment, got %q", cfg.Log.Level)
	}
	if cfg.DB.Driver != "postgres" {
		t.Fatalf("expected DB driver from environment, got %q", cfg.DB.Driver)
	}
	if cfg.DB.DSN != "postgres://example" {
		t.Fatalf("expected DB DSN from environment, got %q", cfg.DB.DSN)
	}
}
