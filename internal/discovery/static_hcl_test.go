package discovery_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/discovery"
)

func TestLoadStaticMonitorsHCL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probes.hcl")
	if err := os.WriteFile(path, []byte(staticMonitorHCL), 0o600); err != nil {
		t.Fatalf("write HCL file: %v", err)
	}

	monitors, err := discovery.LoadStaticMonitorsHCL([]string{path})
	if err != nil {
		t.Fatalf("load static monitors HCL: %v", err)
	}
	if len(monitors) != 2 {
		t.Fatalf("expected two monitors, got %#v", monitors)
	}
	if monitors[0].SourceKey != "static:hcl:http:server-health" {
		t.Fatalf("unexpected default source key: %#v", monitors[0])
	}
	if monitors[0].Interval != 15*time.Second || monitors[0].Timeout != 3*time.Second {
		t.Fatalf("unexpected monitor timing: %#v", monitors[0])
	}
	if monitors[1].SourceKey != "custom:redis" {
		t.Fatalf("unexpected explicit source key: %#v", monitors[1])
	}
}

const staticMonitorHCL = `
probe "http" "server-health" {
  target      = "http://127.0.0.1:8080/healthz"
  group       = "core"
  environment = "dev"
  enabled     = true
  interval    = "15s"
  timeout     = "3s"
  retry_count = 1
  aggregation = "majority_down"
}

probe "redis" "redis-cache" {
  source_key  = "custom:redis"
  target      = "redis://127.0.0.1:6379"
  group       = "datastores"
  environment = "dev"
}
`
