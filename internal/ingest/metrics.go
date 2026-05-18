package ingest

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/observabilityx"
)

type metrics struct {
	queueLength  observabilityx.Gauge
	queueFull    observabilityx.Counter
	flushSize    observabilityx.Histogram
	flushSeconds observabilityx.Histogram
	recordErrors observabilityx.Counter
}

func newMetrics(obs observabilityx.Observability, logger *slog.Logger) metrics {
	obs = observabilityx.Normalize(obs, logger)
	return metrics{
		queueLength: obs.Gauge(observabilityx.NewGaugeSpec(
			"ingest_queue_length",
			observabilityx.WithDescription("Current number of probe results waiting in the ingest queue."),
			observabilityx.WithUnit("results"),
		)),
		queueFull: obs.Counter(observabilityx.NewCounterSpec(
			"ingest_queue_full_total",
			observabilityx.WithDescription("Total number of probe results rejected because the ingest queue was full."),
			observabilityx.WithUnit("results"),
		)),
		flushSize: obs.Histogram(observabilityx.NewHistogramSpec(
			"ingest_flush_batch_size",
			observabilityx.WithDescription("Number of probe results handled in an ingest flush batch."),
			observabilityx.WithUnit("results"),
		)),
		flushSeconds: obs.Histogram(observabilityx.NewHistogramSpec(
			"ingest_flush_duration_seconds",
			observabilityx.WithDescription("Duration of a probe result ingest flush batch."),
			observabilityx.WithUnit("s"),
		)),
		recordErrors: obs.Counter(observabilityx.NewCounterSpec(
			"ingest_record_errors_total",
			observabilityx.WithDescription("Total number of probe result record errors."),
			observabilityx.WithUnit("errors"),
		)),
	}
}

func (m metrics) observeQueueLength(ctx context.Context, length int) {
	m.queueLength.Set(ctx, float64(length))
}

func (m metrics) observeQueueFull(ctx context.Context, length int) {
	m.queueFull.Add(ctx, 1)
	m.observeQueueLength(ctx, length)
}

func (m metrics) observeFlushBatch(ctx context.Context, size int, duration time.Duration, err error) {
	m.flushSize.Record(ctx, float64(size))
	m.flushSeconds.Record(ctx, duration.Seconds())
	if err != nil {
		m.recordErrors.Add(ctx, 1)
	}
}
