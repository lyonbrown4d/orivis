package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
)

func assertDefaultConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.Server.URL != "" {
		t.Fatalf("expected empty default server URL for mDNS fallback, got %q", cfg.Server.URL)
	}
	assertDefaultServerConfig(t, cfg)
	assertDefaultAgentConfig(t, cfg)
	if cfg.Runtime != "host" {
		t.Fatalf("expected default runtime, got %q", cfg.Runtime)
	}
	if cfg.Poll.Interval != 30*time.Second {
		t.Fatalf("expected default poll interval, got %s", cfg.Poll.Interval)
	}
	if cfg.Poll.Workers <= 0 {
		t.Fatalf("expected default poll workers, got %d", cfg.Poll.Workers)
	}
	assertDefaultBufferConfig(t, cfg)
	assertDefaultTransportConfig(t, cfg)
	assertDefaultDiscoveryConfig(t, cfg)
}

func assertDefaultServerConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.Server.MDNS.Service != "orivis" || cfg.Server.MDNS.Domain != "local." || cfg.Server.MDNS.Timeout != 5*time.Second {
		t.Fatalf("unexpected default server mDNS config: %#v", cfg.Server.MDNS)
	}
}

func assertDefaultAgentConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if !strings.HasPrefix(cfg.Agent.Name, "local-agent@") {
		t.Fatalf("unexpected default agent config: %#v", cfg.Agent)
	}
	if strings.TrimPrefix(cfg.Agent.Name, "local-agent@") == "" {
		t.Fatalf("expected default agent name to include hostname suffix, got %q", cfg.Agent.Name)
	}
	if cfg.Agent.Region != "local" {
		t.Fatalf("unexpected default agent region: %#v", cfg.Agent)
	}
}

func assertDefaultTransportConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.Transport.RequestTimeout != 10*time.Second || cfg.Transport.ResponseHeaderTimeout != 10*time.Second {
		t.Fatalf("unexpected default transport timeouts: %#v", cfg.Transport)
	}
	if cfg.Transport.RetryAttempts != 3 || cfg.Transport.RetryJitterRatio != 0.2 || !cfg.Transport.GzipResults {
		t.Fatalf("unexpected default transport retry config: %#v", cfg.Transport)
	}
}

func assertDefaultDiscoveryConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	if cfg.Discovery.Provider != "" || cfg.Discovery.Docker.Enabled || cfg.Discovery.Docker.Mode != "" {
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
	if cfg.Buffer.Driver != "memory" || cfg.Buffer.Path != "orivis-agent-buffer.jsonl" {
		t.Fatalf("unexpected buffer storage defaults: %#v", cfg.Buffer)
	}
}

func isolateOrivisEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"ORIVIS_SERVER__URL",
		"ORIVIS_SERVER__MDNS__SERVICE",
		"ORIVIS_SERVER__MDNS__DOMAIN",
		"ORIVIS_SERVER__MDNS__TIMEOUT",
		"ORIVIS_SERVER__MDNS__DEFAULTSCHEME",
		"ORIVIS_AGENT__NAME",
		"ORIVIS_AGENT__TOKEN",
		"ORIVIS_AGENT__REGION",
		"ORIVIS_AGENT__ENVIRONMENTS",
		"ORIVIS_RUNTIME",
		"ORIVIS_POLL__INTERVAL",
		"ORIVIS_POLL__JITTER",
		"ORIVIS_POLL__WORKERS",
		"ORIVIS_BUFFER__ENABLED",
		"ORIVIS_BUFFER__DRIVER",
		"ORIVIS_BUFFER__PATH",
		"ORIVIS_BUFFER__CAPACITY",
		"ORIVIS_TRANSPORT__REQUESTTIMEOUT",
		"ORIVIS_TRANSPORT__MAXIDLECONNS",
		"ORIVIS_TRANSPORT__MAXIDLECONNSPERHOST",
		"ORIVIS_TRANSPORT__IDLECONNTIMEOUT",
		"ORIVIS_TRANSPORT__TLSHANDSHAKETIMEOUT",
		"ORIVIS_TRANSPORT__RESPONSEHEADERTIMEOUT",
		"ORIVIS_TRANSPORT__RETRYATTEMPTS",
		"ORIVIS_TRANSPORT__RETRYBASEDELAY",
		"ORIVIS_TRANSPORT__RETRYMAXDELAY",
		"ORIVIS_TRANSPORT__RETRYJITTERRATIO",
		"ORIVIS_TRANSPORT__GZIPRESULTS",
		"ORIVIS_LOG__LEVEL",
		"ORIVIS_DISCOVERY__PROVIDER",
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
