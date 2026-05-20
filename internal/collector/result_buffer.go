package collector

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

type ResultQueue interface {
	Push(req protocol.AgentResultRequest) ResultQueuePush
	Peek() (protocol.AgentResultRequest, bool)
	Drop() error
	Len() int
}

type ResultQueuePush struct {
	size          int
	buffered      bool
	droppedOldest bool
	err           error
}

func (p ResultQueuePush) Size() int {
	return p.size
}

func (p ResultQueuePush) Buffered() bool {
	return p.buffered
}

func (p ResultQueuePush) DroppedOldest() bool {
	return p.droppedOldest
}

func newResultQueue(driver, path string, capacity int) ResultQueue {
	if driver == "file" {
		return NewFileResultBuffer(path, capacity)
	}
	return newMemoryResultBuffer(capacity)
}

func NewResultQueue(cfg config.Config) ResultQueue {
	if !cfg.Buffer.Enabled {
		return nil
	}
	return newResultQueue(cfg.Buffer.Driver, cfg.Buffer.Path, cfg.Buffer.Capacity)
}

type memoryResultBuffer struct {
	mu    sync.Mutex
	max   int
	items []protocol.AgentResultRequest
}

func newMemoryResultBuffer(capacity int) *memoryResultBuffer {
	if capacity < 0 {
		capacity = 0
	}
	return &memoryResultBuffer{
		max:   capacity,
		items: make([]protocol.AgentResultRequest, 0, min(capacity, 64)),
	}
}

func (b *memoryResultBuffer) Push(req protocol.AgentResultRequest) ResultQueuePush {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.max == 0 {
		return ResultQueuePush{}
	}
	if len(b.items) < b.max {
		b.items = append(b.items, req)
		return ResultQueuePush{size: len(b.items), buffered: true}
	}
	copy(b.items, b.items[1:])
	b.items[len(b.items)-1] = req
	return ResultQueuePush{size: len(b.items), buffered: true, droppedOldest: true}
}

func (b *memoryResultBuffer) Peek() (protocol.AgentResultRequest, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) == 0 {
		return protocol.AgentResultRequest{}, false
	}
	return b.items[0], true
}

func (b *memoryResultBuffer) Drop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) == 0 {
		return nil
	}
	copy(b.items, b.items[1:])
	b.items[len(b.items)-1] = protocol.AgentResultRequest{}
	b.items = b.items[:len(b.items)-1]
	return nil
}

func (b *memoryResultBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.items)
}

type fileResultBuffer struct {
	mu   sync.Mutex
	max  int
	path string
}

func NewFileResultBuffer(path string, capacity int) *fileResultBuffer {
	if capacity < 0 {
		capacity = 0
	}
	return &fileResultBuffer{max: capacity, path: path}
}

func (b *fileResultBuffer) Push(req protocol.AgentResultRequest) ResultQueuePush {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.max == 0 {
		return ResultQueuePush{}
	}

	items, err := b.readLocked()
	if err != nil {
		return ResultQueuePush{err: err}
	}
	droppedOldest := false
	if len(items) >= b.max {
		items = items[len(items)-b.max+1:]
		droppedOldest = true
	}
	items = append(items, req)
	if err := b.writeLocked(items); err != nil {
		return ResultQueuePush{size: len(items), err: err}
	}
	return ResultQueuePush{size: len(items), buffered: true, droppedOldest: droppedOldest}
}

func (b *fileResultBuffer) Peek() (protocol.AgentResultRequest, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	items, err := b.readLocked()
	if err != nil || len(items) == 0 {
		return protocol.AgentResultRequest{}, false
	}
	return items[0], true
}

func (b *fileResultBuffer) Drop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	items, err := b.readLocked()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return b.writeLocked(items[1:])
}

func (b *fileResultBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	items, err := b.readLocked()
	if err != nil {
		return 0
	}
	return len(items)
}

func (b *fileResultBuffer) readLocked() ([]protocol.AgentResultRequest, error) {
	raw, err := os.ReadFile(b.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, oops.Wrapf(err, "read agent result buffer file")
	}
	if len(raw) == 0 {
		return nil, nil
	}

	lines := bytesLines(raw)
	items := make([]protocol.AgentResultRequest, 0, min(len(lines), b.max))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var item protocol.AgentResultRequest
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, oops.Wrapf(err, "decode agent result buffer entry")
		}
		items = append(items, item)
	}
	if b.max > 0 && len(items) > b.max {
		items = items[len(items)-b.max:]
	}
	return items, nil
}

func (b *fileResultBuffer) writeLocked(items []protocol.AgentResultRequest) error {
	if len(items) == 0 {
		if err := os.Remove(b.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return oops.Wrapf(err, "remove empty agent result buffer file")
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(b.path), 0o700); err != nil {
		return oops.Wrapf(err, "create agent result buffer directory")
	}
	raw, err := marshalResultBufferItems(items)
	if err != nil {
		return err
	}
	tmpPath := b.path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return oops.Wrapf(err, "write agent result buffer file")
	}
	if err := os.Rename(tmpPath, b.path); err != nil {
		return oops.Wrapf(err, "replace agent result buffer file")
	}
	return nil
}

func marshalResultBufferItems(items []protocol.AgentResultRequest) ([]byte, error) {
	out := make([]byte, 0, len(items)*128)
	for i := range items {
		raw, err := json.Marshal(&items[i])
		if err != nil {
			return nil, oops.Wrapf(err, "encode agent result buffer entry")
		}
		out = append(out, raw...)
		out = append(out, '\n')
	}
	return out, nil
}

func bytesLines(raw []byte) [][]byte {
	lines := make([][]byte, 0)
	start := 0
	for i, b := range raw {
		if b != '\n' {
			continue
		}
		lines = append(lines, raw[start:i])
		start = i + 1
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}
