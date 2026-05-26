package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/go-co-op/gocron"
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
	if err := r.scheduleResultBufferCompaction(runCtx, scheduler); err != nil {
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
	r.closeDiscovery(ctx)
	r.closeResultBuffer()
	r.closeChecker()
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

func (r *Runner) scheduleResultBufferCompaction(ctx context.Context, scheduler *gocron.Scheduler) error {
	if !r.cfg.Buffer.Enabled {
		return nil
	}
	interval := runnerResultBufferCompactionInterval()
	if _, err := scheduler.Every(interval).SingletonMode().Do(func() {
		r.compactResultBuffer(ctx)
	}); err != nil {
		return oops.Wrapf(err, "schedule agent result buffer compaction")
	}
	r.logger.Info("agent result buffer compaction scheduler started", "interval", interval)
	return nil
}
