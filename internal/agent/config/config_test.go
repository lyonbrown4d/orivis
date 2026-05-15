package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	unset(t, "ORIVIS_SERVER_URL", "ORIVIS_AGENT_NAME", "ORIVIS_AGENT_TOKEN", "ORIVIS_AGENT_REGION", "ORIVIS_AGENT_ENVIRONMENTS", "ORIVIS_RUNTIME", "ORIVIS_POLL_INTERVAL", "ORIVIS_LOG_LEVEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected defaults to load: %v", err)
	}

	if cfg.Server.URL != "http://127.0.0.1:8080" {
		t.Fatalf("expected default server URL, got %q", cfg.Server.URL)
	}
	if cfg.Agent.Name != "local-agent" {
		t.Fatalf("expected default agent name, got %q", cfg.Agent.Name)
	}
	if cfg.Agent.Region != "local" {
		t.Fatalf("expected default region, got %q", cfg.Agent.Region)
	}
	if cfg.Runtime != "host" {
		t.Fatalf("expected default runtime, got %q", cfg.Runtime)
	}
	if cfg.Poll.Interval != 30*time.Second {
		t.Fatalf("expected default poll interval, got %s", cfg.Poll.Interval)
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

func TestLoadPollInterval(t *testing.T) {
	t.Setenv("ORIVIS_POLL_INTERVAL", "5s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if cfg.Poll.Interval != 5*time.Second {
		t.Fatalf("expected poll interval from environment, got %s", cfg.Poll.Interval)
	}
}

func TestLoadAgentEnvironments(t *testing.T) {
	t.Setenv("ORIVIS_AGENT_ENVIRONMENTS", "prod,staging")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if len(cfg.Agent.Environments) != 2 {
		t.Fatalf("expected 2 agent environments, got %#v", cfg.Agent.Environments)
	}
	if cfg.Agent.Environments[0] != "prod" || cfg.Agent.Environments[1] != "staging" {
		t.Fatalf("unexpected agent environments: %#v", cfg.Agent.Environments)
	}
}
