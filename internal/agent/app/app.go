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
	cfg    config.Config
	logger *slog.Logger
	client *agentclient.Client
	stop   context.CancelFunc
	done   chan struct{}
}

func (r *Runtime) Start(ctx context.Context) error {
	r.logger.Info(
		"starting agent",
		"name", r.cfg.Agent.Name,
		"region", r.cfg.Agent.Region,
		"runtime", r.cfg.Runtime,
		"server_url", r.cfg.Server.URL,
	)

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
			r.logger.Debug("agent polling tasks")
		}
	}
}

func profileFromEnv(runtime string) dix.Profile {
	if runtime == "test" {
		return dix.ProfileTest
	}
	return dix.ProfileDev
}
