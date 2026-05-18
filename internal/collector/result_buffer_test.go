package collector_test

import (
	"path/filepath"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/collector"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestFileResultBufferPersistsFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-buffer.jsonl")
	buffer := collector.NewFileResultBuffer(path, 2)

	assertPush(t, buffer, "first", false, 1)
	assertPush(t, buffer, "second", false, 2)
	assertPush(t, buffer, "third", true, 2)

	reopened := collector.NewFileResultBuffer(path, 2)
	assertBufferedMonitor(t, reopened, "second")
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
