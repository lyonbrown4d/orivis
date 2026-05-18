package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/spf13/pflag"
)

func TestLoadHCLConfigFromFile(t *testing.T) {
	cfg := loadHCLConfig(t)

	assertHCLAgentIdentity(t, cfg)
	assertHCLAgentDiscovery(t, cfg)
}

func TestLoadHCLConfigAllowsEnvOverride(t *testing.T) {
	path := writeAgentHCLConfig(t)
	t.Setenv("ORIVIS_SERVER__URL", "http://env-server:8080")

	cfg, err := config.LoadFromFlags(pflag.NewFlagSet("test", pflag.ContinueOnError), path)
	if err != nil {
		t.Fatalf("load HCL config file: %v", err)
	}

	if cfg.Server.URL != "http://env-server:8080" {
		t.Fatalf("expected env override for server URL, got %q", cfg.Server.URL)
	}
}

func loadHCLConfig(t *testing.T) config.Config {
	t.Helper()
	path := writeAgentHCLConfig(t)
	cfg, err := config.LoadFromFlags(pflag.NewFlagSet("test", pflag.ContinueOnError), path)
	if err != nil {
		t.Fatalf("load HCL config file: %v", err)
	}
	return cfg
}

func writeAgentHCLConfig(t *testing.T) string {
	t.Helper()
	isolateOrivisEnv(t)
	path := filepath.Join(t.TempDir(), "agent.hcl")
	if err := os.WriteFile(path, []byte(agentConfigHCL), 0o600); err != nil {
		t.Fatalf("write HCL config file: %v", err)
	}
	return path
}

func assertHCLAgentIdentity(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.Server.URL != "http://server:8080" {
		t.Fatalf("expected HCL server URL, got %q", cfg.Server.URL)
	}
	if cfg.Agent.Name != "hcl-agent" || cfg.Runtime != "docker-compose" {
		t.Fatalf("unexpected HCL agent config: %#v", cfg)
	}
	if cfg.Poll.Interval != 12*time.Second || cfg.Poll.Jitter != time.Second || cfg.Log.Level != "debug" {
		t.Fatalf("unexpected HCL timing/log config: %#v", cfg)
	}
	assertHCLBuffer(t, cfg)
}

func assertHCLBuffer(t *testing.T, cfg config.Config) {
	t.Helper()
	if !cfg.Buffer.Enabled || cfg.Buffer.Capacity != 42 || cfg.Buffer.Driver != "file" || cfg.Buffer.Path != "agent-buffer.jsonl" {
		t.Fatalf("unexpected HCL buffer config: %#v", cfg.Buffer)
	}
}

func assertHCLAgentDiscovery(t *testing.T, cfg config.Config) {
	t.Helper()
	if !cfg.Discovery.Docker.Enabled || cfg.Discovery.Docker.Mode != "container" {
		t.Fatalf("unexpected HCL Docker discovery config: %#v", cfg.Discovery.Docker)
	}
	if len(cfg.Discovery.Static.Monitors) != 2 {
		t.Fatalf("expected two HCL monitors, got %#v", cfg.Discovery.Static.Monitors)
	}
	if cfg.Discovery.Static.Monitors[0].Name != "server-health" {
		t.Fatalf("unexpected first HCL monitor: %#v", cfg.Discovery.Static.Monitors[0])
	}
	if cfg.Discovery.Static.Monitors[1].Name != "redis" {
		t.Fatalf("unexpected second HCL monitor: %#v", cfg.Discovery.Static.Monitors[1])
	}
}

const agentConfigHCL = `
server {
  url = "http://server:8080"
}

agent {
  name = "hcl-agent"
  token = ""
  region = "local"
  environments = ["dev", "staging"]
}

runtime = "docker-compose"

poll {
  interval = "12s"
  jitter = "1s"
}

buffer {
  enabled = true
  driver = "file"
  path = "agent-buffer.jsonl"
  capacity = 42
}

log {
  level = "debug"
}

discovery {
  static {
    enabled = true

    probe "redis" "redis" {
      target = "redis://redis:6379"
      group = "datastores"
      environment = "dev"
      enabled = true
      interval = "10s"
      timeout = "2s"
      retry_count = 1
      aggregation = "majority_down"
    }
  }

  docker {
    enabled = true
    mode = "container"
  }

  probe "http" "server-health" {
    target = "http://server:8080/healthz"
    group = "core"
    environment = "dev"
    enabled = true
    interval = "15s"
    timeout = "3s"
    retry_count = 0
    aggregation = "majority_down"
  }
}
`
