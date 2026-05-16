// Package ingest provides asynchronous ingestion pipelines for server-side data writes.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

var (
	ErrQueueFull = errors.New("ingest: result queue is full")
	ErrClosed    = errors.New("ingest: result ingestor is closed")
)

type ResultIngestor struct {
	store         *store.Store
	logger        *slog.Logger
	queue         *resultQueue
	batchSize     int
	flushInterval time.Duration
	wake          chan struct{}
	stop          chan struct{}
	done          chan struct{}
	startOnce     sync.Once
	stopOnce      sync.Once
	started       atomic.Bool
}

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

func NewResultIngestor(cfg config.Config, storage *store.Store, logger *slog.Logger) (*ResultIngestor, error) {
	queueSize := cfg.Ingest.QueueSize
	if queueSize <= 0 {
		queueSize = 4096
	}
	batchSize := cfg.Ingest.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	flushInterval, err := time.ParseDuration(cfg.Ingest.FlushInterval)
	if err != nil {
		return nil, fmt.Errorf("parse ingest flush interval: %w", err)
	}
	if flushInterval <= 0 {
		flushInterval = time.Second
	}

	queue, err := newResultQueue(queueSize)
	if err != nil {
		return nil, err
	}
	return &ResultIngestor{
		store:         storage,
		logger:        logger,
		queue:         queue,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		wake:          make(chan struct{}, 1),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}, nil
}

func (i *ResultIngestor) Enqueue(ctx context.Context, params store.RecordProbeResultParams) error {
	if i == nil {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("enqueue probe result: %w", err)
	}

	size, err := i.queue.push(cloneRecordProbeResultParams(params))
	if err != nil {
		return err
	}
	if size >= i.batchSize {
		i.notify()
	}
	return nil
}

func (i *ResultIngestor) Start(ctx context.Context) error {
	if i == nil {
		return nil
	}
	i.startOnce.Do(func() {
		i.started.Store(true)
		go i.run(ctx)
	})
	return nil
}

func (i *ResultIngestor) Stop(ctx context.Context) error {
	if i == nil {
		return nil
	}
	i.queue.close()
	if !i.started.Load() {
		return i.Flush(ctx)
	}

	i.stopOnce.Do(func() {
		close(i.stop)
	})
	select {
	case <-i.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop result ingestor: %w", ctx.Err())
	}
}

func (i *ResultIngestor) Flush(ctx context.Context) error {
	if i == nil {
		return nil
	}
	return i.flush(ctx)
}

func (i *ResultIngestor) run(ctx context.Context) {
	defer close(i.done)

	ticker := time.NewTicker(i.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			i.flushOnStop(ctx)
			return
		case <-i.stop:
			i.flushOnStop(ctx)
			return
		case <-ticker.C:
			i.logFlushError(i.flush(ctx))
		case <-i.wake:
			i.logFlushError(i.flush(ctx))
		}
	}
}

func (i *ResultIngestor) notify() {
	select {
	case i.wake <- struct{}{}:
	default:
	}
}

func (i *ResultIngestor) flush(ctx context.Context) error {
	if i.store == nil || i.store.ResultStore() == nil {
		return errors.New("ingest: result store is not available")
	}

	var flushErr error
	for {
		done, err := i.flushNextBatch(ctx)
		if err != nil {
			flushErr = errors.Join(flushErr, err)
		}
		if done {
			return flushErr
		}
	}
}

func (i *ResultIngestor) flushNextBatch(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, fmt.Errorf("flush result ingest queue: %w", err)
	}

	batch := i.queue.popBatch(i.batchSize)
	if len(batch) == 0 {
		return true, nil
	}
	if err := i.recordBatch(ctx, batch); err != nil {
		return len(batch) < i.batchSize, err
	}
	return len(batch) < i.batchSize, nil
}

func (i *ResultIngestor) recordBatch(ctx context.Context, batch []store.RecordProbeResultParams) error {
	if _, err := i.store.ResultStore().RecordBatch(ctx, batch); err != nil {
		if len(batch) == 1 {
			return fmt.Errorf("record probe result batch: %w", err)
		}
		i.logFlushError(fmt.Errorf("record probe result batch: %w", err))
		return i.recordIndividually(ctx, batch)
	}
	return nil
}

func (i *ResultIngestor) recordIndividually(ctx context.Context, batch []store.RecordProbeResultParams) error {
	var batchErr error
	for index := range batch {
		if _, err := i.store.ResultStore().Record(ctx, batch[index]); err != nil {
			batchErr = errors.Join(batchErr, err)
		}
	}
	return batchErr
}

func (i *ResultIngestor) flushOnStop(ctx context.Context) {
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), i.flushInterval)
	defer cancel()

	if err := i.flush(flushCtx); err != nil && i.logger != nil {
		i.logger.Error("flush result ingest queue on stop", "error", err)
	}
}

func (i *ResultIngestor) logFlushError(err error) {
	if err != nil && i.logger != nil {
		i.logger.Error("flush result ingest queue", "error", err)
	}
}

func newResultQueue(capacity int) (*resultQueue, error) {
	items, err := collectionlist.NewPriorityQueue(func(a, b resultQueueItem) bool {
		return a.sequence < b.sequence
	})
	if err != nil {
		return nil, fmt.Errorf("create result ingest queue: %w", err)
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
		return q.items.Len(), ErrClosed
	}
	if q.items.Len() >= q.capacity {
		return q.items.Len(), ErrQueueFull
	}
	q.next++
	q.items.Push(resultQueueItem{
		sequence: q.next,
		params:   params,
	})
	return q.items.Len(), nil
}

func (q *resultQueue) popBatch(limit int) []store.RecordProbeResultParams {
	if limit <= 0 {
		return nil
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]store.RecordProbeResultParams, 0, min(limit, q.items.Len()))
	for len(out) < limit {
		item, ok := q.items.Pop()
		if !ok {
			return out
		}
		out = append(out, item.params)
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
