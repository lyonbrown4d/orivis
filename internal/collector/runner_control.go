package collector

import (
	"context"
	"fmt"
	"time"

	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/go-co-op/gocron"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

func (r *Runner) Start(ctx context.Context) error {
	r.logger.Info(
		"starting agent",
		"name", r.cfg.Agent.Name,
		"region", r.cfg.Agent.Region,
		"runtime", r.cfg.Runtime,
		"server_url", r.cfg.Server.URL,
		"log_level", r.cfg.Log.Level,
		"poll_interval", r.cfg.Poll.Interval,
		"poll_jitter", r.cfg.Poll.Jitter,
		"poll_workers", r.cfg.Poll.Workers,
		"discovery_provider", r.cfg.Discovery.Provider,
		"docker_mode", r.cfg.Discovery.Docker.Mode,
		"buffer_enabled", r.cfg.Buffer.Enabled,
	)

	runCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	r.stop = stop

	if r.taskPool == nil {
		return fmt.Errorf("%w", oops.New("runner task pool is not initialized"))
	}

	scheduler := gocron.NewScheduler(time.UTC)
	if _, err := scheduler.Every(r.cfg.Poll.Interval).SingletonMode().Do(func() {
		r.syncTasks(runCtx)
	}); err != nil {
		stop()
		return oops.Wrapf(err, "schedule agent sync")
	}
	if err := r.scheduleResultFlush(runCtx, scheduler); err != nil {
		stop()
		return err
	}
	r.sched = scheduler
	scheduler.StartAsync()
	r.logger.Info("agent sync scheduler started", "interval", r.cfg.Poll.Interval)
	go r.syncTasks(runCtx)
	go r.flushBufferedResults(runCtx)
	return nil
}

func (r *Runner) Stop(ctx context.Context) error {
	if r.stop != nil {
		r.stop()
	}
	if r.sched != nil {
		r.sched.Stop()
	}
	if r.discovery != nil {
		if err := r.discovery.Close(ctx); err != nil {
			r.logger.Warn("close monitor discovery failed", "error", err)
		}
	}
	if r.results != nil {
		if err := r.results.Close(); err != nil {
			r.logger.Warn("close result buffer failed", "error", err)
		}
	}
	r.logger.Info("stopped agent")
	return nil
}

func (r *Runner) syncTasks(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	start := time.Now()
	if !r.ensureRegistered(ctx) {
		return
	}
	if err := r.heartbeat(ctx); err != nil {
		r.logger.Warn("agent heartbeat failed", "error", err)
		return
	}
	if err := r.syncDiscoveredMonitors(ctx); err != nil {
		r.logger.Warn("agent monitor discovery sync failed", "error", err)
	}
	tasks, err := r.pullTasks(ctx)
	if err != nil {
		r.logger.Warn("agent task pull failed", "error", err)
		return
	}
	r.logger.Debug("agent tasks pulled", "count", len(tasks.Tasks))
	for i := range tasks.Tasks {
		task := tasks.Tasks[i]
		r.logger.Debug(
			"agent pulled task",
			"task_id", task.ID,
			"monitor_id", task.MonitorID,
			"monitor_type", task.Type,
			"monitor_target", task.Target,
			"interval_seconds", task.IntervalSeconds,
			"timeout_seconds", task.TimeoutSeconds,
		)
	}
	r.reconcileTasks(ctx, tasks.Tasks)
	r.logger.Debug("agent sync cycle completed", "duration", time.Since(start), "task_count", len(tasks.Tasks))
}

func (r *Runner) scheduleResultFlush(ctx context.Context, scheduler *gocron.Scheduler) error {
	if !r.cfg.Buffer.Enabled {
		return nil
	}
	interval := resultFlushInterval(r.cfg.Poll.Interval)
	if _, err := scheduler.Every(interval).SingletonMode().Do(func() {
		r.flushBufferedResults(ctx)
	}); err != nil {
		return oops.Wrapf(err, "schedule agent result flush")
	}
	r.logger.Info("agent result flush scheduler started", "interval", interval)
	return nil
}

func resultFlushInterval(interval time.Duration) time.Duration {
	if interval > 0 {
		return interval
	}
	return 30 * time.Second
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
