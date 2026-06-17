package client_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestClientPreservesServerBasePath(t *testing.T) {
	paths := &recordedPaths{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths.add(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(agentClientTestResponse(r.URL.Path)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := newBasePathTestClient(t, server.URL+"/orivis")
	t.Cleanup(func() {
		if err := client.Close(context.Background()); err != nil {
			t.Fatalf("close client: %v", err)
		}
	})

	exerciseAgentClient(t, client)
	want := []string{
		"/orivis/api/agent/register",
		"/orivis/api/agent/heartbeat",
		"/orivis/api/agent/tasks",
		"/orivis/api/agent/monitors",
		"/orivis/api/agent/results",
		"/orivis/api/agent/results/batch",
	}
	if got := paths.values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected request paths: got %#v, want %#v", got, want)
	}
}

type recordedPaths struct {
	mu         sync.Mutex
	valuesList []string
}

func (p *recordedPaths) add(value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.valuesList = append(p.valuesList, value)
}

func (p *recordedPaths) values() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.valuesList...)
}

func newBasePathTestClient(t *testing.T, serverURL string) *agentclient.Client {
	t.Helper()
	var cfg config.Config
	cfg.Server.URL = serverURL
	cfg.Transport.RequestTimeout = time.Second
	cfg.Transport.RetryAttempts = 1
	cfg.Transport.RetryBaseDelay = time.Millisecond
	cfg.Transport.RetryMaxDelay = time.Millisecond
	client, err := agentclient.New(cfg, slog.New(slog.DiscardHandler), nil)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

func exerciseAgentClient(t *testing.T, client *agentclient.Client) {
	t.Helper()
	ctx := context.Background()
	if _, err := client.Register(ctx, protocol.AgentRegisterRequest{Name: "agent"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := client.Heartbeat(ctx, protocol.AgentHeartbeatRequest{AgentID: "agent"}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if _, err := client.Tasks(ctx, protocol.AgentTasksRequest{AgentID: "agent"}); err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if _, err := client.SyncMonitors(ctx, protocol.AgentMonitorSyncRequest{AgentID: "agent"}); err != nil {
		t.Fatalf("sync monitors: %v", err)
	}
	if err := client.ReportResult(ctx, protocol.AgentResultRequest{AgentID: "agent"}); err != nil {
		t.Fatalf("report result: %v", err)
	}
	if _, err := client.ReportResults(ctx, protocol.AgentResultBatchRequest{AgentID: "agent"}); err != nil {
		t.Fatalf("report results: %v", err)
	}
}

func agentClientTestResponse(path string) []byte {
	switch path {
	case "/orivis/api/agent/register":
		return []byte(`{"agent_id":"agent","region_id":"region","status":"online","server_time":"2026-01-01T00:00:00Z"}`)
	case "/orivis/api/agent/heartbeat":
		return []byte(`{"agent_id":"agent","status":"online","server_time":"2026-01-01T00:00:00Z"}`)
	case "/orivis/api/agent/tasks":
		return []byte(`{"tasks":[]}`)
	case "/orivis/api/agent/monitors":
		return []byte(`{"synced":0}`)
	case "/orivis/api/agent/results":
		return []byte(`{"status":"accepted"}`)
	case "/orivis/api/agent/results/batch":
		return []byte(`{"accepted":0}`)
	default:
		return []byte(`{"error":"unexpected path"}`)
	}
}
