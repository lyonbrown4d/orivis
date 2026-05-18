package probe_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/probe"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestHTTPProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := probe.New().Check(context.Background(), protocol.AgentTask{
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
	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		closeTestResource(t, listener)
	})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			closeTestResource(t, conn)
		}
	}()

	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorTCP),
		Target:         listener.Addr().String(),
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected TCP probe up, got %#v", result)
	}
}

func TestDNSProbe(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorDNS),
		Target:         "localhost",
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected DNS probe up, got %#v", result)
	}
}

func TestUDPProbe(t *testing.T) {
	conn, err := new(net.ListenConfig).ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	t.Cleanup(func() {
		closeTestResource(t, conn)
	})
	go func() {
		buffer := make([]byte, 32)
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			return
		}
		if strings.Contains(string(buffer[:n]), "ping") {
			if _, writeErr := conn.WriteTo([]byte("pong"), addr); writeErr != nil {
				return
			}
		}
	}()

	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorUDP),
		Target:         "udp://" + conn.LocalAddr().String() + "?payload=ping&expect=pong",
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected UDP probe up, got %#v", result)
	}
}

func TestSMTPProbe(t *testing.T) {
	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen smtp: %v", err)
	}
	t.Cleanup(func() {
		closeTestResource(t, listener)
	})
	go serveSMTPProbeTestConnection(t, listener)

	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorSMTP),
		Target:         listener.Addr().String(),
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected SMTP probe up, got %#v", result)
	}
}

func TestMemcachedProbe(t *testing.T) {
	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen memcached: %v", err)
	}
	t.Cleanup(func() {
		closeTestResource(t, listener)
	})
	go serveMemcachedProbeTestConnection(t, listener)

	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorMemcached),
		Target:         "memcached://" + listener.Addr().String(),
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected memcached probe up, got %#v", result)
	}
}

func serveSMTPProbeTestConnection(t *testing.T, listener net.Listener) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer closeTestResource(t, conn)
	if _, writeErr := io.WriteString(conn, "220 test ESMTP\r\n"); writeErr != nil {
		return
	}
	buffer := make([]byte, 128)
	n, err := conn.Read(buffer)
	if err != nil || !strings.HasPrefix(string(buffer[:n]), "NOOP") {
		return
	}
	if _, writeErr := io.WriteString(conn, "250 OK\r\n"); writeErr != nil {
		return
	}
	if _, readErr := conn.Read(buffer); readErr != nil {
		return
	}
}

func serveMemcachedProbeTestConnection(t *testing.T, listener net.Listener) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer closeTestResource(t, conn)
	buffer := make([]byte, 128)
	n, err := conn.Read(buffer)
	if err != nil || !strings.HasPrefix(strings.ToLower(string(buffer[:n])), "version") {
		return
	}
	if _, writeErr := io.WriteString(conn, "VERSION 1.6.22\r\n"); writeErr != nil {
		return
	}
}

func TestRedisProbe(t *testing.T) {
	redisServer := miniredis.RunT(t)

	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorRedis),
		Target:         redisServer.Addr(),
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected Redis probe up, got %#v", result)
	}
}

func TestSQLiteDatabaseProbe(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorDatabase),
		Target:         "sqlite://file:probe-test?mode=memory&cache=shared",
		TimeoutSeconds: 2,
	})

	if result.Status != model.StatusUp {
		t.Fatalf("expected database probe up, got %#v", result)
	}
}

func TestMySQLDatabaseProbeRecognized(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           string(model.MonitorMySQL),
		Target:         "mysql://127.0.0.1:1/orivis?timeout=1s",
		TimeoutSeconds: 1,
	})

	if result.Status == model.StatusUnknown {
		t.Fatalf("expected MySQL probe to be recognized, got %#v", result)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected MySQL probe error message")
	}
}

func TestPostgresDatabaseProbeRecognized(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:           "pg",
		Target:         "postgres://127.0.0.1:1/orivis?connect_timeout=1",
		TimeoutSeconds: 1,
	})

	if result.Status == model.StatusUnknown {
		t.Fatalf("expected Postgres probe to be recognized, got %#v", result)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected Postgres probe error message")
	}
}

func TestUnsupportedProbe(t *testing.T) {
	result := probe.New().Check(context.Background(), protocol.AgentTask{
		Type:   "amqp",
		Target: "localhost:6379",
	})

	if result.Status != model.StatusUnknown {
		t.Fatalf("expected unsupported probe unknown, got %#v", result)
	}
	if result.ErrorMessage == "" {
		t.Fatal("expected unsupported probe error message")
	}
}

func closeTestResource(t *testing.T, closer io.Closer) {
	t.Helper()
	if err := closer.Close(); err != nil {
		t.Errorf("close test resource: %v", err)
	}
}
