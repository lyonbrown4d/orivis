package collector

import (
	"context"
	"log/slog"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

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

	if r.hasBufferedResults() {
		r.bufferResult(req, nil)
		r.flushBufferedResults(ctx)
		r.logTaskResult(ctx, task, result.Status, result.Latency)
		return
	}

	if err := r.client.ReportResult(ctx, req); err != nil {
		r.bufferResult(req, err)
		r.logTaskResult(ctx, task, result.Status, result.Latency)
		return
	}
	r.logTaskResult(ctx, task, result.Status, result.Latency)
}

func (r *Runner) logTaskResult(ctx context.Context, task protocol.AgentTask, status model.Status, latency time.Duration) {
	level := slog.LevelDebug
	if status != model.StatusUp {
		level = slog.LevelWarn
	}
	r.logger.Log(ctx, level, "agent task checked", "monitor_id", task.MonitorID, "status", status, "latency", latency)
}

func (r *Runner) hasBufferedResults() bool {
	return r.results != nil && r.results.Len() > 0
}

func (r *Runner) bufferResult(req protocol.AgentResultRequest, reportErr error) {
	if r.results == nil {
		if reportErr != nil {
			r.logger.Warn("agent result report failed", "monitor_id", req.MonitorID, "error", reportErr)
		}
		return
	}

	result := r.results.Push(req)
	if result.err != nil {
		if reportErr == nil {
			r.logger.Warn("agent result buffer write failed", "monitor_id", req.MonitorID, "error", result.err)
			return
		}
		r.logger.Warn("agent result report failed; buffer write failed", "monitor_id", req.MonitorID, "error", reportErr, "buffer_error", result.err)
		return
	}
	if !result.buffered {
		if reportErr == nil {
			r.logger.Debug("agent result dropped; buffer capacity is zero", "monitor_id", req.MonitorID, "buffer_size", result.size)
			return
		}
		r.logger.Warn("agent result report failed; buffer capacity is zero", "monitor_id", req.MonitorID, "buffer_size", result.size, "error", reportErr)
		return
	}
	if reportErr == nil {
		r.logger.Debug(
			"agent result queued behind buffered results",
			"monitor_id", req.MonitorID,
			"buffer_size", result.size,
			"dropped_oldest", result.droppedOldest,
		)
		return
	}
	r.logger.Warn(
		"agent result report failed; buffered for retry",
		"monitor_id", req.MonitorID,
		"buffer_size", result.size,
		"dropped_oldest", result.droppedOldest,
		"error", reportErr,
	)
}

func (r *Runner) flushBufferedResults(ctx context.Context) {
	if r.results == nil || r.agentID == "" {
		return
	}

	r.flushMu.Lock()
	defer r.flushMu.Unlock()

	flushed := 0
	for {
		req, ok := r.results.Peek()
		if !ok {
			break
		}
		req.AgentID = r.agentID
		req.Token = r.cfg.Agent.Token
		if err := r.client.ReportResult(ctx, req); err != nil {
			r.logger.Warn("agent buffered result flush failed", "flushed", flushed, "remaining", r.results.Len(), "error", err)
			return
		}
		if err := r.results.Drop(); err != nil {
			r.logger.Warn("agent buffered result drop failed", "flushed", flushed, "remaining", r.results.Len(), "error", err)
			return
		}
		flushed++
	}
	if flushed > 0 {
		r.logger.Info("agent buffered results flushed", "count", flushed)
	}
}
