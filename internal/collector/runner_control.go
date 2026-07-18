package collector

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/samber/oops"
)

var (
	errRunnerStartContextRequired      = errors.New("runner start context is required")
	errRunnerStopContextRequired       = errors.New("runner stop context is required")
	errRunnerTaskPoolNotInitializedYet = errors.New("runner task pool is not initialized")
)

func (r *Runner) Start(ctx context.Context) error {
	if ctx == nil {
		return oops.Wrapf(errRunnerStartContextRequired, "start agent")
	}

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

	runCtx, stop := context.WithCancel(ctx)
	runSeq, shouldStart, err := r.beginRunnerLifecycle(stop)
	if err != nil {
		stop()
		return err
	}
	if !shouldStart {
		stop()
		return nil
	}

	scheduler, err := r.buildScheduler(runCtx)
	if err != nil {
		r.endRunnerLifecycle(runSeq)
		stop()
		return err
	}
	if !r.commitRunnerScheduler(scheduler, runSeq) {
		r.endRunnerLifecycle(runSeq)
		stop()
		return nil
	}

	scheduler.StartAsync()
	r.logger.Info("agent sync scheduler started", "interval", r.cfg.Poll.Interval)

	go func() {
		defer r.runnerBackgroundWG.Done()
		r.syncTasks(runCtx)
	}()
	go func() {
		defer r.runnerBackgroundWG.Done()
		r.flushBufferedResults(runCtx)
	}()

	return nil
}

func (r *Runner) beginRunnerLifecycle(stop context.CancelFunc) (uint64, bool, error) {
	r.runnerStopMu.Lock()
	defer r.runnerStopMu.Unlock()

	if r.stop != nil || r.runnerStopping {
		return 0, false, nil
	}
	if r.taskPool == nil {
		return 0, false, oops.Wrapf(errRunnerTaskPoolNotInitializedYet, "start agent")
	}

	r.runnerRunSeq++
	runSeq := r.runnerRunSeq
	r.stop = stop
	r.sched = nil
	r.runnerStopping = false
	r.runnerShutdownOnce = sync.Once{}
	r.runnerBackgroundWG = sync.WaitGroup{}
	return runSeq, true, nil
}

func (r *Runner) buildScheduler(ctx context.Context) (*gocron.Scheduler, error) {
	scheduler := gocron.NewScheduler(time.UTC)
	if _, err := scheduler.Every(r.cfg.Poll.Interval).SingletonMode().Do(func() {
		r.syncTasks(ctx)
	}); err != nil {
		return nil, oops.Wrapf(err, "schedule agent sync")
	}
	if err := r.scheduleResultFlush(ctx, scheduler); err != nil {
		return nil, err
	}
	if err := r.scheduleResultBufferCompaction(ctx, scheduler); err != nil {
		return nil, err
	}
	return scheduler, nil
}

func (r *Runner) commitRunnerScheduler(scheduler *gocron.Scheduler, runSeq uint64) bool {
	r.runnerStopMu.Lock()
	defer r.runnerStopMu.Unlock()

	if r.runnerRunSeq != runSeq || r.stop == nil {
		return false
	}
	r.sched = scheduler
	r.runnerBackgroundWG.Add(2)
	return true
}

func (r *Runner) endRunnerLifecycle(runSeq uint64) {
	r.runnerStopMu.Lock()
	if r.runnerRunSeq == runSeq {
		r.stop = nil
		r.sched = nil
		r.runnerStopping = false
	}
	r.runnerStopMu.Unlock()
}

func (r *Runner) Stop(ctx context.Context) error {
	if ctx == nil {
		return oops.Wrapf(errRunnerStopContextRequired, "stop agent")
	}

	var scheduler *gocron.Scheduler
	var runSeq uint64
	var stop context.CancelFunc
	r.runnerStopMu.Lock()
	if r.stop == nil {
		r.runnerStopMu.Unlock()
		return nil
	}
	if r.runnerStopping {
		r.runnerStopMu.Unlock()
		return nil
	}

	stop = r.stop
	scheduler = r.sched
	runSeq = r.runnerRunSeq
	r.sched = nil
	r.runnerStopping = true
	r.runnerStopMu.Unlock()

	if scheduler != nil {
		scheduler.Stop()
	}
	stop()

	if err := r.waitForBackground(ctx); err != nil {
		r.finishRunnerStop(ctx, runSeq, false)
		return err
	}
	r.finishRunnerStop(ctx, runSeq, true)

	r.logger.Info("stopped agent")
	return nil
}

func (r *Runner) finishRunnerStop(ctx context.Context, runSeq uint64, shouldClose bool) {
	r.runnerStopMu.Lock()
	if r.runnerRunSeq != runSeq || !r.runnerStopping {
		r.runnerStopMu.Unlock()
		return
	}
	r.runnerStopping = false
	r.sched = nil
	r.stop = nil
	r.runnerStopMu.Unlock()

	if !shouldClose {
		return
	}
	r.runnerShutdownOnce.Do(func() {
		r.closeDiscovery(ctx)
		r.closeResultBuffer()
		r.closeChecker()
	})
}

func (r *Runner) waitForBackground(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.runnerBackgroundWG.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return oops.Wrapf(ctx.Err(), "runner shutdown")
	case <-done:
		return nil
	}
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
