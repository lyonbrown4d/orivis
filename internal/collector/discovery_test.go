package collector_test

import (
	"context"
	"log/slog"
	"testing"

	agentconfig "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/collector"
	"github.com/lyonbrown4d/orivis/internal/discovery"
)

func TestDeduplicateMonitorsFromStaticDiscovery(t *testing.T) {
	enabled := true
	cfg := agentconfig.Config{
		Discovery: struct {
			Provider string `mapstructure:"provider"`
			Static   struct {
				Monitor  discovery.StaticMonitor   `mapstructure:"monitor"`
				Enabled  bool                      `mapstructure:"enabled"`
				HCLFiles []string                  `mapstructure:"hcl_files"`
				Monitors []discovery.StaticMonitor `mapstructure:"monitors"`
			} `mapstructure:"static"`
			Docker struct {
				Enabled bool   `mapstructure:"enabled"`
				Mode    string `mapstructure:"mode"`
			} `mapstructure:"docker"`
			Kubernetes struct {
				Enabled    bool     `mapstructure:"enabled"`
				Mode       string   `mapstructure:"mode"`
				Namespace  string   `mapstructure:"namespace"`
				Namespaces []string `mapstructure:"namespaces"`
				Kubeconfig string   `mapstructure:"kubeconfig"`
			} `mapstructure:"kubernetes"`
		}{
			Static: struct {
				Monitor  discovery.StaticMonitor   `mapstructure:"monitor"`
				Enabled  bool                      `mapstructure:"enabled"`
				HCLFiles []string                  `mapstructure:"hcl_files"`
				Monitors []discovery.StaticMonitor `mapstructure:"monitors"`
			}{
				Enabled: true,
				Monitors: []discovery.StaticMonitor{
					{
						SourceKey: "docker:container:web",
						Name:      "web",
						Type:      "http",
						Target:    "http://localhost:8080",
						Enabled:   &enabled,
					},
					{
						SourceKey: "docker:container:web",
						Name:      "web",
						Type:      "http",
						Target:    "http://localhost:8080",
						Enabled:   &enabled,
					},
					{
						SourceKey: "docker:container:worker",
						Name:      "worker",
						Type:      "tcp",
						Target:    "tcp://localhost:1234",
					},
				},
			},
		},
	}

	discoverer, err := collector.NewMonitorDiscoverer(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("new monitor discoverer: %v", err)
	}

	monitors, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if len(monitors) != 2 {
		t.Fatalf("expected deduplicated monitor count 2, got %d", len(monitors))
	}
}
