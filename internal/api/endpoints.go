package api

import (
	"log/slog"
	"time"

	"github.com/arcgolabs/authx"
	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/observabilityx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardEndpoint struct {
	cfg         config.Config
	store       *store.Store
	auth        *authx.Engine
	cache       cachex.Store
	snapshotTTL time.Duration
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
	metrics        agentEndpointMetrics
}

type AgentEndpointDeps struct {
	Obs            observabilityx.Observability
	ResultIngestor *ingest.ResultIngestor
}

func NewAgentEndpointDeps(obs observabilityx.Observability, resultIngestor *ingest.ResultIngestor) AgentEndpointDeps {
	return AgentEndpointDeps{Obs: obs, ResultIngestor: resultIngestor}
}

func NewMetadataEndpoint(cfg config.Config, storage *store.Store) httpx.Endpoint {
	return &metadataEndpoint{cfg: cfg, store: storage}
}

func NewHealthEndpoint() httpx.Endpoint {
	return &healthEndpoint{}
}

func NewAgentEndpoint(
	cfg config.Config,
	storage *store.Store,
	deps AgentEndpointDeps,
) httpx.Endpoint {
	return &agentEndpoint{
		cfg:            cfg,
		store:          storage,
		resultIngestor: deps.ResultIngestor,
		metrics:        newAgentEndpointMetrics(deps.Obs, slog.Default()),
	}
}

func NewDefaultEndpoints(
	cfg config.Config,
	storage *store.Store,
	obs observabilityx.Observability,
	resultIngestors ...*ingest.ResultIngestor,
) *collectionlist.List[httpx.Endpoint] {
	var resultIngestor *ingest.ResultIngestor
	if len(resultIngestors) > 0 {
		resultIngestor = resultIngestors[0]
	}
	deps := NewAgentEndpointDeps(obs, resultIngestor)
	return collectionlist.NewList(
		NewMetadataEndpoint(cfg, storage),
		NewHealthEndpoint(),
		NewAgentEndpoint(cfg, storage, deps),
	)
}
