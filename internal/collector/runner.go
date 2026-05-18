package collector

import (
	"context"
	"log/slog"
	"time"

	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/go-co-op/gocron"
	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/model"
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
}

type monitorDiscoverer interface {
	Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error)
	Close(ctx context.Context) error
}

type scheduledTask struct {
	signature string
}

func NewRunner(cfg config.Config, logger *slog.Logger, client *agentclient.Client) *Runner {
	return &Runner{
		cfg:     cfg,
		logger:  logger,
		client:  client,
		checker: probe.New(),
		tasks:   collectionmapping.NewMap[string, scheduledTask](),
	}
}

func (r *Runner) Start(ctx context.Context) error {
	r.logger.Info(
		"starting agent",
		"name", r.cfg.Agent.Name,
		"region", r.cfg.Agent.Region,
		"runtime", r.cfg.Runtime,
		"server_url", r.cfg.Server.URL,
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
	r.reconcileTasks(ctx, tasks.Tasks)
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
	r.logger.Debug("agent discovered monitors synced", "count", response.Synced)
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
		return
	}
	if ok {
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
	r.tasks.Delete(monitorID)
}

func (r *Runner) runTask(ctx context.Context, task protocol.AgentTask) {
	if err := ctx.Err(); err != nil {
		return
	}

	result := r.checker.Check(ctx, task)
	if err := r.client.ReportResult(ctx, protocol.AgentResultRequest{
		AgentID:      r.agentID,
		Token:        r.cfg.Agent.Token,
		MonitorID:    task.MonitorID,
		Status:       string(result.Status),
		LatencyMS:    result.Latency.Milliseconds(),
		ErrorMessage: result.ErrorMessage,
		CheckedAt:    result.CheckedAt,
		RawDetail:    result.RawDetail,
	}); err != nil {
		r.logger.Warn("agent result report failed", "monitor_id", task.MonitorID, "error", err)
		return
	}

	level := slog.LevelDebug
	if result.Status != model.StatusUp {
		level = slog.LevelWarn
	}
	r.logger.Log(ctx, level, "agent task checked", "monitor_id", task.MonitorID, "status", result.Status, "latency", result.Latency)
}
