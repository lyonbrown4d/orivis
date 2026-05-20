package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/go-co-op/gocron"
	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/probe"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

type Runner struct {
	cfg       config.Config
	logger    *slog.Logger
	client    *agentclient.Client
	checker   *probe.Checker
	discovery monitorDiscoverer
	agentID   string
	stop      context.CancelFunc
	sched     *gocron.Scheduler
	tasks     *collectionmapping.Map[string, scheduledTask]
	results   ResultQueue
	flushMu   sync.Mutex
}

type monitorDiscoverer interface {
	Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error)
	Close(ctx context.Context) error
}

type scheduledTask struct {
	signature string
}

func NewRunner(cfg config.Config, logger *slog.Logger, client *agentclient.Client) *Runner {
	runner := &Runner{
		cfg:     cfg,
		logger:  logger,
		client:  client,
		checker: probe.New(),
		tasks:   collectionmapping.NewMap[string, scheduledTask](),
	}
	if cfg.Buffer.Enabled {
		runner.results = newResultQueue(cfg.Buffer.Driver, cfg.Buffer.Path, cfg.Buffer.Capacity)
	}
	return runner
}

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
		"discovery_provider", r.cfg.Discovery.Provider,
		"docker_mode", r.cfg.Discovery.Docker.Mode,
		"buffer_enabled", r.cfg.Buffer.Enabled,
	)

	if err := r.configureDiscovery(); err != nil {
		return oops.Wrapf(err, "configure monitor discovery")
	}

	runCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	r.stop = stop

	scheduler := gocron.NewScheduler(time.UTC)
	if _, err := scheduler.Every(r.cfg.Poll.Interval).SingletonMode().Do(func() {
		r.syncTasks(runCtx)
	}); err != nil {
		stop()
		return oops.Wrapf(err, "schedule agent sync")
	}
	r.sched = scheduler
	scheduler.StartAsync()
	r.logger.Info("agent sync scheduler started", "interval", r.cfg.Poll.Interval)
	go r.syncTasks(runCtx)
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
	r.flushBufferedResults(ctx)
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
		r.logger.Info(
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
	for i := range monitors {
		monitor := &monitors[i]
		r.logger.Info(
			"agent discovered monitor",
			"source_key", monitor.SourceKey,
			"monitor_name", monitor.Name,
			"monitor_type", monitor.Type,
			"monitor_target", monitor.Target,
			"group_name", monitor.GroupName,
			"environment_code", monitor.EnvironmentCode,
			"interval_seconds", monitor.IntervalSeconds,
			"timeout_seconds", monitor.TimeoutSeconds,
			"retry_count", monitor.RetryCount,
		)
	}
	if len(monitors) == 0 {
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
		r.runTask(ctx, taskCopy)
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
