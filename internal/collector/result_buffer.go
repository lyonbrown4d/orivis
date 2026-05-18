package collector

import (
	"sync"

	"github.com/lyonbrown4d/orivis/internal/protocol"
)

type resultBuffer struct {
	mu    sync.Mutex
	max   int
	items []protocol.AgentResultRequest
}

type resultBufferPush struct {
	size          int
	buffered      bool
	droppedOldest bool
}

func newResultBuffer(capacity int) *resultBuffer {
	if capacity < 0 {
		capacity = 0
	}
	return &resultBuffer{
		max:   capacity,
		items: make([]protocol.AgentResultRequest, 0, min(capacity, 64)),
	}
}

func (b *resultBuffer) push(req protocol.AgentResultRequest) resultBufferPush {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.max == 0 {
		return resultBufferPush{}
	}
	if len(b.items) < b.max {
		b.items = append(b.items, req)
		return resultBufferPush{size: len(b.items), buffered: true}
	}
	copy(b.items, b.items[1:])
	b.items[len(b.items)-1] = req
	return resultBufferPush{size: len(b.items), buffered: true, droppedOldest: true}
}

func (b *resultBuffer) peek() (protocol.AgentResultRequest, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) == 0 {
		return protocol.AgentResultRequest{}, false
	}
	return b.items[0], true
}

func (b *resultBuffer) drop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) == 0 {
		return
	}
	copy(b.items, b.items[1:])
	b.items[len(b.items)-1] = protocol.AgentResultRequest{}
	b.items = b.items[:len(b.items)-1]
}

func (b *resultBuffer) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.items)
}
