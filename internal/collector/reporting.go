package collector

import (
	"context"
	"log/slog"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const defaultResultFlushBatchSize = 100

func (r *Runner) runTask(ctx context.Context, task protocol.AgentTask) {
	if err := ctx.Err(); err != nil {
		return
	}

	result := r.checker.Check(ctx, task)
	req := ensureAgentResultID(protocol.AgentResultRequest{
		AgentID:      r.agentID,
		Token:        r.cfg.Agent.Token,
		MonitorID:    task.MonitorID,
		Status:       string(result.Status),
		LatencyMS:    result.Latency.Milliseconds(),
		ErrorMessage: result.ErrorMessage,
		CheckedAt:    result.CheckedAt,
		RawDetail:    result.RawDetail,
	})

	if r.cfg.Buffer.Enabled {
		if !r.queueResult(ctx, req) {
			r.reportResultDirect(ctx, req)
		}
		r.logTaskResult(ctx, task, result.Status, result.Latency)
		return
	}

	r.reportResultDirect(ctx, req)
	r.logTaskResult(ctx, task, result.Status, result.Latency)
}

func (r *Runner) logTaskResult(ctx context.Context, task protocol.AgentTask, status model.Status, latency time.Duration) {
	level := slog.LevelDebug
	if status != model.StatusUp {
		level = slog.LevelWarn
	}
	r.logger.Log(
		ctx,
		level,
		"agent task checked",
		"monitor_id", task.MonitorID,
		"type", task.Type,
		"target", task.Target,
		"status", status,
		"latency", latency,
	)
}

func (r *Runner) queueResult(ctx context.Context, req protocol.AgentResultRequest) bool {
	if r.results == nil {
		return false
	}

	result := r.results.Push(req)
	if result.err != nil {
		r.logger.Warn("agent result buffer write failed", "monitor_id", req.MonitorID, "error", result.err)
		return false
	}
	r.metrics.observeBufferLength(ctx, result.size)
	if !result.buffered {
		r.logger.Debug("agent result dropped; buffer capacity is zero", "monitor_id", req.MonitorID, "buffer_size", result.size)
		return false
	}
	if result.droppedOldest {
		r.metrics.observeBufferDropped(ctx)
	}
	r.logger.Debug(
		"agent result queued for flush",
		"result_id", req.ResultID,
		"monitor_id", req.MonitorID,
		"buffer_size", result.size,
		"dropped_oldest", result.droppedOldest,
	)
	return true
}

func (r *Runner) reportResultDirect(ctx context.Context, req protocol.AgentResultRequest) {
	if err := r.client.ReportResult(ctx, req); err != nil {
		r.logger.Warn("agent result report failed", "monitor_id", req.MonitorID, "error", err)
	}
}

func (r *Runner) flushBufferedResults(ctx context.Context) {
	if r.results == nil || r.agentID == "" {
		return
	}

	r.flushMu.Lock()
	defer r.flushMu.Unlock()

	now := time.Now()
	if !r.flushBackoff.CanAttempt(now) {
		r.logger.Debug(
			"agent buffered result flush skipped by backoff",
			"remaining", r.results.Len(),
			"next_attempt", r.flushBackoff.nextAttempt,
		)
		return
	}

	batch, err := r.results.PeekBatch(resultFlushBatchSize(r.cfg.Buffer.FlushBatchSize, r.cfg.Buffer.Capacity))
	if err != nil {
		r.recordFlushFailure(ctx, now, "agent buffered result read failed", 0, err)
		return
	}
	if len(batch) == 0 {
		r.flushBackoff.Reset()
		return
	}

	req := protocol.AgentResultBatchRequest{
		AgentID: r.agentID,
		Token:   r.cfg.Agent.Token,
		Results: collectionlist.MapList(
			collectionlist.NewList(batch...),
			func(_ int, result protocol.AgentResultRequest) protocol.AgentResult {
				return protocol.AgentResultFromRequest(result)
			},
		).Values(),
	}
	resp, err := r.client.ReportResults(ctx, req)
	if err != nil {
		r.recordFlushFailure(ctx, now, "agent buffered result batch flush failed", len(batch), err)
		return
	}
	if err := r.results.DropBatch(len(batch)); err != nil {
		r.recordFlushFailure(ctx, now, "agent buffered result batch drop failed", len(batch), err)
		return
	}
	r.flushBackoff.Reset()
	r.metrics.observeFlushSuccess(ctx, resp.Accepted)
	r.metrics.observeBufferLength(ctx, r.results.Len())
	r.logger.Info("agent buffered results flushed", "count", resp.Accepted, "remaining", r.results.Len())
}

func (r *Runner) recordFlushFailure(ctx context.Context, now time.Time, message string, count int, err error) {
	delay := r.flushBackoff.RecordFailure(now)
	r.metrics.observeFlushFailure(ctx, delay)
	r.metrics.observeBufferLength(ctx, r.results.Len())
	r.logger.Warn(
		message,
		"count", count,
		"remaining", r.results.Len(),
		"retry_after", delay,
		"next_attempt", r.flushBackoff.nextAttempt,
		"error", err,
	)
}

func resultFlushBatchSize(configured, capacity int) int {
	size := configured
	if size <= 0 {
		size = defaultResultFlushBatchSize
	}
	if capacity > 0 {
		size = min(size, capacity)
	}
	return max(1, size)
}
