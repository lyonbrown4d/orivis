package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arcgolabs/configx"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
)

func TestLoadDefaults(t *testing.T) {
	isolateOrivisEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected defaults to load: %v", err)
	}
	assertDefaultConfig(t, cfg)
}

func assertDefaultConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.Server.URL != "http://127.0.0.1:8080" {
		t.Fatalf("expected default server URL, got %q", cfg.Server.URL)
	}
	if cfg.Agent.Name != "local-agent" || cfg.Agent.Region != "local" {
		t.Fatalf("unexpected default agent config: %#v", cfg.Agent)
	}
	if cfg.Runtime != "host" {
		t.Fatalf("expected default runtime, got %q", cfg.Runtime)
	}
	if cfg.Poll.Interval != 30*time.Second {
		t.Fatalf("expected default poll interval, got %s", cfg.Poll.Interval)
	}
	assertDefaultBufferConfig(t, cfg)
	if cfg.Discovery.Docker.Enabled || cfg.Discovery.Docker.Mode != "container" {
		t.Fatalf("unexpected Docker discovery defaults: %#v", cfg.Discovery.Docker)
	}
	if !cfg.Discovery.Static.Enabled || len(cfg.Discovery.Static.Monitors) != 0 {
		t.Fatalf("unexpected static discovery defaults: %#v", cfg.Discovery.Static)
	}
}

func assertDefaultBufferConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if !cfg.Buffer.Enabled || cfg.Buffer.Capacity != 1024 {
		t.Fatalf("unexpected buffer defaults: %#v", cfg.Buffer)
	}
}

func isolateOrivisEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"ORIVIS_SERVER__URL",
		"ORIVIS_AGENT__NAME",
		"ORIVIS_AGENT__TOKEN",
		"ORIVIS_AGENT__REGION",
		"ORIVIS_AGENT__ENVIRONMENTS",
		"ORIVIS_RUNTIME",
		"ORIVIS_POLL__INTERVAL",
		"ORIVIS_POLL__JITTER",
		"ORIVIS_BUFFER__ENABLED",
		"ORIVIS_BUFFER__CAPACITY",
		"ORIVIS_LOG__LEVEL",
		"ORIVIS_DISCOVERY__DOCKER__ENABLED",
		"ORIVIS_DISCOVERY__DOCKER__MODE",
		"ORIVIS_DISCOVERY__STATIC__ENABLED",
		"ORIVIS_DISCOVERY__STATIC__HCL_FILES",
		"ORIVIS_DISCOVERY__STATIC__MONITORS",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__SOURCE_KEY",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__NAME",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__TYPE",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__TARGET",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__GROUP",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__ENVIRONMENT",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__ENABLED",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__INTERVAL",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__TIMEOUT",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__RETRY_COUNT",
		"ORIVIS_DISCOVERY__STATIC__MONITOR__AGGREGATION",
	}
	for _, key := range keys {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
}

func TestLoadPollInterval(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_POLL__INTERVAL", "5s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if cfg.Poll.Interval != 5*time.Second {
		t.Fatalf("expected poll interval from environment, got %s", cfg.Poll.Interval)
	}
}

func TestLoadAgentEnvironments(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_AGENT__ENVIRONMENTS", "prod,staging")

	cfg, err := config.Load()
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
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_DISCOVERY__DOCKER__ENABLED", "true")
	t.Setenv("ORIVIS_DISCOVERY__DOCKER__MODE", "swarm")

	cfg, err := config.Load()
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

func TestLoadStaticDiscoveryMonitorFromEnvironment(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_DISCOVERY__STATIC__MONITOR__NAME", "server-health")
	t.Setenv("ORIVIS_DISCOVERY__STATIC__MONITOR__TYPE", "http")
	t.Setenv("ORIVIS_DISCOVERY__STATIC__MONITOR__TARGET", "http://127.0.0.1:8080/healthz")
	t.Setenv("ORIVIS_DISCOVERY__STATIC__MONITOR__ENVIRONMENT", "dev")
	t.Setenv("ORIVIS_DISCOVERY__STATIC__MONITOR__INTERVAL", "15s")
	t.Setenv("ORIVIS_DISCOVERY__STATIC__MONITOR__TIMEOUT", "3s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected environment static monitor to load: %v", err)
	}

	if len(cfg.Discovery.Static.Monitors) != 1 {
		t.Fatalf("expected one static monitor, got %#v", cfg.Discovery.Static.Monitors)
	}
	monitor := cfg.Discovery.Static.Monitors[0]
	if monitor.Name != "server-health" || monitor.Type != "http" || monitor.Target != "http://127.0.0.1:8080/healthz" {
		t.Fatalf("unexpected static monitor: %#v", monitor)
	}
	if monitor.Interval != 15*time.Second || monitor.Timeout != 3*time.Second {
		t.Fatalf("unexpected static monitor timing: %#v", monitor)
	}
}

func TestLoadStaticDiscoveryFromFile(t *testing.T) {
	isolateOrivisEnv(t)
	path := filepath.Join(t.TempDir(), "agent.yaml")
	if err := os.WriteFile(path, []byte(staticDiscoveryConfigYAML), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := config.Load(configx.WithFiles(path))
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

const staticDiscoveryConfigYAML = `
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
`
