package collector

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/observabilityx"
)

type runnerMetrics struct {
	bufferLength         observabilityx.Gauge
	bufferDropped        observabilityx.Counter
	flushSize            observabilityx.Histogram
	flushSuccess         observabilityx.Counter
	flushFailure         observabilityx.Counter
	backoffSeconds       observabilityx.Histogram
	bufferCompaction      observabilityx.Counter
	bufferCompactionFails observabilityx.Counter
	bufferCompactionTime  observabilityx.Histogram
}

func newRunnerMetrics(obs observabilityx.Observability, logger *slog.Logger) runnerMetrics {
	obs = observabilityx.Normalize(obs, logger)
	return runnerMetrics{
		bufferLength: obs.Gauge(observabilityx.NewGaugeSpec(
			"agent_result_buffer_length",
			observabilityx.WithDescription("Current number of probe results waiting in the agent result buffer."),
			observabilityx.WithUnit("results"),
		)),
		bufferDropped: obs.Counter(observabilityx.NewCounterSpec(
			"agent_result_buffer_dropped_total",
			observabilityx.WithDescription("Total number of probe results dropped because the agent result buffer was full."),
			observabilityx.WithUnit("results"),
		)),
		flushSize: obs.Histogram(observabilityx.NewHistogramSpec(
			"agent_result_flush_batch_size",
			observabilityx.WithDescription("Number of probe results included in an agent result flush request."),
			observabilityx.WithUnit("results"),
		)),
		flushSuccess: obs.Counter(observabilityx.NewCounterSpec(
			"agent_result_flush_success_total",
			observabilityx.WithDescription("Total number of successful agent result flush requests."),
			observabilityx.WithUnit("requests"),
		)),
		flushFailure: obs.Counter(observabilityx.NewCounterSpec(
			"agent_result_flush_failure_total",
			observabilityx.WithDescription("Total number of failed agent result flush attempts."),
			observabilityx.WithUnit("requests"),
		)),
		backoffSeconds: obs.Histogram(observabilityx.NewHistogramSpec(
			"agent_result_flush_backoff_seconds",
			observabilityx.WithDescription("Backoff delay after agent result flush failures."),
			observabilityx.WithUnit("s"),
		)),
		bufferCompaction: obs.Counter(observabilityx.NewCounterSpec(
			"agent_result_buffer_compaction_total",
			observabilityx.WithDescription("Total number of agent result buffer compaction attempts."),
			observabilityx.WithUnit("operations"),
		)),
		bufferCompactionFails: obs.Counter(observabilityx.NewCounterSpec(
			"agent_result_buffer_compaction_failures_total",
			observabilityx.WithDescription("Total number of failed agent result buffer compactions."),
			observabilityx.WithUnit("operations"),
		)),
		bufferCompactionTime: obs.Histogram(observabilityx.NewHistogramSpec(
			"agent_result_buffer_compaction_duration_seconds",
			observabilityx.WithDescription("Duration of agent result buffer compaction attempts."),
			observabilityx.WithUnit("s"),
		)),
	}
}

func (m runnerMetrics) observeBufferLength(ctx context.Context, length int) {
	m.bufferLength.Set(ctx, float64(length))
}

func (m runnerMetrics) observeBufferDropped(ctx context.Context) {
	m.bufferDropped.Add(ctx, 1)
}

func (m runnerMetrics) observeFlushSuccess(ctx context.Context, size int) {
	m.flushSuccess.Add(ctx, 1)
	m.flushSize.Record(ctx, float64(size))
}

func (m runnerMetrics) observeFlushFailure(ctx context.Context, delay time.Duration) {
	m.flushFailure.Add(ctx, 1)
	m.backoffSeconds.Record(ctx, delay.Seconds())
}

func (m runnerMetrics) observeBufferCompaction(ctx context.Context, duration time.Duration) {
	m.bufferCompaction.Add(ctx, 1)
	m.bufferCompactionTime.Record(ctx, duration.Seconds())
}

func (m runnerMetrics) observeBufferCompactionFailure(ctx context.Context) {
	m.bufferCompactionFails.Add(ctx, 1)
}
