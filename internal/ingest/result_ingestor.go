// Package ingest provides asynchronous ingestion pipelines for server-side data writes.
package ingest

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arcgolabs/eventx"
	"github.com/arcgolabs/observabilityx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

const dashboardSnapshotResultLimit = 80

type ResultIngestor struct {
	store         *store.Store
	logger        *slog.Logger
	bus           eventx.BusRuntime
	cache         cachex.Store
	metrics       metrics
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

type ProbeResultsRecordedEvent struct {
	Results []model.ProbeResult
}

func (e ProbeResultsRecordedEvent) Name() string {
	return "orivis.probe.results.recorded"
}

func NewResultIngestor(
	cfg config.Config,
	storage *store.Store,
	logger *slog.Logger,
	bus eventx.BusRuntime,
	cacheStore cachex.Store,
	obs observabilityx.Observability,
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
		cache:         cacheStore,
		metrics:       newMetrics(obs, logger),
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
	return i.enqueue(ctx, params)
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

func (i *ResultIngestor) enqueue(ctx context.Context, params store.RecordProbeResultParams) error {
	if err := ctx.Err(); err != nil {
		return wrapError(err, "enqueue result ingest queue")
	}
	size, err := i.queue.push(cloneRecordProbeResultParams(params))
	if err != nil {
		if errors.Is(err, ErrQueueFull) {
			i.metrics.observeQueueFull(ctx, size)
		}
		return err
	}
	i.metrics.observeQueueLength(ctx, size)
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
	i.metrics.observeQueueLength(ctx, i.queue.len())
	if batch.Len() == 0 {
		return true, nil
	}
	start := time.Now()
	err := i.recordBatch(ctx, batch)
	i.metrics.observeFlushBatch(ctx, batch.Len(), time.Since(start), err)
	if err != nil {
		return batch.Len() < i.batchSize, err
	}
	return batch.Len() < i.batchSize, nil
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
