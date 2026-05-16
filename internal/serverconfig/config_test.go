package config_test

import (
	"os"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestLoadDefaults(t *testing.T) {
	unset(t, "ORIVIS_APP__ENV", "ORIVIS_HTTP__ADDR", "ORIVIS_LOG__LEVEL", "ORIVIS_DB__DRIVER", "ORIVIS_DB__DSN")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected defaults to load: %v", err)
	}
	assertDefaultConfig(t, cfg)
}

func assertDefaultConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.App.Env != "development" {
		t.Fatalf("expected default environment, got %q", cfg.App.Env)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("expected default HTTP address, got %q", cfg.HTTP.Addr)
	}
	if cfg.DB.Driver != "sqlite" || cfg.DB.DSN != config.DefaultSQLiteDSN {
		t.Fatalf("unexpected default DB config: %#v", cfg.DB)
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
	t.Setenv("ORIVIS_DB__DRIVER", "sqlite")
	t.Setenv("ORIVIS_DB__DSN", "file:orivis.db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}
	assertEnvironmentConfig(t, cfg)
}

func assertEnvironmentConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.App.Env != "test" || cfg.HTTP.Addr != ":9090" || cfg.Log.Level != "debug" {
		t.Fatalf("unexpected environment app config: %#v", cfg)
	}
	if cfg.DB.Driver != "sqlite" || cfg.DB.DSN != "file:orivis.db" {
		t.Fatalf("unexpected environment DB config: %#v", cfg.DB)
	}
}
