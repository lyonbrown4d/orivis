package collector

import (
	"context"
	"time"

	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

type compactingResultQueue interface {
	Compact(context.Context) (bool, error)
}

func (r *Runner) compactResultBuffer(ctx context.Context) {
	if r.results == nil {
		return
	}
	compact, ok := r.results.(compactingResultQueue)
	if !ok {
		return
	}

	start := time.Now()
	attempted, err := compact.Compact(ctx)
	if err != nil {
		r.metrics.observeBufferCompaction(ctx, time.Since(start))
		r.metrics.observeBufferCompactionFailure(ctx)
		r.logger.Warn("agent result buffer compaction failed", "error", err)
		return
	}
	if attempted {
		r.metrics.observeBufferCompaction(ctx, time.Since(start))
	}
}

func resultFlushInterval(interval time.Duration) time.Duration {
	if interval > 0 {
		return interval
	}
	return 30 * time.Second
}

func runnerResultBufferCompactionInterval() time.Duration {
	return time.Minute
}

func (r *Runner) ensureRegistered(ctx context.Context) bool {
	if r.agentID != "" {
		return true
	}

	registration, err := r.client.Register(ctx, protocol.AgentRegisterRequest{
		Name:             r.cfg.Agent.Name,
		Token:            r.cfg.Agent.Token,
		RegionCode:       r.cfg.Agent.Region,
		EnvironmentCodes: r.cfg.Agent.Environments,
		RuntimeType:      r.cfg.Runtime,
		Version:          buildinfo.Version,
	})
	if err != nil {
		r.logger.Warn("agent registration failed; will retry", "error", err)
		return false
	}
	r.agentID = registration.AgentID
	r.logger.Info("agent registered", "agent_id", r.agentID, "status", registration.Status)
	return true
}

func (r *Runner) heartbeat(ctx context.Context) error {
	response, err := r.client.Heartbeat(ctx, protocol.AgentHeartbeatRequest{
		AgentID: r.agentID,
		Token:   r.cfg.Agent.Token,
		Version: buildinfo.Version,
		SentAt:  time.Now().UTC(),
	})
	if err != nil {
		return oops.Wrapf(err, "send agent heartbeat")
	}
	r.logger.Debug("agent heartbeat accepted", "agent_id", response.AgentID, "status", response.Status)
	return nil
}

func (r *Runner) pullTasks(ctx context.Context) (protocol.AgentTasksResponse, error) {
	response, err := r.client.Tasks(ctx, protocol.AgentTasksRequest{
		AgentID: r.agentID,
		Token:   r.cfg.Agent.Token,
	})
	if err != nil {
		return protocol.AgentTasksResponse{}, oops.Wrapf(err, "pull agent tasks")
	}
	return response, nil
}

func (r *Runner) syncDiscoveredMonitors(ctx context.Context) error {
	if r.discovery == nil {
		return nil
	}
	monitors, err := r.discovery.Discover(ctx)
	if err != nil {
		return oops.Wrapf(err, "discover monitors")
	}
	r.logger.Debug("agent monitor discovery completed", "count", len(monitors))
	if len(monitors) == 0 {
		r.rememberDiscoverySignature("", 0)
		return nil
	}

	signature := discoverySignature(monitors)
	if !r.shouldSyncDiscovery(signature, len(monitors)) {
		r.logger.Debug("agent discovered monitors unchanged, skipping sync", "count", len(monitors), "signature", signature)
		return nil
	}

	response, err := r.client.SyncMonitors(ctx, protocol.AgentMonitorSyncRequest{
		AgentID:  r.agentID,
		Token:    r.cfg.Agent.Token,
		Monitors: monitors,
	})
	if err != nil {
		return oops.Wrapf(err, "sync discovered monitors")
	}
	r.rememberDiscoverySignature(signature, len(monitors))
	r.logger.Info("agent discovered monitors synced", "discovered", len(monitors), "synced", response.Synced)
	return nil
}

func (r *Runner) reconcileTasks(ctx context.Context, tasks []protocol.AgentTask) {
	seen := collectionset.NewSetWithCapacity[string](len(tasks))
	for _, task := range tasks {
		if task.MonitorID == "" {
			continue
		}
		seen.Add(task.MonitorID)
		r.reconcileTask(ctx, task)
	}

	r.removeMissingTasks(func(monitorID string) bool {
		return seen.Contains(monitorID)
	})
}

func (r *Runner) reconcileTask(ctx context.Context, task protocol.AgentTask) {
	signature := taskSignature(task)
	current, ok := r.tasks.Get(task.MonitorID)
	if ok && current.signature == signature {
		r.logger.Debug("agent task unchanged", "monitor_id", task.MonitorID)
		return
	}
	if ok {
		r.logger.Info("agent task signature changed, rescheduling", "monitor_id", task.MonitorID)
		r.removeTask(task.MonitorID)
	}
	if err := r.scheduleTask(ctx, task, signature); err != nil {
		r.logger.Warn("schedule agent task failed", "monitor_id", task.MonitorID, "error", err)
	}
}

func (r *Runner) removeMissingTasks(contains func(string) bool) {
	r.tasks.Range(func(monitorID string, _ scheduledTask) bool {
		if !contains(monitorID) {
			r.removeTask(monitorID)
		}
		return true
	})
}

func (r *Runner) scheduleTask(ctx context.Context, task protocol.AgentTask, signature string) error {
	interval := taskInterval(task, r.cfg.Poll.Interval)
	jitter := taskInitialJitter(task, r.cfg.Poll.Jitter, interval)
	taskCopy := task
	schedule := r.sched.Every(interval)
	if jitter > 0 {
		schedule = schedule.StartAt(time.Now().UTC().Add(jitter))
	}
	job, err := schedule.SingletonMode().Do(func() {
		if err := r.taskPool.Submit(func() { r.runTask(ctx, taskCopy) }); err != nil {
			r.logger.Warn("submit agent task failed", "monitor_id", task.MonitorID, "error", err)
		}
	})
	if err != nil {
		return oops.Wrapf(err, "schedule probe task")
	}
	job.Tag(taskTag(task.MonitorID))
	r.tasks.Set(task.MonitorID, scheduledTask{signature: signature})
	r.logger.Info("agent task scheduled", "monitor_id", task.MonitorID, "interval", interval, "initial_jitter", jitter)
	return nil
}

func (r *Runner) removeTask(monitorID string) {
	if err := r.sched.RemoveByTag(taskTag(monitorID)); err != nil {
		r.logger.Warn("remove agent task failed", "monitor_id", monitorID, "error", err)
	}
	r.logger.Info("agent task removed", "monitor_id", monitorID)
	r.tasks.Delete(monitorID)
}
