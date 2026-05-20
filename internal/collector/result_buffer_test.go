package collector_test

import (
	"path/filepath"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/collector"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestBadgerResultBufferPersistsFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-buffer.badger")
	buffer, err := collector.NewPersistentResultBuffer(path, 2)
	if err != nil {
		t.Fatalf("new persistent result buffer: %v", err)
	}

	assertPush(t, buffer, "first", false, 1)
	assertPush(t, buffer, "second", false, 2)
	assertPush(t, buffer, "third", true, 2)
	if closeErr := buffer.Close(); closeErr != nil {
		t.Fatalf("close badger result buffer: %v", closeErr)
	}

	reopened, err := collector.NewPersistentResultBuffer(path, 2)
	if err != nil {
		t.Fatalf("reopen persistent result buffer: %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Errorf("close reopened badger result buffer: %v", err)
		}
	})
	assertBufferedMonitor(t, reopened, "second")
	assertBufferedBatch(t, reopened, []string{"second", "third"})
	if err := reopened.Drop(); err != nil {
		t.Fatalf("drop second result: %v", err)
	}
	assertBufferedMonitor(t, reopened, "third")
	if err := reopened.Drop(); err != nil {
		t.Fatalf("drop third result: %v", err)
	}
	if reopened.Len() != 0 {
		t.Fatalf("expected empty reopened buffer, got %d", reopened.Len())
	}
}

func TestBadgerMemoryResultBuffer(t *testing.T) {
	buffer, err := collector.NewMemoryBadgerResultBuffer(2)
	if err != nil {
		t.Fatalf("new memory badger result buffer: %v", err)
	}
	t.Cleanup(func() {
		if err := buffer.Close(); err != nil {
			t.Errorf("close memory badger result buffer: %v", err)
		}
	})

	assertPush(t, buffer, "first", false, 1)
	assertPush(t, buffer, "second", false, 2)
	assertPush(t, buffer, "third", true, 2)
	assertBufferedMonitor(t, buffer, "second")
	assertBufferedBatch(t, buffer, []string{"second", "third"})
	if err := buffer.DropBatch(2); err != nil {
		t.Fatalf("drop memory badger result buffer batch: %v", err)
	}
	if buffer.Len() != 0 {
		t.Fatalf("expected empty memory buffer, got %d", buffer.Len())
	}
}

func assertPush(t *testing.T, buffer collector.ResultQueue, monitorID string, droppedOldest bool, size int) {
	t.Helper()
	result := buffer.Push(bufferedResult(monitorID))
	if result.Buffered() != true || result.DroppedOldest() != droppedOldest || result.Size() != size {
		t.Fatalf("unexpected push result for %q: %#v", monitorID, result)
	}
}

func bufferedResult(monitorID string) protocol.AgentResultRequest {
	return protocol.AgentResultRequest{MonitorID: monitorID, Status: "up"}
}

func assertBufferedMonitor(t *testing.T, buffer collector.ResultQueue, monitorID string) {
	t.Helper()
	req, ok := buffer.Peek()
	if !ok {
		t.Fatalf("expected buffered result %q", monitorID)
	}
	if req.MonitorID != monitorID {
		t.Fatalf("expected buffered result %q, got %#v", monitorID, req)
	}
}

func assertBufferedBatch(t *testing.T, buffer collector.ResultQueue, monitorIDs []string) {
	t.Helper()
	batch, err := buffer.PeekBatch(len(monitorIDs))
	if err != nil {
		t.Fatalf("peek buffered result batch: %v", err)
	}
	if len(batch) != len(monitorIDs) {
		t.Fatalf("expected buffered result batch len %d, got %d", len(monitorIDs), len(batch))
	}
	for i, monitorID := range monitorIDs {
		if batch[i].MonitorID != monitorID {
			t.Fatalf("expected buffered result batch[%d] %q, got %#v", i, monitorID, batch[i])
		}
	}
}
