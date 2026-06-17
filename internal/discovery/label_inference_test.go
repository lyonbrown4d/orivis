package discovery_test

import (
	"testing"

	"github.com/lyonbrown4d/orivis/internal/discovery"
)

func TestParseLabelsInfersHTTPAndCacheComponentProvidersFromImage(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		ports      []int
		wantType   string
		wantTarget string
	}{
		{name: "grafana", image: "grafana/grafana:13.0.1", ports: []int{3000}, wantType: "http", wantTarget: "http://grafana:3000"},
		{name: "prometheus", image: "prom/prometheus:v3.0.0", ports: []int{9090}, wantType: "http", wantTarget: "http://prometheus:9090"},
		{name: "phpmyadmin", image: "phpmyadmin:5.2", ports: []int{80}, wantType: "http", wantTarget: "http://phpmyadmin:80"},
		{name: "node-exporter", image: "prom/node-exporter:v1.9.1", ports: []int{9100}, wantType: "http", wantTarget: "http://node-exporter:9100"},
		{name: "uptime-kuma", image: "louislam/uptime-kuma:2.2.1-slim", ports: []int{3001}, wantType: "http", wantTarget: "http://uptime-kuma:3001"},
		{name: "memcached", image: "memcached:1.6-alpine", ports: []int{11211}, wantType: "memcached", wantTarget: "memcached://memcached:11211"},
		{name: "dragonfly", image: "docker.dragonflydb.io/dragonflydb/dragonfly:v1.28.0", ports: []int{6379}, wantType: "redis", wantTarget: "redis://dragonfly:6379"},
		{name: "valkey", image: "valkey/valkey:8-alpine", ports: []int{6379}, wantType: "redis", wantTarget: "redis://valkey:6379"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertInferredComponentProvider(t, tt.name, tt.image, tt.ports, tt.wantType, tt.wantTarget)
		})
	}
}

func TestParseLabelsInfersMessagingDatabaseAndTCPComponentProvidersFromImage(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		ports      []int
		wantType   string
		wantTarget string
	}{
		{name: "kafka", image: "bitnami/kafka:3.9.0", ports: []int{29092, 9092}, wantType: "kafka", wantTarget: "kafka:9092"},
		{name: "redpanda", image: "redpandadata/redpanda:v24.3.1", ports: []int{29092}, wantType: "kafka", wantTarget: "redpanda:29092"},
		{name: "nats", image: "nats:2.10-alpine", ports: []int{4222}, wantType: "nats", wantTarget: "nats://nats:4222"},
		{name: "mailpit", image: "axllent/mailpit:v1.22", ports: []int{1025, 8025}, wantType: "smtp", wantTarget: "smtp://mailpit:1025"},
		{name: "percona", image: "percona/percona-server:8.4", ports: []int{3306}, wantType: "mysql", wantTarget: "percona:3306"},
		{name: "timescaledb", image: "timescale/timescaledb:latest-pg17", ports: []int{5432}, wantType: "postgres", wantTarget: "timescaledb:5432"},
		{name: "zookeeper", image: "zookeeper:3.9", ports: []int{2181, 8080}, wantType: "tcp", wantTarget: "zookeeper:2181"},
		{name: "etcd", image: "bitnami/etcd:3.5", ports: []int{2379}, wantType: "tcp", wantTarget: "etcd:2379"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertInferredComponentProvider(t, tt.name, tt.image, tt.ports, tt.wantType, tt.wantTarget)
		})
	}
}

func assertInferredComponentProvider(t *testing.T, name, image string, ports []int, wantType, wantTarget string) {
	t.Helper()

	monitors, err := discovery.ParseLabels(discovery.LabelSource{
		SourceKey:   "docker:container:" + name,
		Labels:      map[string]string{"orivis.enable": "true"},
		DefaultName: name,
		TargetHost:  name,
		ImageName:   image,
		Ports:       ports,
	})
	if err != nil {
		t.Fatalf("parse inferred labels: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected one inferred monitor, got %#v", monitors)
	}
	monitor := monitors[0]
	if monitor.Name != name || monitor.Type != wantType || monitor.Target != wantTarget {
		t.Fatalf("unexpected inferred monitor: %#v", monitor)
	}
}
