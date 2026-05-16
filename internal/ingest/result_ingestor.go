// Package ingest provides asynchronous ingestion pipelines for server-side data writes.
package ingest

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/eventx"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type ResultIngestor struct {
	store         *store.Store
	logger        *slog.Logger
	bus           eventx.BusRuntime
	queue         *resultQueue
	batchSize     int
	flushInterval time.Duration
	wake          chan struct{}
	stop          chan struct{}
	done          chan struct{}
	startOnce     sync.Once
	stopOnce      sync.Once
	started       atomic.Bool
	unsubscribe   func()
}

type probeResultReceivedEvent struct {
	params store.RecordProbeResultParams
}

func (e probeResultReceivedEvent) Name() string {
	return "orivis.probe.result.received"
}

func NewResultIngestor(
	cfg config.Config,
	storage *store.Store,
	logger *slog.Logger,
	bus eventx.BusRuntime,
) (*ResultIngestor, error) {
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
		return nil, wrapError(err, "parse ingest flush interval")
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
		bus:           bus,
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
		return wrapError(ErrClosed, "enqueue probe result")
	}
	if err := ctx.Err(); err != nil {
		return wrapError(err, "enqueue probe result")
	}
	if i.bus == nil {
		return wrapError(ErrClosed, "enqueue probe result")
	}
	event := probeResultReceivedEvent{params: cloneRecordProbeResultParams(params)}
	if err := i.bus.PublishAsync(context.WithoutCancel(ctx), event); err != nil {
		return wrapError(err, "publish probe result received event")
	}
	return nil
}

func (i *ResultIngestor) Start(ctx context.Context) error {
	if i == nil {
		return nil
	}
	var startErr error
	i.startOnce.Do(func() {
		unsubscribe, err := eventx.Subscribe[probeResultReceivedEvent](i.bus, i.handleProbeResultReceived)
		if err != nil {
			startErr = wrapError(err, "subscribe probe result received event")
			return
		}
		i.unsubscribe = unsubscribe
		i.started.Store(true)
		go i.run(ctx)
	})
	return startErr
}

func (i *ResultIngestor) Stop(ctx context.Context) error {
	if i == nil {
		return nil
	}
	if i.unsubscribe != nil {
		i.unsubscribe()
		i.unsubscribe = nil
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
		return wrapError(ctx.Err(), "stop result ingestor")
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

func (i *ResultIngestor) handleProbeResultReceived(ctx context.Context, event probeResultReceivedEvent) error {
	if err := ctx.Err(); err != nil {
		return wrapError(err, "handle probe result received event")
	}
	size, err := i.queue.push(event.params)
	if err != nil {
		return err
	}
	if size >= i.batchSize {
		i.notify()
	}
	return nil
}

func (i *ResultIngestor) flush(ctx context.Context) error {
	if i.store == nil || i.store.ResultStore() == nil {
		return newError("ingest: result store is not available")
	}

	var flushErr error
	for {
		done, err := i.flushNextBatch(ctx)
		if err != nil {
			flushErr = joinErrors(flushErr, err)
		}
		if done {
			return flushErr
		}
	}
}

func (i *ResultIngestor) flushNextBatch(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, wrapError(err, "flush result ingest queue")
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
			return wrapError(err, "record probe result batch")
		}
		i.logFlushError(wrapError(err, "record probe result batch"))
		return i.recordIndividually(ctx, batch)
	}
	return nil
}

func (i *ResultIngestor) recordIndividually(ctx context.Context, batch []store.RecordProbeResultParams) error {
	var batchErr error
	collectionlist.NewList(batch...).Range(func(index int, _ store.RecordProbeResultParams) bool {
		if _, err := i.store.ResultStore().Record(ctx, batch[index]); err != nil {
			batchErr = joinErrors(batchErr, err)
		}
		return true
	})
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
