package observability

import (
	"log/slog"

	"github.com/arcgolabs/observabilityx"
	obsprom "github.com/arcgolabs/observabilityx/prometheus"
	"github.com/lyonbrown4d/orivis/internal/server/config"
)

func New(cfg config.Config, logger *slog.Logger) observabilityx.Observability {
	if cfg.Observability.Prometheus.Enabled {
		return obsprom.New(
			obsprom.WithLogger(logger),
			obsprom.WithNamespace(cfg.Observability.Prometheus.Namespace),
		)
	}

	return observabilityx.NopWithLogger(logger)
}
