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
	"github.com/arcgolabs/eventx"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/logx"
	"github.com/lyonbrown4d/orivis/internal/api"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	"github.com/lyonbrown4d/orivis/internal/notification"
	baseobs "github.com/lyonbrown4d/orivis/internal/observability"
	"github.com/lyonbrown4d/orivis/internal/retention"
	"github.com/lyonbrown4d/orivis/internal/security"
	serverconfig "github.com/lyonbrown4d/orivis/internal/serverconfig"
	serverobs "github.com/lyonbrown4d/orivis/internal/serverobservability"
	"github.com/lyonbrown4d/orivis/internal/servicediscovery"
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

	registerServerFlags(cmd, &configFile)

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
				return serverconfig.LoadFromFlags(cmd.Flags(), configFile, configx.WithObservability(baseobs.NewBootstrap()))
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
			dix.ProviderErr2(store.OpenDB),
			dix.Provider1(store.NewRepositories),
			dix.Provider1(store.NewIDGenerator),
			dix.ProviderErr3(store.New),
		),
		dix.WithModuleHooks(
			dix.OnStop[*store.Store](func(ctx context.Context, storage *store.Store) error {
				return storage.Close(ctx)
			}, dix.LifecycleName("close-store"), dix.LifecycleAfter("stop-http-server", "stop-result-ingestor", "stop-retention")),
		),
	)

	eventModule := newServerEventModule(loggingModule)
	concurrencyModule := newServerConcurrencyModule(configModule, loggingModule)
	cacheModule := newServerCacheModule(configModule, loggingModule)
	observabilityModule := newServerObservabilityModule(configModule, loggingModule)
	ingestModule := newServerIngestModule(configModule, loggingModule, storeModule, eventModule, cacheModule, observabilityModule)
	notificationModule := newServerNotificationModule(configModule, loggingModule, eventModule, cacheModule, storeModule)
	retentionModule := newServerRetentionModule(configModule, loggingModule, storeModule)

	securityModule := newServerSecurityModule(configModule, loggingModule, observabilityModule)

	endpointModule := newServerEndpointModule(configModule, storeModule, ingestModule)

	httpModule := dix.NewModule("http",
		dix.WithModuleImports(configModule, loggingModule, storeModule, cacheModule, securityModule, observabilityModule, endpointModule),
		dix.WithModuleProviders(
			dix.Provider3(api.NewServerRuntimeDeps),
			dix.Provider5(api.NewServer),
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
	mdnsModule := newServerMDNSModule(configModule, loggingModule)

	return dix.New("orivis-server",
		dix.WithProfile(dix.ProfileDev),
		dix.WithVersion(buildinfo.Version),
		dix.WithAppDescription("distributed availability observability platform"),
		dix.WithRunStopTimeout(10*time.Second),
		dix.WithModules(httpModule, mdnsModule, retentionModule, notificationModule, concurrencyModule),
	)
}

func newServerMDNSModule(configModule, loggingModule dix.Module) dix.Module {
	return dix.NewModule("mdns",
		dix.WithModuleImports(configModule, loggingModule),
		dix.WithModuleProviders(
			dix.Provider2(func(cfg serverconfig.Config, logger *slog.Logger) *servicediscovery.MDNSAdvertiser {
				port := cfg.MDNS.Port
				if port <= 0 {
					port = servicediscovery.HTTPPortFromAddr(cfg.HTTP.Addr, 8080)
				}
				return servicediscovery.NewMDNSAdvertiser(servicediscovery.MDNSAdvertiseConfig{
					Enabled:  cfg.MDNS.Enabled,
					Service:  cfg.MDNS.Service,
					Domain:   cfg.MDNS.Domain,
					Instance: cfg.MDNS.Instance,
					Scheme:   cfg.MDNS.Scheme,
					Port:     port,
					Version:  buildinfo.Version,
				}, logger)
			}),
		),
		dix.WithModuleHooks(
			dix.OnStart[*servicediscovery.MDNSAdvertiser](func(ctx context.Context, advertiser *servicediscovery.MDNSAdvertiser) error {
				return advertiser.Start(ctx)
			}, dix.LifecycleName("start-mdns-discovery"), dix.LifecycleAfter("start-http-server")),
			dix.OnStop[*servicediscovery.MDNSAdvertiser](func(ctx context.Context, advertiser *servicediscovery.MDNSAdvertiser) error {
				return advertiser.Stop(ctx)
			}, dix.LifecycleName("stop-mdns-discovery"), dix.LifecycleBefore("stop-http-server")),
		),
	)
}

func newServerObservabilityModule(configModule, loggingModule dix.Module) dix.Module {
	return dix.NewModule("observability",
		dix.WithModuleImports(configModule, loggingModule),
		dix.WithModuleProviders(
			dix.Provider2(serverobs.New),
		),
	)
}

func newServerCacheModule(configModule, loggingModule dix.Module) dix.Module {
	return dix.NewModule("cache",
		dix.WithModuleImports(configModule, loggingModule),
		dix.WithModuleProviders(
			dix.ProviderErr2(cachex.NewStore),
		),
		dix.WithModuleHooks(
			dix.OnStop[cachex.Store](func(ctx context.Context, cacheStore cachex.Store) error {
				return cacheStore.Close(ctx)
			}, dix.LifecycleName("close-cache"), dix.LifecycleAfter("stop-http-server", "stop-notification")),
		),
	)
}

func newServerSecurityModule(configModule, loggingModule, observabilityModule dix.Module) dix.Module {
	return dix.NewModule("security",
		dix.WithModuleImports(configModule, loggingModule, observabilityModule),
		dix.WithModuleProviders(
			dix.Provider3(security.NewEngine),
		),
	)
}

func newServerRetentionModule(configModule, loggingModule, storeModule dix.Module) dix.Module {
	return dix.NewModule("retention",
		dix.WithModuleImports(configModule, loggingModule, storeModule),
		dix.WithModuleProviders(
			dix.ProviderErr3(retention.New),
		),
		dix.WithModuleHooks(
			dix.OnStart[*retention.Cleaner](func(ctx context.Context, cleaner *retention.Cleaner) error {
				return cleaner.Start(ctx)
			}, dix.LifecycleName("start-retention")),
			dix.OnStop[*retention.Cleaner](func(ctx context.Context, cleaner *retention.Cleaner) error {
				return cleaner.Stop(ctx)
			}, dix.LifecycleName("stop-retention"), dix.LifecycleBefore("close-store")),
		),
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
			}, dix.LifecycleName("close-event-bus"), dix.LifecycleAfter("stop-result-ingestor", "stop-notification")),
		),
	)
}

func newServerNotificationModule(configModule, loggingModule, eventModule, cacheModule, storeModule dix.Module) dix.Module {
	return dix.NewModule("notification",
		dix.WithModuleImports(configModule, loggingModule, eventModule, cacheModule, storeModule),
		dix.WithModuleProviders(
			dix.ProviderErr5(notification.NewManager),
		),
		dix.WithModuleHooks(
			dix.OnStart[*notification.Manager](func(ctx context.Context, manager *notification.Manager) error {
				return manager.Start(ctx)
			}, dix.LifecycleName("start-notification"), dix.LifecycleBefore("start-result-ingestor")),
			dix.OnStop[*notification.Manager](func(ctx context.Context, manager *notification.Manager) error {
				return manager.Stop(ctx)
			}, dix.LifecycleName("stop-notification"), dix.LifecycleBefore("close-event-bus")),
		),
	)
}

func newServerIngestModule(configModule, loggingModule, storeModule, eventModule, cacheModule, observabilityModule dix.Module) dix.Module {
	return dix.NewModule("ingest",
		dix.WithModuleImports(configModule, loggingModule, storeModule, eventModule, cacheModule, observabilityModule),
		dix.WithModuleProviders(
			dix.ProviderErr6(ingest.NewResultIngestor),
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
			dix.Contribute2[httpx.Endpoint, serverconfig.Config, *store.Store](api.NewMetadataEndpoint, dix.Order(10)),
			dix.Contribute0[httpx.Endpoint](api.NewHealthEndpoint, dix.Order(20)),
			dix.Contribute3[httpx.Endpoint, serverconfig.Config, *store.Store, *ingest.ResultIngestor](api.NewAgentEndpoint, dix.Order(30)),
		),
	)
}
