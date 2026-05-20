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

func TestLoadPollWorkers(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_POLL__WORKERS", "7")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if cfg.Poll.Workers != 7 {
		t.Fatalf("expected poll workers from environment, got %d", cfg.Poll.Workers)
	}
}

func TestLoadAgentNameAppendsHostnameSuffix(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_AGENT__NAME", "edge-agent")
	t.Setenv("HOSTNAME", "edge-node")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if cfg.Agent.Name != "edge-agent@edge-node" {
		t.Fatalf("expected agent name with hostname suffix, got %q", cfg.Agent.Name)
	}
}

func TestLoadAgentNameKeepsExistingSuffix(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_AGENT__NAME", "edge-agent@pre-set")
	t.Setenv("HOSTNAME", "edge-node")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if cfg.Agent.Name != "edge-agent@pre-set" {
		t.Fatalf("expected existing suffix preserved, got %q", cfg.Agent.Name)
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
	t.Setenv("ORIVIS_DISCOVERY__PROVIDER", "docker")
	t.Setenv("ORIVIS_DISCOVERY__DOCKER__MODE", "container")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected environment config to load: %v", err)
	}

	if cfg.Discovery.Provider != "docker" || !cfg.Discovery.Docker.Enabled {
		t.Fatal("expected Docker discovery to be enabled")
	}
	if cfg.Discovery.Docker.Mode != "container" {
		t.Fatalf("expected Docker discovery mode from environment, got %q", cfg.Discovery.Docker.Mode)
	}
	if cfg.Runtime != "docker" {
		t.Fatalf("expected Docker provider to set runtime, got %q", cfg.Runtime)
	}
}

func TestLoadDockerDiscoveryRequiresMode(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_DISCOVERY__PROVIDER", "docker")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected load to fail when docker mode is missing")
	}
}

func TestLoadLegacyDockerDiscoveryMode(t *testing.T) {
	isolateOrivisEnv(t)
	t.Setenv("ORIVIS_DISCOVERY__DOCKER__ENABLED", "true")
	t.Setenv("ORIVIS_DISCOVERY__DOCKER__MODE", "swarm")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected legacy environment config to load: %v", err)
	}

	if !cfg.Discovery.Docker.Enabled || cfg.Discovery.Docker.Mode != "swarm" {
		t.Fatalf("unexpected legacy Docker discovery config: %#v", cfg.Discovery.Docker)
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
