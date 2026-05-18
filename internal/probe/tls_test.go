package probe_test

import (
	"context"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/probe"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestTLSProbeRecognized(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorTLS),
		Target:         "tls://127.0.0.1:1?server_name=example.com&degraded_before=24h",
		TimeoutSeconds: 1,
	})

	if result.Status == model.StatusUnknown {
		t.Fatalf("expected TLS probe to be recognized, got %#v", result)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected TLS probe error message")
	}
}

func TestTLSProbeInvalidDegradedThreshold(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:   string(model.MonitorTLS),
		Target: "tls://example.com?degraded_before=invalid",
	})

	if result.Status != model.StatusDown {
		t.Fatalf("expected invalid TLS threshold down, got %#v", result)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected invalid TLS threshold error message")
	}
}
