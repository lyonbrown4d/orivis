package config_test

import (
	"os"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestLoadDefaults(t *testing.T) {
	unset(t, "ORIVIS_APP__ENV", "ORIVIS_HTTP__ADDR", "ORIVIS_HTTP__BODYLIMITBYTES", "ORIVIS_MDNS__ENABLED", "ORIVIS_MDNS__SERVICE", "ORIVIS_MDNS__DOMAIN", "ORIVIS_MDNS__INSTANCE", "ORIVIS_MDNS__SCHEME", "ORIVIS_MDNS__PORT", "ORIVIS_LOG__LEVEL", "ORIVIS_DB__DRIVER", "ORIVIS_DB__DSN")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected defaults to load: %v", err)
	}
	assertDefaultConfig(t, cfg)
}

func assertDefaultConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	assertEqual(t, "app env", cfg.App.Env, "development")
	assertEqual(t, "http addr", cfg.HTTP.Addr, ":8080")
	assertEqual(t, "http body limit", cfg.HTTP.BodyLimitBytes, 4*1024*1024)
	assertEqual(t, "mDNS enabled", cfg.MDNS.Enabled, true)
	assertEqual(t, "mDNS service", cfg.MDNS.Service, "orivis")
	assertEqual(t, "mDNS domain", cfg.MDNS.Domain, "local.")
	assertEqual(t, "mDNS port", cfg.MDNS.Port, 0)
	assertEqual(t, "web enabled", cfg.Web.Enabled, false)
	assertEqual(t, "web root", cfg.Web.Root, "web/dist")
	assertEqual(t, "db driver", cfg.DB.Driver, "sqlite")
	assertEqual(t, "db dsn", cfg.DB.DSN, config.DefaultSQLiteDSN)
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
	t.Setenv("ORIVIS_HTTP__BODYLIMITBYTES", "1024")
	t.Setenv("ORIVIS_MDNS__ENABLED", "false")
	t.Setenv("ORIVIS_MDNS__SERVICE", "orivis-test")
	t.Setenv("ORIVIS_MDNS__PORT", "9090")
	t.Setenv("ORIVIS_WEB__ENABLED", "true")
	t.Setenv("ORIVIS_WEB__ROOT", "/app/web")
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
	assertEqual(t, "app env", cfg.App.Env, "test")
	assertEqual(t, "http addr", cfg.HTTP.Addr, ":9090")
	assertEqual(t, "http body limit", cfg.HTTP.BodyLimitBytes, 1024)
	assertEqual(t, "log level", cfg.Log.Level, "debug")
	assertEqual(t, "mDNS enabled", cfg.MDNS.Enabled, false)
	assertEqual(t, "mDNS service", cfg.MDNS.Service, "orivis-test")
	assertEqual(t, "mDNS port", cfg.MDNS.Port, 9090)
	assertEqual(t, "db driver", cfg.DB.Driver, "sqlite")
	assertEqual(t, "db dsn", cfg.DB.DSN, "file:orivis.db")
	assertEqual(t, "web enabled", cfg.Web.Enabled, true)
	assertEqual(t, "web root", cfg.Web.Root, "/app/web")
}

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("unexpected %s: got %v, want %v", name, got, want)
	}
}
