package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/dix"
	agentclient "github.com/lyonbrown4d/orivis/internal/agent/client"
	"github.com/lyonbrown4d/orivis/internal/agent/config"
	"github.com/lyonbrown4d/orivis/internal/shared/buildinfo"
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
				return &Runtime{cfg: cfg, logger: logger, client: client}
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
	cfg     config.Config
	logger  *slog.Logger
	client  *agentclient.Client
	agentID string
	stop    context.CancelFunc
	done    chan struct{}
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

	runCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	r.stop = stop
	r.done = make(chan struct{})

	go r.loop(runCtx)
	return nil
}

func (r *Runtime) Stop(context.Context) error {
	if r.stop != nil {
		r.stop()
	}
	if r.done != nil {
		<-r.done
	}
	r.logger.Info("stopped agent")
	return nil
}

func (r *Runtime) loop(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(r.cfg.Poll.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("stopping agent")
			return
		case <-ticker.C:
			if err := r.heartbeat(ctx); err != nil {
				r.logger.Warn("agent heartbeat failed", "error", err)
				continue
			}
			tasks, err := r.pullTasks(ctx)
			if err != nil {
				r.logger.Warn("agent task pull failed", "error", err)
				continue
			}
			r.logger.Debug("agent tasks pulled", "count", len(tasks.Tasks))
		}
	}
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

func profileFromEnv(runtime string) dix.Profile {
	if runtime == "test" {
		return dix.ProfileTest
	}
	return dix.ProfileDev
}
