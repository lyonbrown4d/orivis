package discovery_test

import (
	"testing"

	"github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestParseLabels(t *testing.T) {
	monitors, err := discovery.ParseLabels(discovery.LabelSource{
		SourceKey: "docker:container:web",
		Labels: map[string]string{
			"orivis.enable":                    "true",
			"orivis.environment":               "prod",
			"orivis.monitor.http.type":         "http",
			"orivis.monitor.http.target":       "http://web:8080/health",
			"orivis.monitor.http.interval":     "15s",
			"orivis.monitor.http.timeout":      "3s",
			"orivis.monitor.http.retry":        "2",
			"orivis.monitor.http.aggregation":  "majority_down",
			"orivis.monitor.http.name":         "web health",
			"orivis.monitor.tcp.type":          "tcp",
			"orivis.monitor.tcp.target":        "web:8080",
			"orivis.monitor.disabled.type":     "http",
			"orivis.monitor.disabled.target":   "http://web:8080/disabled",
			"orivis.monitor.disabled.enabled":  "false",
			"orivis.monitor.invalid_fragment":  "ignored",
			"traefik.http.routers.web.rule":    "Host(`web.example.test`)",
			"orivis.monitor.http.extra.option": "ignored by parser",
		},
	})
	if err != nil {
		t.Fatalf("parse labels: %v", err)
	}
	assertParsedLabelMonitors(t, monitors)
}

func assertParsedLabelMonitors(t *testing.T, monitors []protocol.AgentDiscoveredMonitor) {
	t.Helper()
	if len(monitors) != 3 {
		t.Fatalf("expected three monitors, got %#v", monitors)
	}
	if monitors[1].SourceKey != "docker:container:web:http" {
		t.Fatalf("unexpected source key: %#v", monitors[1])
	}
	if monitors[1].Name != "web health" || monitors[1].EnvironmentCode != "prod" {
		t.Fatalf("unexpected monitor metadata: %#v", monitors[1])
	}
	if monitors[1].IntervalSeconds != 15 || monitors[1].TimeoutSeconds != 3 || monitors[1].RetryCount != 2 {
		t.Fatalf("unexpected monitor timing: %#v", monitors[1])
	}
	if monitors[0].Enabled == nil || *monitors[0].Enabled {
		t.Fatalf("expected disabled monitor to be disabled: %#v", monitors[0])
	}
}

func TestParseLabelsDisabledSource(t *testing.T) {
	monitors, err := discovery.ParseLabels(discovery.LabelSource{
		SourceKey: "docker:container:web",
		Labels: map[string]string{
			"orivis.enable":              "false",
			"orivis.monitor.http.type":   "http",
			"orivis.monitor.http.target": "http://web:8080/health",
		},
	})
	if err != nil {
		t.Fatalf("parse labels: %v", err)
	}
	if len(monitors) != 0 {
		t.Fatalf("expected disabled source to emit no monitors, got %#v", monitors)
	}
}

func TestParseLabelsRequiresTypeAndTarget(t *testing.T) {
	_, err := discovery.ParseLabels(discovery.LabelSource{
		SourceKey: "docker:container:web",
		Labels: map[string]string{
			"orivis.monitor.http.type": "http",
		},
	})
	if err == nil {
		t.Fatal("expected missing target error")
	}
}

func TestParseLabelsInfersSingleMonitorFromDockerMetadata(t *testing.T) {
	monitors, err := discovery.ParseLabels(discovery.LabelSource{
		SourceKey:          "docker:compose:project:redis",
		Labels:             map[string]string{"orivis.enable": "true", "orivis.monitor.type": "redis"},
		DefaultName:        "redis",
		DefaultEnvironment: "project",
		TargetHost:         "redis",
		Ports:              []int{6379},
	})
	if err != nil {
		t.Fatalf("parse inferred labels: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected one inferred monitor, got %#v", monitors)
	}
	monitor := monitors[0]
	if monitor.SourceKey != "docker:compose:project:redis:redis" {
		t.Fatalf("unexpected source key: %#v", monitor)
	}
	if monitor.Name != "redis" || monitor.Type != "redis" || monitor.Target != "redis://redis:6379" {
		t.Fatalf("unexpected inferred monitor: %#v", monitor)
	}
	if monitor.EnvironmentCode != "project" {
		t.Fatalf("unexpected environment: %#v", monitor)
	}
}

func TestParseLabelsInfersTCPMonitorWhenOnlyEnabled(t *testing.T) {
	monitors, err := discovery.ParseLabels(discovery.LabelSource{
		SourceKey:   "docker:container:web",
		Labels:      map[string]string{"orivis.enable": "true"},
		DefaultName: "web",
		TargetHost:  "web",
		Ports:       []int{8080},
	})
	if err != nil {
		t.Fatalf("parse inferred labels: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected one inferred monitor, got %#v", monitors)
	}
	monitor := monitors[0]
	if monitor.Name != "web" || monitor.Type != "tcp" || monitor.Target != "web:8080" {
		t.Fatalf("unexpected inferred monitor: %#v", monitor)
	}
}

func TestParseLabelsDoesNotInferWithoutOrivisLabels(t *testing.T) {
	monitors, err := discovery.ParseLabels(discovery.LabelSource{
		SourceKey:   "docker:container:web",
		Labels:      map[string]string{},
		DefaultName: "web",
		TargetHost:  "web",
		Ports:       []int{8080},
	})
	if err != nil {
		t.Fatalf("parse labels: %v", err)
	}
	if len(monitors) != 0 {
		t.Fatalf("expected no monitors without orivis labels, got %#v", monitors)
	}
}
