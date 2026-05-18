package probe_test

import (
	"context"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/probe"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestMongoDBProbeRecognized(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorMongoDB),
		Target:         "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=1000",
		TimeoutSeconds: 1,
	})

	assertRecognizedDownProbe(t, "MongoDB", result)
}

func TestAMQPProbeRecognized(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorAMQP),
		Target:         "amqp://127.0.0.1:1/",
		TimeoutSeconds: 1,
	})

	assertRecognizedDownProbe(t, "AMQP", result)
}

func TestNATSProbeRecognized(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorNATS),
		Target:         "nats://127.0.0.1:1",
		TimeoutSeconds: 1,
	})

	assertRecognizedDownProbe(t, "NATS", result)
}

func assertRecognizedDownProbe(t *testing.T, name string, result probe.Result) {
	t.Helper()
	if result.Status == model.StatusUnknown {
		t.Fatalf("expected %s probe to be recognized, got %#v", name, result)
	}
	if result.ErrorMessage == "" {
		t.Fatalf("expected %s probe error message", name)
	}
}
