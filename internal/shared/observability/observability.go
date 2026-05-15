package observability

import (
	"log/slog"

	"github.com/arcgolabs/observabilityx"
)

func NewNop(logger *slog.Logger) observabilityx.Observability {
	return observabilityx.NopWithLogger(logger)
}
