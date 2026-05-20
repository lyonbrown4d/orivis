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
	req := protocol.AgentResultRequest{
		AgentID:      r.agentID,
		Token:        r.cfg.Agent.Token,
		MonitorID:    task.MonitorID,
		Status:       string(result.Status),
		LatencyMS:    result.Latency.Milliseconds(),
		ErrorMessage: result.ErrorMessage,
		CheckedAt:    result.CheckedAt,
		RawDetail:    result.RawDetail,
	}

	if r.cfg.Buffer.Enabled {
		if !r.queueResult(req) {
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

func (r *Runner) queueResult(req protocol.AgentResultRequest) bool {
	if r.results == nil {
		return false
	}

	result := r.results.Push(req)
	if result.err != nil {
		r.logger.Warn("agent result buffer write failed", "monitor_id", req.MonitorID, "error", result.err)
		return false
	}
	if !result.buffered {
		r.logger.Debug("agent result dropped; buffer capacity is zero", "monitor_id", req.MonitorID, "buffer_size", result.size)
		return false
	}
	r.logger.Debug(
		"agent result queued for flush",
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

	batch, err := r.results.PeekBatch(defaultResultFlushBatchSize)
	if err != nil {
		r.logger.Warn("agent buffered result read failed", "remaining", r.results.Len(), "error", err)
		return
	}
	if len(batch) == 0 {
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
		r.logger.Warn("agent buffered result batch flush failed", "count", len(batch), "remaining", r.results.Len(), "error", err)
		return
	}
	if err := r.results.DropBatch(len(batch)); err != nil {
		r.logger.Warn("agent buffered result batch drop failed", "count", len(batch), "remaining", r.results.Len(), "error", err)
		return
	}
	r.logger.Info("agent buffered results flushed", "count", resp.Accepted, "remaining", r.results.Len())
}
