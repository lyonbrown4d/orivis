package collector

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/observabilityx"
)

type runnerMetrics struct {
	bufferLength   observabilityx.Gauge
	bufferDropped  observabilityx.Counter
	flushSize      observabilityx.Histogram
	flushSuccess   observabilityx.Counter
	flushFailure   observabilityx.Counter
	backoffSeconds observabilityx.Histogram
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
