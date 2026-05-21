package api

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/observabilityx"
)

type agentEndpointMetrics struct {
	batchRequestSize observabilityx.Histogram
	batchAccepted    observabilityx.Histogram
	batchDuplicates  observabilityx.Histogram
	batchRejected    observabilityx.Counter
}

func newAgentEndpointMetrics(obs observabilityx.Observability, logger *slog.Logger) agentEndpointMetrics {
	obs = observabilityx.Normalize(obs, logger)
	return agentEndpointMetrics{
		batchRequestSize: obs.Histogram(observabilityx.NewHistogramSpec(
			"agent_result_batch_request_size",
			observabilityx.WithDescription("Number of probe results submitted by agents in a batch request."),
			observabilityx.WithUnit("results"),
		)),
		batchAccepted: obs.Histogram(observabilityx.NewHistogramSpec(
			"agent_result_batch_accepted",
			observabilityx.WithDescription("Number of probe results accepted from an agent batch request."),
			observabilityx.WithUnit("results"),
		)),
		batchDuplicates: obs.Histogram(observabilityx.NewHistogramSpec(
			"agent_result_batch_duplicates",
			observabilityx.WithDescription("Number of duplicate probe results detected in an agent batch request."),
			observabilityx.WithUnit("results"),
		)),
		batchRejected: obs.Counter(observabilityx.NewCounterSpec(
			"agent_result_batch_rejected_total",
			observabilityx.WithDescription("Total number of oversized agent result batch requests rejected."),
			observabilityx.WithUnit("requests"),
		)),
	}
}

func (m agentEndpointMetrics) observeBatch(ctx context.Context, requested, accepted int) {
	m.batchRequestSize.Record(ctx, float64(requested))
	m.batchAccepted.Record(ctx, float64(accepted))
	m.batchDuplicates.Record(ctx, float64(max(0, requested-accepted)))
}

func (m agentEndpointMetrics) observeBatchRejected(ctx context.Context) {
	m.batchRejected.Add(ctx, 1)
}
