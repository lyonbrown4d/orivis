package collector_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/collector"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestBadgerResultBufferLifecycle(t *testing.T) {
	ctx := context.Background()
	buffer, err := collector.NewMemoryBadgerResultBuffer(2)
	if err != nil {
		t.Fatalf("new memory badger result buffer: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := buffer.Close(); closeErr != nil {
			t.Fatalf("close memory badger result buffer: %v", closeErr)
		}
	})

	if err := buffer.Close(); err != nil {
		t.Fatalf("close badger result buffer: %v", err)
	}

	assertQueuePushAfterClose(ctx, t, buffer)
	assertQueuePeekAfterClose(ctx, t, buffer)
	assertQueuePeekBatchAfterClose(ctx, t, buffer)
	assertQueueDropAfterClose(ctx, t, buffer)
	assertQueueDropBatchAfterClose(ctx, t, buffer)
	assertQueueCompactAfterClose(ctx, t, buffer)
	assertBufferLen(ctx, t, 0, buffer.Len())
}

func TestBadgerResultBufferContextRequiredForStorageOps(t *testing.T) {
	buffer, err := collector.NewMemoryBadgerResultBuffer(2)
	if err != nil {
		t.Fatalf("new memory badger result buffer: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := buffer.Close(); closeErr != nil {
			t.Fatalf("close memory badger result buffer: %v", closeErr)
		}
	})

	var nilCtx context.Context
	assertContextRequiredForStorageOps(nilCtx, t, buffer)
}

func assertQueuePushAfterClose(ctx context.Context, t *testing.T, buffer collector.ResultQueue) {
	t.Helper()
	pushed := buffer.Push(ctx, protocol.AgentResultRequest{ResultID: "test", MonitorID: "test"})
	if pushed.Buffered() || pushed.Size() != 0 || pushed.DroppedOldest() {
		t.Fatalf("expected push after close to return empty result, got %#v", pushed)
	}
}

func assertQueuePeekAfterClose(ctx context.Context, t *testing.T, buffer collector.ResultQueue) {
	t.Helper()
	if _, ok := buffer.Peek(ctx); ok {
		t.Fatal("expected peek after close to return no result")
	}
}

func assertQueuePeekBatchAfterClose(ctx context.Context, t *testing.T, buffer collector.ResultQueue) {
	t.Helper()
	batch, batchErr := buffer.PeekBatch(ctx, 1)
	if !errors.Is(batchErr, collector.ErrResultBufferClosed) {
		t.Fatalf("peek batch after close: %v", batchErr)
	}
	if len(batch) != 0 {
		t.Fatalf("expected empty peek batch after close, got %d", len(batch))
	}
}

func assertQueueDropAfterClose(ctx context.Context, t *testing.T, buffer collector.ResultQueue) {
	t.Helper()
	if err := buffer.Drop(ctx); !errors.Is(err, collector.ErrResultBufferClosed) {
		t.Fatalf("expected drop after close to return closed error, got %v", err)
	}
}

func assertQueueDropBatchAfterClose(ctx context.Context, t *testing.T, buffer collector.ResultQueue) {
	t.Helper()
	if err := buffer.DropBatch(ctx, 1); !errors.Is(err, collector.ErrResultBufferClosed) {
		t.Fatalf("expected drop batch after close to return closed error, got %v", err)
	}
}

func assertQueueCompactAfterClose(ctx context.Context, t *testing.T, buffer collector.ResultQueue) {
	t.Helper()
	compactable, ok := buffer.(interface {
		Compact(context.Context) (bool, error)
	})
	if !ok {
		t.Fatal("expected buffer to support compacting")
	}
	if _, err := compactable.Compact(ctx); !errors.Is(err, collector.ErrResultBufferClosed) {
		t.Fatalf("expected compact after close to return closed error, got %v", err)
	}
}

func assertBufferLen(ctx context.Context, t *testing.T, want, got int) {
	t.Helper()
	if ctx == nil {
		t.Fatal("context should not be nil when asserting buffer length")
	}
	if got != want {
		t.Fatalf("expected closed buffer length %d, got %d", want, got)
	}
}

func assertContextRequiredForStorageOps(ctx context.Context, t *testing.T, buffer collector.ResultQueue) {
	t.Helper()
	pushed := buffer.Push(ctx, protocol.AgentResultRequest{ResultID: "test", MonitorID: "test"})
	if pushed.Buffered() || pushed.Size() != 0 || pushed.DroppedOldest() {
		t.Fatalf("expected push with nil context to be rejected, got %#v", pushed)
	}
	if _, ok := buffer.Peek(ctx); ok {
		t.Fatal("expected peek with nil context to return no result")
	}
	if _, err := buffer.PeekBatch(ctx, 1); !errors.Is(err, collector.ErrResultBufferContextRequired) {
		t.Fatalf("expected peek batch with nil context to return context required error: %v", err)
	}
	if !errors.Is(buffer.Drop(ctx), collector.ErrResultBufferContextRequired) {
		t.Fatalf("expected drop with nil context to return context required error: %v", buffer.Drop(ctx))
	}
	if !errors.Is(buffer.DropBatch(ctx, 1), collector.ErrResultBufferContextRequired) {
		t.Fatalf("expected drop batch with nil context to return context required error: %v", buffer.DropBatch(ctx, 1))
	}
}
