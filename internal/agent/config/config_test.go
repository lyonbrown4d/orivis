package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arcgolabs/configx"
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
	if cfg.Discovery.Docker.Enabled {
		t.Fatal("expected Docker discovery to be disabled by default")
	}
	if cfg.Discovery.Docker.Mode != "container" {
		t.Fatalf("expected default Docker discovery mode, got %q", cfg.Discovery.Docker.Mode)
	}
	if !cfg.Discovery.Static.Enabled {
		t.Fatal("expected static discovery to be enabled by default")
	}
	if len(cfg.Discovery.Static.Monitors) != 0 {
		t.Fatalf("expected no default static monitors, got %#v", cfg.Discovery.Static.Monitors)
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

func TestLoadDockerDiscovery(t *testing.T) {
	t.Setenv("ORIVIS_DISCOVERY_DOCKER_ENABLED", "true")
	t.Setenv("ORIVIS_DISCOVERY_DOCKER_MODE", "swarm")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if !cfg.Discovery.Docker.Enabled {
		t.Fatal("expected Docker discovery to be enabled")
	}
	if cfg.Discovery.Docker.Mode != "swarm" {
		t.Fatalf("expected Docker discovery mode from environment, got %q", cfg.Discovery.Docker.Mode)
	}
}

func TestLoadStaticDiscoveryFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.yaml")
	if err := os.WriteFile(path, []byte(`
server:
  url: http://127.0.0.1:8080
agent:
  name: local-agent
  region: local
  environments:
    - dev
runtime: host
poll:
  interval: 5s
log:
  level: info
discovery:
  static:
    enabled: true
    monitors:
      - name: server health
        type: http
        target: http://127.0.0.1:8080/healthz
        environment: dev
        enabled: true
        interval: 10s
        timeout: 2s
        retry_count: 1
        aggregation: majority_down
`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load(configx.WithFiles(path))
	if err != nil {
		t.Fatalf("load config file: %v", err)
	}

	if len(cfg.Discovery.Static.Monitors) != 1 {
		t.Fatalf("expected one static monitor, got %#v", cfg.Discovery.Static.Monitors)
	}
	monitor := cfg.Discovery.Static.Monitors[0]
	if monitor.Name != "server health" || monitor.Type != "http" {
		t.Fatalf("unexpected static monitor identity: %#v", monitor)
	}
	if monitor.Interval != 10*time.Second || monitor.Timeout != 2*time.Second {
		t.Fatalf("unexpected static monitor timing: %#v", monitor)
	}
	if monitor.Enabled == nil || !*monitor.Enabled {
		t.Fatalf("expected static monitor enabled flag, got %#v", monitor.Enabled)
	}
}
