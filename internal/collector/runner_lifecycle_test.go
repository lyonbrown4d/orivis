package collector_test

import (
	"context"
	"testing"
	"time"

	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/collector"
	"log/slog"

	"github.com/panjf2000/ants/v2"
)

func TestRunnerStopWaitsForBackgroundGoroutinesBeforeClosingResultBuffer(t *testing.T) {
	tasksStarted := make(chan struct{}, 1)
	releaseTasks := make(chan struct{})
	serverURL, client := newTestAgentClientWithBlockingTasks(t, tasksStarted, releaseTasks)

	pool, err := ants.NewPool(2)
	if err != nil {
		t.Fatalf("new task pool: %v", err)
	}
	t.Cleanup(pool.Release)

	results := newLifecycleResultQueue()
	runner := collector.NewRunner(
		runnerLifecycleConfig(serverURL),
		slog.New(slog.DiscardHandler),
		nil,
		client,
		pool,
		nil,
		results,
	)
	if err := runner.Start(context.Background()); err != nil {
		t.Fatalf("start runner: %v", err)
	}

	waitForSignal(t, tasksStarted, time.Second, "timed out waiting for runner sync task call to block")

	stopErrChan := make(chan error, 1)
	go func() {
		stopErrChan <- runner.Stop(context.Background())
	}()

	mustBlock(t, stopErrChan, 100*time.Millisecond, "runner stop returned before background sync task completed")
	mustNotEmit(t, results.closeStarted, 100*time.Millisecond, "result queue closed while background sync task was still blocked")

	close(releaseTasks)
	waitForSignal(t, results.closeStarted, time.Second, "timed out waiting for result queue close attempt")

	mustBlock(t, stopErrChan, 100*time.Millisecond, "runner stop returned before result buffer close was released")
	close(results.releaseClose)

	waitForStop(t, stopErrChan, time.Second)
	assertResultQueueClosedOnce(t, results)

	if err := runner.Stop(context.Background()); err != nil {
		t.Fatalf("second stop should be idempotent: %v", err)
	}
}

func runnerLifecycleConfig(serverURL string) config.Config {
	cfg := config.Config{}
	cfg.Buffer.Enabled = true
	cfg.Buffer.Driver = "memory"
	cfg.Buffer.Capacity = 10
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
	cfg.Poll.Workers = 1
	return cfg
}

func waitForSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

func mustBlock(t *testing.T, ch <-chan error, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	case <-time.After(timeout):
	}
}

func waitForStop(t *testing.T, stopErr <-chan error, timeout time.Duration) {
	t.Helper()
	select {
	case err := <-stopErr:
		if err != nil {
			t.Fatalf("stop runner: %v", err)
		}
	case <-time.After(timeout):
		t.Fatal("timed out waiting for runner stop")
	}
}

func assertResultQueueClosedOnce(t *testing.T, queue *lifecycleResultQueue) {
	t.Helper()
	if queue.closeCount.Load() != 1 {
		t.Fatalf("expected result queue to close once, got %d", queue.closeCount.Load())
	}
}

func mustNotEmit(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	case <-time.After(timeout):
	}
}
