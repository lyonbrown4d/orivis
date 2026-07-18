package collector_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/collector"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/panjf2000/ants/v2"
	"github.com/spf13/pflag"
)

func waitFor(t *testing.T, ch <-chan error, timeout time.Duration, msg string) error {
	t.Helper()

	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		t.Fatalf("timed out waiting: %s", msg)
		return nil
	}
}

type lifecycleResultQueue struct {
	peekStarted      chan struct{}
	releasePeek      chan struct{}
	closeStarted     chan struct{}
	releaseClose     chan struct{}
	closeCount       atomic.Int32
	closedDuringPeek atomic.Int32
	inPeek           atomic.Int32
}

func newLifecycleResultQueue() *lifecycleResultQueue {
	return &lifecycleResultQueue{
		peekStarted:  make(chan struct{}, 1),
		releasePeek:  nil,
		closeStarted: make(chan struct{}, 1),
		releaseClose: make(chan struct{}),
	}
}

func (q *lifecycleResultQueue) Push(_ context.Context, _ protocol.AgentResultRequest) collector.ResultQueuePush {
	return collector.ResultQueuePush{}
}

func (q *lifecycleResultQueue) Peek(_ context.Context) (protocol.AgentResultRequest, bool) {
	return protocol.AgentResultRequest{}, false
}

func (q *lifecycleResultQueue) PeekBatch(_ context.Context, _ int) ([]protocol.AgentResultRequest, error) {
	q.inPeek.Store(1)
	select {
	case q.peekStarted <- struct{}{}:
	default:
	}
	if q.releasePeek != nil {
		<-q.releasePeek
	}
	q.inPeek.Store(0)
	return []protocol.AgentResultRequest{}, nil
}

func (q *lifecycleResultQueue) Drop(_ context.Context) error {
	return nil
}

func (q *lifecycleResultQueue) DropBatch(_ context.Context, _ int) error {
	return nil
}

func (q *lifecycleResultQueue) Len() int {
	return 0
}

func (q *lifecycleResultQueue) Close() error {
	if q.inPeek.Load() == 1 {
		q.closedDuringPeek.Store(1)
	}
	q.closeCount.Add(1)
	select {
	case q.closeStarted <- struct{}{}:
	default:
	}
	<-q.releaseClose
	return nil
}

func newRuntimeControllerAgentClient(t *testing.T, serverURL string) *agentclient.Client {
	t.Helper()
	cfg := config.Config{}
	cfg.Server.URL = serverURL
	cfg.Transport.RequestTimeout = time.Second
	cfg.Transport.RetryAttempts = 1
	cfg.Transport.RetryBaseDelay = 100 * time.Millisecond
	cfg.Transport.RetryMaxDelay = 100 * time.Millisecond

	client, err := agentclient.New(cfg, slog.New(slog.DiscardHandler), nil)
	if err != nil {
		t.Fatalf("new agent client: %v", err)
	}
	return client
}

func newRuntimeControllerMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler := http.NewServeMux()
	handler.HandleFunc("/api/agent/register", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"agent_id":"agent","region_id":"region","status":"online","server_time":"2026-01-01T00:00:00Z"}`)
	})
	handler.HandleFunc("/api/agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"agent_id":"agent","status":"online","server_time":"2026-01-01T00:00:00Z"}`)
	})
	handler.HandleFunc("/api/agent/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"tasks":[]}`)
	})
	handler.HandleFunc("/api/agent/monitors", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"synced":0}`)
	})
	handler.HandleFunc("/api/agent/results", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"status":"accepted"}`)
	})
	handler.HandleFunc("/api/agent/results/batch", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, `{"accepted":0}`)
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, body); err != nil {
		t.Fatalf("write response body: %v", err)
	}
}

func newRuntimeControllerWatcher(t *testing.T, serverURL string) *config.Watcher {
	t.Helper()

	payload := map[string]any{
		"server": map[string]any{"url": serverURL},
		"log":    map[string]any{"level": "info"},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal controller config: %v", err)
	}

	configFile := filepath.Join(t.TempDir(), "orivis-runtime-controller.json")
	writeErr := os.WriteFile(configFile, raw, 0o600)
	if writeErr != nil {
		t.Fatalf("write controller config: %v", writeErr)
	}

	flags := pflag.NewFlagSet("orivis-runtime-controller", pflag.ContinueOnError)
	watcher, err := config.NewWatcherFromFlags(flags, configFile)
	if err != nil {
		t.Fatalf("new runtime controller watcher: %v", err)
	}
	return watcher
}

func newRuntimeControllerForTest(
	t *testing.T,
	serverURL string,
	deps collector.RuntimeControllerDeps,
) *collector.RuntimeController {
	t.Helper()

	watcher := newRuntimeControllerWatcher(t, serverURL)
	pool, err := ants.NewPool(2)
	if err != nil {
		t.Fatalf("new ants pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Release()
	})

	controller, err := collector.NewRuntimeController(
		watcher,
		slog.New(slog.DiscardHandler),
		nil,
		pool,
		deps,
	)
	if err != nil {
		t.Fatalf("new runtime controller: %v", err)
	}
	return controller
}

func newTestAgentClientWithBlockingTasks(
	t *testing.T,
	tasksStarted chan struct{},
	releaseTasks <-chan struct{},
) (string, *agentclient.Client) {
	t.Helper()
	server := httptest.NewServer(newBlockingTasksAgentHandler(t, tasksStarted, releaseTasks))
	t.Cleanup(server.Close)
	_, client := newTestAgentClientFromURL(t, server.URL)
	return server.URL, client
}

func newBlockingTasksAgentHandler(t *testing.T, tasksStarted chan struct{}, releaseTasks <-chan struct{}) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agent/tasks" {
			select {
			case tasksStarted <- struct{}{}:
			default:
			}
			if releaseTasks != nil {
				<-releaseTasks
			}
		}
		writeJSON(t, w, agentResponseBody(r.URL.Path))
	}
}

func newTestAgentClientFromURL(t *testing.T, serverURL string) (string, *agentclient.Client) {
	t.Helper()
	cfg := config.Config{}
	cfg.Server.URL = serverURL
	cfg.Transport.RequestTimeout = time.Second
	cfg.Transport.RetryAttempts = 1
	cfg.Transport.RetryBaseDelay = 10 * time.Millisecond
	cfg.Transport.RetryMaxDelay = 10 * time.Millisecond
	cfg.Agent.Name = "test-agent"
	cfg.Agent.Region = "test-region"
	cfg.Runtime = "host"
	cfg.Log.Level = "info"
	cfg.Poll.Interval = 10 * time.Millisecond
	client, err := agentclient.New(cfg, slog.New(slog.DiscardHandler), nil)
	if err != nil {
		t.Fatalf("new agent client: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(context.Background()); closeErr != nil {
			t.Fatalf("close agent client: %v", closeErr)
		}
	})
	return serverURL, client
}

func agentResponseBody(path string) string {
	responses := map[string]string{
		"/api/agent/register":      `{"agent_id":"agent-id","region_id":"region","status":"online","server_time":"2026-01-01T00:00:00Z"}`,
		"/api/agent/heartbeat":     `{"agent_id":"agent-id","status":"online","server_time":"2026-01-01T00:00:00Z"}`,
		"/api/agent/tasks":         `{"tasks":[]}`,
		"/api/agent/monitors":      `{"synced":0}`,
		"/api/agent/results":       `{"status":"accepted"}`,
		"/api/agent/results/batch": `{"accepted":0}`,
	}
	response, ok := responses[path]
	if !ok {
		return `{}`
	}
	return response
}
