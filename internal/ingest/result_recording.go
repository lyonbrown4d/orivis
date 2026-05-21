package ingest

import (
	"context"

	"github.com/arcgolabs/collectionx/bytex"
	collectionlist "github.com/arcgolabs/collectionx/list"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func (i *ResultIngestor) recordBatch(ctx context.Context, batch *collectionlist.List[store.RecordProbeResultParams]) error {
	results, err := i.store.ResultStore().RecordBatch(ctx, batch.Values())
	if err != nil {
		if batch.Len() == 1 {
			return wrapError(err, "record probe result batch")
		}
		i.logFlushError(wrapError(err, "record probe result batch"))
		if i.logger != nil {
			i.logger.Warn("result batch record failed; falling back to individual writes", "count", batch.Len())
		}
		return i.recordIndividually(ctx, batch)
	}
	i.publishRecordedResults(ctx, results)
	i.invalidateDashboardSnapshotCache(ctx)
	return nil
}

func (i *ResultIngestor) recordIndividually(ctx context.Context, batch *collectionlist.List[store.RecordProbeResultParams]) error {
	var batchErr error
	results := collectionlist.NewListWithCapacity[model.ProbeResult](batch.Len())
	batch.Range(func(_ int, params store.RecordProbeResultParams) bool {
		result, err := i.store.ResultStore().Record(ctx, params)
		if err != nil {
			batchErr = joinErrors(batchErr, err)
			return true
		}
		results.Add(result)
		return true
	})
	i.publishRecordedResults(ctx, results.Values())
	if results.Len() > 0 {
		i.invalidateDashboardSnapshotCache(ctx)
	}
	return batchErr
}

func (i *ResultIngestor) publishRecordedResults(ctx context.Context, results []model.ProbeResult) {
	if i == nil || i.bus == nil || len(results) == 0 {
		return
	}
	if err := i.bus.PublishAsync(context.WithoutCancel(ctx), ProbeResultsRecordedEvent{Results: cloneProbeResults(results)}); err != nil {
		i.logFlushError(wrapError(err, "publish probe results recorded event"))
	}
}

func (i *ResultIngestor) invalidateDashboardSnapshotCache(ctx context.Context) {
	if i == nil || i.cache == nil {
		return
	}
	if err := i.cache.Delete(context.WithoutCancel(ctx), cachex.DashboardSnapshotKey(dashboardSnapshotResultLimit)); err != nil && i.logger != nil {
		i.logger.Warn("invalidate dashboard snapshot cache failed", "error", err)
	}
}

func cloneProbeResults(results []model.ProbeResult) []model.ProbeResult {
	out := make([]model.ProbeResult, len(results))
	copy(out, results)
	for index := range out {
		out[index].RawDetail = bytex.WrapList(out[index].RawDetail).Snapshot()
	}
	return out
}
