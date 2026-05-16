package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/discovery"
)

func TestStaticDiscoverer(t *testing.T) {
	enabled := true
	discoverer := discovery.NewStaticDiscoverer([]discovery.StaticMonitor{
		{
			Name:              "server health",
			Type:              "http",
			Target:            "http://127.0.0.1:8080/healthz",
			EnvironmentCode:   "dev",
			Enabled:           &enabled,
			Interval:          10 * time.Second,
			Timeout:           2 * time.Second,
			RetryCount:        1,
			AggregationPolicy: "majority_down",
		},
	})

	monitors, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover static monitors: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected one static monitor, got %#v", monitors)
	}
	if monitors[0].SourceKey != "static:server health" {
		t.Fatalf("unexpected source key: %#v", monitors[0])
	}
	if monitors[0].IntervalSeconds != 10 || monitors[0].TimeoutSeconds != 2 {
		t.Fatalf("unexpected monitor timings: %#v", monitors[0])
	}
}

func TestStaticDiscovererRequiresName(t *testing.T) {
	discoverer := discovery.NewStaticDiscoverer([]discovery.StaticMonitor{{Type: "http", Target: "http://127.0.0.1:8080/healthz"}})
	_, err := discoverer.Discover(context.Background())
	if err == nil {
		t.Fatal("expected static monitor name error")
	}
}
