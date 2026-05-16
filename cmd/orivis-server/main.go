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
	"github.com/lyonbrown4d/orivis/internal/api"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
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
				return err
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
	cmd.Flags().String("auth-agent-token", "", "agent shared token")
	cmd.Flags().Bool("auth-dashboard-enabled", false, "enable dashboard basic auth")
	cmd.Flags().String("auth-dashboard-username", "", "dashboard basic auth username")
	cmd.Flags().String("auth-dashboard-password", "", "dashboard basic auth password")
	cmd.Flags().Bool("observability-prometheus-enabled", false, "enable Prometheus observability adapter")
	cmd.Flags().String("observability-prometheus-namespace", "", "Prometheus metric namespace")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newServerApp(cmd *cobra.Command, configFile string) *dix.App {
	configModule := dix.NewModule("config",
		dix.Providers(
			dix.ProviderErr0(func() (serverconfig.Config, error) {
				return serverconfig.LoadFromFlags(cmd.Flags(), configFile)
			}),
		),
	)

	loggingModule := dix.NewModule("logging",
		dix.Imports(configModule),
		dix.Providers(
			dix.ProviderErr1(func(cfg serverconfig.Config) (*slog.Logger, error) {
				return logx.New(
					logx.WithConsole(true),
					logx.WithLevelString(cfg.Log.Level),
					logx.WithCaller(true),
				)
			}),
		),
		dix.Hooks(
			dix.OnStop[*slog.Logger](func(_ context.Context, logger *slog.Logger) error {
				return logx.Close(logger)
			}),
		),
	)

	storeModule := dix.NewModule("store",
		dix.Imports(configModule, loggingModule),
		dix.Providers(
			dix.ProviderErr2(store.Open),
		),
		dix.Hooks(
			dix.OnStop[*store.Store](func(ctx context.Context, storage *store.Store) error {
				return storage.Close(ctx)
			}),
		),
	)

	observabilityModule := dix.NewModule("observability",
		dix.Imports(configModule, loggingModule),
		dix.Providers(
			dix.Provider2(serverobs.New),
		),
	)

	securityModule := dix.NewModule("security",
		dix.Imports(configModule, loggingModule, observabilityModule),
		dix.Providers(
			dix.Provider3(security.NewEngine),
		),
	)

	httpModule := dix.NewModule("http",
		dix.Imports(configModule, loggingModule, storeModule, securityModule, observabilityModule),
		dix.Providers(
			dix.Provider5(api.NewServer),
		),
		dix.Hooks(
			dix.OnStart[*api.Server](func(ctx context.Context, server *api.Server) error {
				return server.Start(ctx)
			}),
			dix.OnStop[*api.Server](func(ctx context.Context, server *api.Server) error {
				return server.Stop(ctx)
			}),
		),
	)

	return dix.New("orivis-server",
		dix.UseProfile(dix.ProfileDev),
		dix.Version(buildinfo.Version),
		dix.AppDescription("distributed availability observability platform"),
		dix.RunStopTimeout(10*time.Second),
		dix.Modules(httpModule),
	)
}
