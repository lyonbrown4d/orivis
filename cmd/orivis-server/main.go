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
	"github.com/arcgolabs/eventx"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/logx"
	"github.com/lyonbrown4d/orivis/internal/api"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	"github.com/lyonbrown4d/orivis/internal/security"
	serverconfig "github.com/lyonbrown4d/orivis/internal/serverconfig"
	serverobs "github.com/lyonbrown4d/orivis/internal/serverobservability"
	"github.com/lyonbrown4d/orivis/internal/store"
	"github.com/spf13/cobra"
)

func main() {
	var configFile string

	cmd := &cobra.Command{
		Use:           "orivis-server",
		Short:         "Run the Orivis server",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			app := newServerApp(cmd, configFile)
			if err := app.RunContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("run server app: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "config file path")
	cmd.Flags().String("app-env", "", "runtime environment")
	cmd.Flags().String("http-addr", "", "HTTP listen address")
	cmd.Flags().String("log-level", "", "log level")
	cmd.Flags().String("db-driver", "", "database driver")
	cmd.Flags().String("db-dsn", "", "database DSN")
	cmd.Flags().Int("ingest-queue-size", 0, "probe result ingest queue size")
	cmd.Flags().Int("ingest-batch-size", 0, "probe result ingest batch size")
	cmd.Flags().String("ingest-flush-interval", "", "probe result ingest flush interval")
	cmd.Flags().String("auth-agent-token", "", "agent shared token")
	cmd.Flags().Bool("auth-dashboard-enabled", false, "enable dashboard basic auth")
	cmd.Flags().String("auth-dashboard-username", "", "dashboard basic auth username")
	cmd.Flags().String("auth-dashboard-password", "", "dashboard basic auth password")
	cmd.Flags().Bool("observability-prometheus-enabled", false, "enable Prometheus observability adapter")
	cmd.Flags().String("observability-prometheus-namespace", "", "Prometheus metric namespace")

	if err := cmd.Execute(); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func newServerApp(cmd *cobra.Command, configFile string) *dix.App {
	configModule := dix.NewModule("config",
		dix.WithModuleProviders(
			dix.ProviderErr0(func() (serverconfig.Config, error) {
				return serverconfig.LoadFromFlags(cmd.Flags(), configFile)
			}),
		),
	)

	loggingModule := dix.NewModule("logging",
		dix.WithModuleImports(configModule),
		dix.WithModuleProviders(
			dix.ProviderErr1(func(cfg serverconfig.Config) (*slog.Logger, error) {
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
			}, dix.LifecycleName("close-server-logger")),
		),
	)

	storeModule := dix.NewModule("store",
		dix.WithModuleImports(configModule, loggingModule),
		dix.WithModuleProviders(
			dix.ProviderErr2(store.Open),
		),
		dix.WithModuleHooks(
			dix.OnStop[*store.Store](func(ctx context.Context, storage *store.Store) error {
				return storage.Close(ctx)
			}, dix.LifecycleName("close-store"), dix.LifecycleAfter("stop-http-server", "stop-result-ingestor")),
		),
	)

	eventModule := newServerEventModule(loggingModule)
	ingestModule := newServerIngestModule(configModule, loggingModule, storeModule, eventModule)

	observabilityModule := dix.NewModule("observability",
		dix.WithModuleImports(configModule, loggingModule),
		dix.WithModuleProviders(
			dix.Provider2(serverobs.New),
		),
	)

	securityModule := dix.NewModule("security",
		dix.WithModuleImports(configModule, loggingModule, observabilityModule),
		dix.WithModuleProviders(
			dix.Provider3(security.NewEngine),
		),
	)

	endpointModule := newServerEndpointModule(configModule, storeModule, ingestModule)

	httpModule := dix.NewModule("http",
		dix.WithModuleImports(configModule, loggingModule, storeModule, securityModule, observabilityModule, endpointModule),
		dix.WithModuleProviders(
			dix.Provider6(api.NewServer),
		),
		dix.WithModuleHooks(
			dix.OnStart[*api.Server](func(ctx context.Context, server *api.Server) error {
				return server.Start(ctx)
			}, dix.LifecycleName("start-http-server")),
			dix.OnStop[*api.Server](func(ctx context.Context, server *api.Server) error {
				return server.Stop(ctx)
			}, dix.LifecycleName("stop-http-server")),
		),
	)

	return dix.New("orivis-server",
		dix.WithProfile(dix.ProfileDev),
		dix.WithVersion(buildinfo.Version),
		dix.WithAppDescription("distributed availability observability platform"),
		dix.WithRunStopTimeout(10*time.Second),
		dix.WithLifecycleConcurrency(4),
		dix.WithModules(httpModule),
	)
}

func newServerEventModule(loggingModule dix.Module) dix.Module {
	return dix.NewModule("event",
		dix.WithModuleImports(loggingModule),
		dix.WithModuleProviders(
			dix.Provider1(func(logger *slog.Logger) eventx.BusRuntime {
				return eventx.New(
					eventx.WithParallelDispatch(true),
					eventx.WithAsyncErrorHandler(func(_ context.Context, event eventx.Event, err error) {
						logger.Error("handle async event failed", "event", event.Name(), "error", err)
					}),
				)
			}),
		),
		dix.WithModuleHooks(
			dix.OnStop[eventx.BusRuntime](func(_ context.Context, bus eventx.BusRuntime) error {
				return bus.Close()
			}, dix.LifecycleName("close-event-bus"), dix.LifecycleAfter("stop-result-ingestor")),
		),
	)
}

func newServerIngestModule(configModule, loggingModule, storeModule, eventModule dix.Module) dix.Module {
	return dix.NewModule("ingest",
		dix.WithModuleImports(configModule, loggingModule, storeModule, eventModule),
		dix.WithModuleProviders(
			dix.ProviderErr4(ingest.NewResultIngestor),
		),
		dix.WithModuleHooks(
			dix.OnStart[*ingest.ResultIngestor](func(ctx context.Context, resultIngestor *ingest.ResultIngestor) error {
				return resultIngestor.Start(ctx)
			}, dix.LifecycleName("start-result-ingestor")),
			dix.OnStop[*ingest.ResultIngestor](func(ctx context.Context, resultIngestor *ingest.ResultIngestor) error {
				return resultIngestor.Stop(ctx)
			}, dix.LifecycleName("stop-result-ingestor"), dix.LifecycleBefore("close-store", "close-event-bus")),
		),
	)
}

func newServerEndpointModule(configModule, storeModule, ingestModule dix.Module) dix.Module {
	return dix.NewModule("http-endpoints",
		dix.WithModuleImports(configModule, storeModule, ingestModule),
		dix.WithModuleProviders(
			dix.Contribute2[httpx.Endpoint, serverconfig.Config, *store.Store](api.NewDashboardEndpoint, dix.Order(10)),
			dix.Contribute2[httpx.Endpoint, serverconfig.Config, *store.Store](api.NewMetadataEndpoint, dix.Order(20)),
			dix.Contribute0[httpx.Endpoint](api.NewHealthEndpoint, dix.Order(30)),
			dix.Contribute3[httpx.Endpoint, serverconfig.Config, *store.Store, *ingest.ResultIngestor](api.NewAgentEndpoint, dix.Order(40)),
		),
	)
}
