package ingest

import (
	"sync"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type resultQueue struct {
	mu       sync.Mutex
	items    *collectionlist.PriorityQueue[resultQueueItem]
	capacity int
	next     uint64
	closed   bool
}

type resultQueueItem struct {
	sequence uint64
	params   store.RecordProbeResultParams
}

func newResultQueue(capacity int) (*resultQueue, error) {
	items, err := collectionlist.NewPriorityQueue(func(a, b resultQueueItem) bool {
		return a.sequence < b.sequence
	})
	if err != nil {
		return nil, wrapError(err, "create result ingest queue")
	}
	return &resultQueue{
		items:    items,
		capacity: capacity,
	}, nil
}

func (q *resultQueue) push(params store.RecordProbeResultParams) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return q.items.Len(), wrapError(ErrClosed, "push result ingest queue")
	}
	if q.items.Len() >= q.capacity {
		return q.items.Len(), wrapError(ErrQueueFull, "push result ingest queue")
	}
	q.next++
	q.items.Push(resultQueueItem{
		sequence: q.next,
		params:   params,
	})
	return q.items.Len(), nil
}

func (q *resultQueue) popBatch(limit int) *collectionlist.List[store.RecordProbeResultParams] {
	if limit <= 0 {
		return collectionlist.NewList[store.RecordProbeResultParams]()
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	out := collectionlist.NewListWithCapacity[store.RecordProbeResultParams](min(limit, q.items.Len()))
	for out.Len() < limit {
		item, ok := q.items.Pop()
		if !ok {
			return out
		}
		out.Add(item.params)
	}
	return out
}

func (q *resultQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
}

func cloneRecordProbeResultParams(params store.RecordProbeResultParams) store.RecordProbeResultParams {
	params.RawDetail = append([]byte(nil), params.RawDetail...)
	return params
}
