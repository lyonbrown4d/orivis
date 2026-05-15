package security

import (
	"log/slog"

	"github.com/arcgolabs/authx"
	"github.com/arcgolabs/observabilityx"
	"github.com/lyonbrown4d/orivis/internal/server/config"
)

func NewEngine(cfg config.Config, logger *slog.Logger, obs observabilityx.Observability) *authx.Engine {
	_ = cfg
	obs = observabilityx.Normalize(obs, logger)

	return authx.NewEngine(
		authx.WithLogger(obs.Logger()),
		authx.WithAuthorizer(authx.RequireAnyRole("admin", "operator", "viewer")),
	)
}
