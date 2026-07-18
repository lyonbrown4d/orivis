package ingest_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/arcgolabs/eventx"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestResultIngestor_StopNilContextReturnsContextRequiredError(t *testing.T) {
	t.Parallel()

	ingestor := newResultIngestorForTest(t)

	if err := ingestor.Stop(typedNilContext()); !errors.Is(err, ingest.ErrResultIngestorContextRequired) {
		t.Fatalf("expected stop with nil context to return context required error, got %v", err)
	}
}

func TestResultIngestor_StartAfterStopReturnsClosedError(t *testing.T) {
	t.Parallel()

	ingestor := newResultIngestorForTest(t)
	ctx := t.Context()

	if err := ingestor.Start(ctx); err != nil {
		t.Fatalf("start result ingestor: %v", err)
	}
	if err := ingestor.Stop(ctx); err != nil {
		t.Fatalf("stop result ingestor: %v", err)
	}

	err := ingestor.Start(ctx)
	if !errors.Is(err, ingest.ErrClosed) {
		t.Fatalf("expected start after stop to return ErrClosed, got %v", err)
	}

	if err := ingestor.Enqueue(ctx, store.RecordProbeResultParams{}); !errors.Is(err, ingest.ErrClosed) {
		t.Fatalf("expected enqueue after stop to return ErrClosed, got %v", err)
	}
}

func TestResultIngestor_StopIsIdempotentAndConcurrentSafe(t *testing.T) {
	t.Parallel()

	ingestor := newResultIngestorForTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err := ingestor.Start(ctx); err != nil {
		t.Fatalf("start result ingestor: %v", err)
	}

	const attempts = 8
	stopErrs := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Go(func() {
			stopErrs <- ingestor.Stop(ctx)
		})
	}
	wg.Wait()
	close(stopErrs)

	for err := range stopErrs {
		if err != nil {
			t.Fatalf("expected concurrent stop to be idempotent and error-free, got: %v", err)
		}
	}

	if err := ingestor.Start(ctx); !errors.Is(err, ingest.ErrClosed) {
		t.Fatalf("expected start after concurrent stop to return ErrClosed, got: %v", err)
	}
}

func TestResultIngestor_StopWithoutStartAndEmptyQueueIsNoop(t *testing.T) {
	t.Parallel()

	ingestor := newResultIngestorForTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err := ingestor.Stop(ctx); err != nil {
		t.Fatalf("expected stop without start and empty queue to be noop-success, got: %v", err)
	}

	if err := ingestor.Start(ctx); !errors.Is(err, ingest.ErrClosed) {
		t.Fatalf("expected start after stop to return ErrClosed, got: %v", err)
	}
}

func TestResultIngestor_StopWithoutStartFlushesNonEmptyQueue(t *testing.T) {
	t.Parallel()

	ingestor := newResultIngestorForTestWithStore(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err := ingestor.Enqueue(ctx, store.RecordProbeResultParams{
		MonitorID: "monitor-id",
		Status:    "invalid",
	}); err != nil {
		t.Fatalf("enqueue before start: %v", err)
	}

	err := ingestor.Stop(ctx)
	if !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("expected stop to run flush path for queued items and return validation error, got: %v", err)
	}
	if err := ingestor.Flush(ctx); err != nil {
		t.Fatalf("expected stop to flush queued items, got queue flush error: %v", err)
	}
}

func newResultIngestorForTest(t *testing.T) *ingest.ResultIngestor {
	t.Helper()

	bus := eventx.New()
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close event bus: %v", err)
		}
	})

	ingestor, err := ingest.NewResultIngestor(resultIngestorTestConfig(), nil, nil, bus, nil, nil)
	if err != nil {
		t.Fatalf("new result ingestor: %v", err)
	}
	return ingestor
}

func newResultIngestorForTestWithStore(t *testing.T) *ingest.ResultIngestor {
	t.Helper()

	storage := newResultStoreForTest(t)
	bus := eventx.New()
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close event bus: %v", err)
		}
	})

	ingestor, err := ingest.NewResultIngestor(resultIngestorTestConfig(), storage, nil, bus, nil, nil)
	if err != nil {
		t.Fatalf("new result ingestor with store: %v", err)
	}
	return ingestor
}

func resultIngestorTestConfig() config.Config {
	cfg := config.Config{}
	cfg.Ingest.QueueSize = 16
	cfg.Ingest.BatchSize = 4
	cfg.Ingest.FlushInterval = "10ms"
	return cfg
}

func newResultStoreForTest(t *testing.T) *store.Store {
	t.Helper()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "orivis-ingest-lifecycle.db")) + "?mode=rwc"

	storage, err := store.Open(cfg, nil)
	if err != nil {
		t.Fatalf("open result test store: %v", err)
	}
	t.Cleanup(func() {
		if err := storage.Close(context.Background()); err != nil {
			t.Fatalf("close result test store: %v", err)
		}
	})

	return storage
}

func typedNilContext() context.Context {
	var ctx context.Context
	return ctx
}
