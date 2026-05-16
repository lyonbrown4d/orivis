package probe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestHTTPProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := New().Check(context.Background(), protocol.AgentTask{
		Type:            string(model.MonitorHTTP),
		Target:          server.URL,
		TimeoutSeconds:  2,
		IntervalSeconds: 1,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected HTTP probe up, got %#v", result)
	}
	if result.Latency <= 0 {
		t.Fatalf("expected positive latency, got %s", result.Latency)
	}
}

func TestTCPProbe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	result := New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorTCP),
		Target:         listener.Addr().String(),
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected TCP probe up, got %#v", result)
	}
}

func TestDNSProbe(t *testing.T) {
	result := New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorDNS),
		Target:         "localhost",
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected DNS probe up, got %#v", result)
	}
}

func TestUnsupportedProbe(t *testing.T) {
	result := New().Check(context.Background(), protocol.AgentTask{
		Type:   "redis",
		Target: "localhost:6379",
	})

	if result.Status != model.StatusUnknown {
		t.Fatalf("expected unsupported probe unknown, got %#v", result)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected unsupported probe error message")
	}
}
