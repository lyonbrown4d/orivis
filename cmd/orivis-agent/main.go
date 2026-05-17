package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/logx"
	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	agentconfig "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/collector"
	"github.com/lyonbrown4d/orivis/internal/observability"
	"github.com/spf13/cobra"
)

func main() {
	var configFile string

	cmd := &cobra.Command{
		Use:           "orivis-agent",
		Short:         "Run the Orivis agent",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			app := newAgentApp(cmd, configFile)
			if err := app.RunContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("run agent app: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "config file path (YAML, JSON, TOML, or HCL)")
	cmd.Flags().String("server-url", "", "server base URL")
	cmd.Flags().String("agent-name", "", "agent name")
	cmd.Flags().String("agent-token", "", "agent token")
	cmd.Flags().String("agent-region", "", "agent region")
	cmd.Flags().StringSlice("agent-environments", nil, "agent environment codes")
	cmd.Flags().String("runtime", "", "agent runtime")
	cmd.Flags().Duration("poll-interval", 0, "task polling interval")
	cmd.Flags().StringSlice("discovery-static-hcl-files", nil, "static probe HCL files")
	cmd.Flags().Bool("discovery-docker-enabled", false, "enable Docker label discovery")
	cmd.Flags().String("discovery-docker-mode", "", "Docker discovery mode: container or swarm")
	cmd.Flags().String("log-level", "", "log level")

	if err := cmd.Execute(); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func newAgentApp(cmd *cobra.Command, configFile string) *dix.App {
	configModule := dix.NewModule("config",
		dix.WithModuleProviders(
			dix.ProviderErr0(func() (agentconfig.Config, error) {
				return agentconfig.LoadFromFlags(cmd.Flags(), configFile)
			}),
		),
	)

	loggingModule := dix.NewModule("logging",
		dix.WithModuleImports(configModule),
		dix.WithModuleProviders(
			dix.ProviderErr1(func(cfg agentconfig.Config) (*slog.Logger, error) {
				return logx.New(
					logx.WithConsole(true),
					logx.WithLevelString(cfg.Log.Level),
					logx.WithCaller(true),
				)
			}),
		),
		dix.WithModuleHooks(
			dix.OnStop[*slog.Logger](func(_ context.Context, logger *slog.Logger) error {
				return logx.Close(logger)
			}, dix.LifecycleName("close-agent-logger")),
		),
	)

	observabilityModule := dix.NewModule("observability",
		dix.WithModuleImports(loggingModule),
		dix.WithModuleProviders(
			dix.Provider1(observability.NewNop),
		),
	)

	clientModule := dix.NewModule("agent-client",
		dix.WithModuleImports(configModule, loggingModule, observabilityModule),
		dix.WithModuleProviders(
			dix.ProviderErr3(agentclient.New),
		),
		dix.WithModuleHooks(
			dix.OnStop[*agentclient.Client](func(ctx context.Context, client *agentclient.Client) error {
				return client.Close(ctx)
			}, dix.LifecycleName("close-agent-client"), dix.LifecycleAfter("stop-agent-collector")),
		),
	)

	collectorModule := dix.NewModule("collector",
		dix.WithModuleImports(configModule, loggingModule, clientModule),
		dix.WithModuleProviders(
			dix.Provider3(collector.NewRunner),
		),
		dix.WithModuleHooks(
			dix.OnStart[*collector.Runner](func(ctx context.Context, runner *collector.Runner) error {
				return runner.Start(ctx)
			}, dix.LifecycleName("start-agent-collector")),
			dix.OnStop[*collector.Runner](func(ctx context.Context, runner *collector.Runner) error {
				return runner.Stop(ctx)
			}, dix.LifecycleName("stop-agent-collector")),
		),
	)

	return dix.New("orivis-agent",
		dix.WithProfile(dix.ProfileDev),
		dix.WithVersion(buildinfo.Version),
		dix.WithRunStopTimeout(10*time.Second),
		dix.WithLifecycleConcurrency(4),
		dix.WithRecentEvents(256),
		dix.WithModules(collectorModule),
	)
}
