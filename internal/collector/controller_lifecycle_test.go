package collector_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/collector"
)

func TestRuntimeController_StartIsIdempotent(t *testing.T) {
	server := newRuntimeControllerMockServer(t)
	ctx := context.Background()

	clientCalls := atomic.Int32{}
	controller := newRuntimeControllerForTest(t, server.URL, collector.RuntimeControllerDeps{
		NewClient: func(cfg config.Config) (*agentclient.Client, error) {
			clientCalls.Add(1)
			return newRuntimeControllerAgentClient(t, cfg.Server.URL), nil
		},
		NewResultQueue: func(ctx context.Context, cfg config.Config) (collector.ResultQueue, error) {
			cfg.Buffer.Enabled = true
			cfg.Buffer.Driver = "memory"
			return collector.NewResultQueue(ctx, cfg)
		},
	})

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			errs <- controller.Start(ctx)
		}()
	}

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("unexpected start error: %v", err)
		}
	}

	if got, want := clientCalls.Load(), int32(1); got != want {
		t.Fatalf("expected agent client to be created once, got %d", got)
	}

	if err := controller.Stop(ctx); err != nil {
		t.Fatalf("stop controller: %v", err)
	}
	if err := controller.Stop(ctx); err != nil {
		t.Fatalf("second stop controller: %v", err)
	}
}

func TestRuntimeController_StopIsIdempotentDuringBlockingClose(t *testing.T) {
	server := newRuntimeControllerMockServer(t)
	ctx := context.Background()

	queue := newLifecycleResultQueue()
	controller := newRuntimeControllerForTest(t, server.URL, collector.RuntimeControllerDeps{
		NewClient: func(cfg config.Config) (*agentclient.Client, error) {
			return newRuntimeControllerAgentClient(t, cfg.Server.URL), nil
		},
		NewResultQueue: func(context.Context, config.Config) (collector.ResultQueue, error) {
			return queue, nil
		},
	})

	if err := controller.Start(ctx); err != nil {
		t.Fatalf("start controller: %v", err)
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- controller.Stop(ctx)
	}()

	select {
	case <-queue.closeStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for runtime close to begin")
	}

	stopAgainDone := make(chan error, 1)
	go func() {
		stopAgainDone <- controller.Stop(ctx)
	}()

	select {
	case err := <-stopAgainDone:
		if err != nil {
			t.Fatalf("unexpected second stop error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second stop blocked while first stop is closing runtime")
	}

	close(queue.releaseClose)

	if err := waitFor(t, stopDone, 2*time.Second, "waiting for first stop to complete"); err != nil {
		t.Fatalf("initial stop error: %v", err)
	}
	if queue.closeCount.Load() != 1 {
		t.Fatalf("expected close once, got %d", queue.closeCount.Load())
	}
	if queue.closedDuringPeek.Load() != 0 {
		t.Fatal("result queue was closed while background flush was in peek")
	}
}
