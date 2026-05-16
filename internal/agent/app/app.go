package app

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/go-co-op/gocron"
	agentclient "github.com/lyonbrown4d/orivis/internal/agent/client"
	"github.com/lyonbrown4d/orivis/internal/agent/config"
	"github.com/lyonbrown4d/orivis/internal/agent/discovery"
	"github.com/lyonbrown4d/orivis/internal/agent/probe"
	"github.com/lyonbrown4d/orivis/internal/shared/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/shared/model"
	"github.com/lyonbrown4d/orivis/internal/shared/observability"
	"github.com/lyonbrown4d/orivis/internal/shared/protocol"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	return New(cfg, logger).RunContext(ctx)
}

func New(cfg config.Config, logger *slog.Logger) *dix.App {
	observabilityModule := dix.NewModule("observability",
		dix.Providers(
			dix.Provider1(observability.NewNop),
		),
	)

	clientModule := dix.NewModule("client",
		dix.Imports(observabilityModule),
		dix.Providers(
			dix.Value(cfg),
			dix.Value(logger),
			dix.ProviderErr3(agentclient.New),
		),
		dix.Hooks(
			dix.OnStop[*agentclient.Client](func(ctx context.Context, client *agentclient.Client) error {
				return client.Close(ctx)
			}),
		),
	)

	runtimeModule := dix.NewModule("agent-runtime",
		dix.Imports(clientModule),
		dix.Providers(
			dix.Provider3(func(cfg config.Config, logger *slog.Logger, client *agentclient.Client) *Runtime {
				return &Runtime{cfg: cfg, logger: logger, client: client, checker: probe.New(), tasks: map[string]scheduledTask{}}
			}),
		),
		dix.Hooks(
			dix.OnStart[*Runtime](func(ctx context.Context, runtime *Runtime) error {
				return runtime.Start(ctx)
			}),
			dix.OnStop[*Runtime](func(ctx context.Context, runtime *Runtime) error {
				return runtime.Stop(ctx)
			}),
		),
	)

	return dix.New("orivis-agent",
		dix.UseProfile(profileFromEnv(cfg.Runtime)),
		dix.Version(buildinfo.Version),
		dix.UseLogger(logger),
		dix.Modules(runtimeModule),
	)
}

type Runtime struct {
	cfg       config.Config
	logger    *slog.Logger
	client    *agentclient.Client
	checker   *probe.Checker
	discovery monitorDiscoverer
	agentID   string
	stop      context.CancelFunc
	sched     *gocron.Scheduler
	tasks     map[string]scheduledTask
}

type monitorDiscoverer interface {
	Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error)
	Close(ctx context.Context) error
}

type scheduledTask struct {
	signature string
}

func (r *Runtime) Start(ctx context.Context) error {
	r.logger.Info(
		"starting agent",
		"name", r.cfg.Agent.Name,
		"region", r.cfg.Agent.Region,
		"runtime", r.cfg.Runtime,
		"server_url", r.cfg.Server.URL,
	)

	registration, err := r.client.Register(ctx, protocol.AgentRegisterRequest{
		Name:             r.cfg.Agent.Name,
		Token:            r.cfg.Agent.Token,
		RegionCode:       r.cfg.Agent.Region,
		EnvironmentCodes: r.cfg.Agent.Environments,
		RuntimeType:      r.cfg.Runtime,
		Version:          buildinfo.Version,
	})
	if err != nil {
		return err
	}
	r.agentID = registration.AgentID
	r.logger.Info("agent registered", "agent_id", r.agentID, "status", registration.Status)

	if err := r.configureDiscovery(); err != nil {
		return err
	}

	runCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	r.stop = stop

	scheduler := gocron.NewScheduler(time.UTC)
	if _, err := scheduler.Every(r.cfg.Poll.Interval).SingletonMode().Do(func() {
		r.syncTasks(runCtx)
	}); err != nil {
		stop()
		return err
	}
	r.sched = scheduler
	scheduler.StartAsync()
	return nil
}

func (r *Runtime) Stop(context.Context) error {
	if r.stop != nil {
		r.stop()
	}
	if r.sched != nil {
		r.sched.Stop()
	}
	if r.discovery != nil {
		if err := r.discovery.Close(context.Background()); err != nil {
			r.logger.Warn("close monitor discovery failed", "error", err)
		}
	}
	r.logger.Info("stopped agent")
	return nil
}

func (r *Runtime) syncTasks(ctx context.Context) {
	if err := ctx.Err(); err != nil {
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

func (r *Runtime) configureDiscovery() error {
	discoverers := make([]monitorDiscoverer, 0, 2)

	if r.cfg.Discovery.Static.Enabled && len(r.cfg.Discovery.Static.Monitors) > 0 {
		discoverers = append(discoverers, discovery.NewStaticDiscoverer(r.cfg.Discovery.Static.Monitors))
		r.logger.Info("static discovery enabled", "count", len(r.cfg.Discovery.Static.Monitors))
	}

	if r.cfg.Discovery.Docker.Enabled {
		discoverer, err := discovery.NewDockerDiscoverer(discovery.DockerOptions{
			Mode: r.cfg.Discovery.Docker.Mode,
		})
		if err != nil {
			return err
		}
		discoverers = append(discoverers, discoverer)
		r.logger.Info("Docker discovery enabled", "mode", r.cfg.Discovery.Docker.Mode)
	}

	switch len(discoverers) {
	case 0:
		return nil
	case 1:
		r.discovery = discoverers[0]
	default:
		r.discovery = compositeDiscoverer{discoverers: discoverers}
	}
	return nil
}

func (r *Runtime) heartbeat(ctx context.Context) error {
	response, err := r.client.Heartbeat(ctx, protocol.AgentHeartbeatRequest{
		AgentID: r.agentID,
		Token:   r.cfg.Agent.Token,
		Version: buildinfo.Version,
		SentAt:  time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	r.logger.Debug("agent heartbeat accepted", "agent_id", response.AgentID, "status", response.Status)
	return nil
}

func (r *Runtime) pullTasks(ctx context.Context) (protocol.AgentTasksResponse, error) {
	return r.client.Tasks(ctx, protocol.AgentTasksRequest{
		AgentID: r.agentID,
		Token:   r.cfg.Agent.Token,
	})
}

func (r *Runtime) syncDiscoveredMonitors(ctx context.Context) error {
	if r.discovery == nil {
		return nil
	}
	monitors, err := r.discovery.Discover(ctx)
	if err != nil {
		return err
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
		return err
	}
	r.logger.Debug("agent discovered monitors synced", "count", response.Synced)
	return nil
}

func (r *Runtime) reconcileTasks(ctx context.Context, tasks []protocol.AgentTask) {
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.MonitorID == "" {
			continue
		}
		seen[task.MonitorID] = struct{}{}
		signature := taskSignature(task)
		current, ok := r.tasks[task.MonitorID]
		if ok && current.signature == signature {
			continue
		}
		if ok {
			r.removeTask(task.MonitorID)
		}
		if err := r.scheduleTask(ctx, task, signature); err != nil {
			r.logger.Warn("schedule agent task failed", "monitor_id", task.MonitorID, "error", err)
		}
	}

	for monitorID := range r.tasks {
		if _, ok := seen[monitorID]; !ok {
			r.removeTask(monitorID)
		}
	}
}

func (r *Runtime) scheduleTask(ctx context.Context, task protocol.AgentTask, signature string) error {
	interval := taskInterval(task, r.cfg.Poll.Interval)
	taskCopy := task
	job, err := r.sched.Every(interval).SingletonMode().Do(func() {
		r.runTask(ctx, taskCopy)
	})
	if err != nil {
		return err
	}
	job.Tag(taskTag(task.MonitorID))
	r.tasks[task.MonitorID] = scheduledTask{signature: signature}
	r.logger.Info("agent task scheduled", "monitor_id", task.MonitorID, "interval", interval)
	return nil
}

func (r *Runtime) removeTask(monitorID string) {
	if err := r.sched.RemoveByTag(taskTag(monitorID)); err != nil {
		r.logger.Warn("remove agent task failed", "monitor_id", monitorID, "error", err)
	}
	delete(r.tasks, monitorID)
}

func (r *Runtime) runTask(ctx context.Context, task protocol.AgentTask) {
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

func taskInterval(task protocol.AgentTask, fallback time.Duration) time.Duration {
	if task.IntervalSeconds > 0 {
		return time.Duration(task.IntervalSeconds) * time.Second
	}
	if fallback > 0 {
		return fallback
	}
	return 30 * time.Second
}

func taskSignature(task protocol.AgentTask) string {
	return task.Type + "\x00" + task.Target + "\x00" + strconv.Itoa(task.IntervalSeconds) + "\x00" + strconv.Itoa(task.TimeoutSeconds)
}

func taskTag(monitorID string) string {
	return "monitor:" + monitorID
}

type compositeDiscoverer struct {
	discoverers []monitorDiscoverer
}

func (d compositeDiscoverer) Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	out := make([]protocol.AgentDiscoveredMonitor, 0)
	for _, discoverer := range d.discoverers {
		monitors, err := discoverer.Discover(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, monitors...)
	}
	return out, nil
}

func (d compositeDiscoverer) Close(ctx context.Context) error {
	for _, discoverer := range d.discoverers {
		if err := discoverer.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}

func profileFromEnv(runtime string) dix.Profile {
	if runtime == "test" {
		return dix.ProfileTest
	}
	return dix.ProfileDev
}
