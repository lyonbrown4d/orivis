package api

import (
	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardEndpoint struct {
	cfg   config.Config
	store *store.Store
}

type metadataEndpoint struct {
	cfg   config.Config
	store *store.Store
}

type healthEndpoint struct{}

type agentEndpoint struct {
	cfg            config.Config
	store          *store.Store
	resultIngestor *ingest.ResultIngestor
}

func NewDashboardEndpoint(cfg config.Config, storage *store.Store) httpx.Endpoint {
	return &dashboardEndpoint{cfg: cfg, store: storage}
}

func NewMetadataEndpoint(cfg config.Config, storage *store.Store) httpx.Endpoint {
	return &metadataEndpoint{cfg: cfg, store: storage}
}

func NewHealthEndpoint() httpx.Endpoint {
	return &healthEndpoint{}
}

func NewAgentEndpoint(cfg config.Config, storage *store.Store, resultIngestor *ingest.ResultIngestor) httpx.Endpoint {
	return &agentEndpoint{cfg: cfg, store: storage, resultIngestor: resultIngestor}
}

func NewDefaultEndpoints(
	cfg config.Config,
	storage *store.Store,
	resultIngestors ...*ingest.ResultIngestor,
) *collectionlist.List[httpx.Endpoint] {
	var resultIngestor *ingest.ResultIngestor
	if len(resultIngestors) > 0 {
		resultIngestor = resultIngestors[0]
	}
	return collectionlist.NewList(
		NewDashboardEndpoint(cfg, storage),
		NewMetadataEndpoint(cfg, storage),
		NewHealthEndpoint(),
		NewAgentEndpoint(cfg, storage, resultIngestor),
	)
}
