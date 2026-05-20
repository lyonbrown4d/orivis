package collector

import (
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

type ResultQueue interface {
	Push(req protocol.AgentResultRequest) ResultQueuePush
	Peek() (protocol.AgentResultRequest, bool)
	PeekBatch(limit int) ([]protocol.AgentResultRequest, error)
	Drop() error
	DropBatch(count int) error
	Len() int
	Close() error
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

func newResultQueue(driver, path string, capacity int) (ResultQueue, error) {
	switch driver {
	case "persistent":
		return NewPersistentResultBuffer(path, capacity)
	case "memory":
		return NewMemoryBadgerResultBuffer(capacity)
	default:
		return NewMemoryBadgerResultBuffer(capacity)
	}
}

func NewResultQueue(cfg config.Config) (ResultQueue, error) {
	if !cfg.Buffer.Enabled {
		return noopResultQueue{}, nil
	}
	return newResultQueue(cfg.Buffer.Driver, cfg.Buffer.Path, cfg.Buffer.Capacity)
}

type noopResultQueue struct{}

func (noopResultQueue) Push(protocol.AgentResultRequest) ResultQueuePush {
	return ResultQueuePush{}
}

func (noopResultQueue) Peek() (protocol.AgentResultRequest, bool) {
	return protocol.AgentResultRequest{}, false
}

func (noopResultQueue) PeekBatch(int) ([]protocol.AgentResultRequest, error) {
	return nil, nil
}

func (noopResultQueue) Drop() error {
	return nil
}

func (noopResultQueue) DropBatch(int) error {
	return nil
}

func (noopResultQueue) Len() int {
	return 0
}

func (noopResultQueue) Close() error {
	return nil
}
