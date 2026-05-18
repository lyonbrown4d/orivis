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

	"github.com/arcgolabs/configx"
	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/logx"
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
	cmd.Flags().Duration("poll-jitter", 0, "maximum initial probe schedule jitter")
	cmd.Flags().Bool("buffer-enabled", false, "enable bounded buffering for failed result reports")
	cmd.Flags().String("buffer-driver", "", "buffer driver: memory or file")
	cmd.Flags().String("buffer-path", "", "file buffer JSONL path")
	cmd.Flags().Int("buffer-capacity", 0, "maximum buffered failed result reports")
	cmd.Flags().StringSlice("discovery-static-hcl-files", nil, "static probe HCL files")
	cmd.Flags().String("discovery-provider", "", "discovery provider, for example docker")
	cmd.Flags().Bool("discovery-docker-enabled", false, "enable Docker label discovery (legacy)")
	cmd.Flags().String("discovery-docker-mode", "", "Docker discovery mode override: auto, container, or swarm")
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
			dix.ProviderErr0(func() (*agentconfig.Watcher, error) {
				return agentconfig.NewWatcherFromFlags(cmd.Flags(), configFile, configx.WithObservability(observability.NewBootstrap()))
			}),
			dix.Provider1(func(watcher *agentconfig.Watcher) agentconfig.Config {
				return watcher.Config()
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

	collectorModule := dix.NewModule("collector",
		dix.WithModuleImports(configModule, loggingModule, observabilityModule),
		dix.WithModuleProviders(
			dix.ProviderErr3(collector.NewRuntimeController),
		),
		dix.WithModuleHooks(
			dix.OnStart[*collector.RuntimeController](func(ctx context.Context, controller *collector.RuntimeController) error {
				return controller.Start(ctx)
			}, dix.LifecycleName("start-agent-collector")),
			dix.OnStop[*collector.RuntimeController](func(ctx context.Context, controller *collector.RuntimeController) error {
				return controller.Stop(ctx)
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
