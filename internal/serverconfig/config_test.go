package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	unset(t, "ORIVIS_APP__ENV", "ORIVIS_HTTP__ADDR", "ORIVIS_LOG__LEVEL", "ORIVIS_DB__DRIVER", "ORIVIS_DB__DSN")

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
	if cfg.DB.Driver != "memory" {
		t.Fatalf("expected default DB driver, got %q", cfg.DB.Driver)
	}
	if cfg.DB.DSN != "" {
		t.Fatalf("expected default DB DSN, got %q", cfg.DB.DSN)
	}
	if cfg.DB.ResultRetention != "24h" {
		t.Fatalf("expected default memory result retention, got %q", cfg.DB.ResultRetention)
	}
	if cfg.DB.CleanupInterval != "1m" {
		t.Fatalf("expected default memory cleanup interval, got %q", cfg.DB.CleanupInterval)
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
	t.Setenv("ORIVIS_APP__ENV", "test")
	t.Setenv("ORIVIS_HTTP__ADDR", ":9090")
	t.Setenv("ORIVIS_LOG__LEVEL", "debug")
	t.Setenv("ORIVIS_DB__DRIVER", "postgres")
	t.Setenv("ORIVIS_DB__DSN", "postgres://example")
	t.Setenv("ORIVIS_DB__RESULTRETENTION", "12h")
	t.Setenv("ORIVIS_DB__CLEANUPINTERVAL", "30s")

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
	if cfg.DB.ResultRetention != "12h" {
		t.Fatalf("expected memory result retention from environment, got %q", cfg.DB.ResultRetention)
	}
	if cfg.DB.CleanupInterval != "30s" {
		t.Fatalf("expected memory cleanup interval from environment, got %q", cfg.DB.CleanupInterval)
	}
}
