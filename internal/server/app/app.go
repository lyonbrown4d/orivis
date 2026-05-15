package app

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/lyonbrown4d/orivis/internal/server/api"
	"github.com/lyonbrown4d/orivis/internal/server/config"
	serverobs "github.com/lyonbrown4d/orivis/internal/server/observability"
	"github.com/lyonbrown4d/orivis/internal/server/security"
	"github.com/lyonbrown4d/orivis/internal/server/store"
	"github.com/lyonbrown4d/orivis/internal/shared/buildinfo"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	return New(cfg, logger).RunContext(ctx)
}

func New(cfg config.Config, logger *slog.Logger) *dix.App {
	configModule := dix.NewModule("config",
		dix.Providers(dix.Value(cfg)),
	)

	loggingModule := dix.NewModule("logging",
		dix.Providers(dix.Value(logger)),
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
			dix.OnStart2[config.Config, *api.Server](func(ctx context.Context, cfg config.Config, server *api.Server) error {
				return server.Start(ctx)
			}),
			dix.OnStop[*api.Server](func(ctx context.Context, server *api.Server) error {
				return server.Stop(ctx)
			}),
		),
	)

	return dix.New("orivis-server",
		dix.UseProfile(profileFromEnv(cfg.App.Env)),
		dix.Version(buildinfo.Version),
		dix.AppDescription("distributed availability observability platform"),
		dix.UseLogger(logger),
		dix.RunStopTimeout(10*time.Second),
		dix.Modules(httpModule),
	)
}

func profileFromEnv(env string) dix.Profile {
	switch strings.ToLower(env) {
	case "production", "prod":
		return dix.ProfileProd
	case "test":
		return dix.ProfileTest
	case "development", "dev", "":
		return dix.ProfileDev
	default:
		return dix.Profile(env)
	}
}
