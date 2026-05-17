package observability

import (
	"log/slog"

	"github.com/arcgolabs/observabilityx"
)

func NewBootstrap() observabilityx.Observability {
	return observabilityx.NopWithLogger(slog.Default())
}

func NewNop(logger *slog.Logger) observabilityx.Observability {
	return observabilityx.NopWithLogger(logger)
}
