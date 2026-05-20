package collector

import (
	"context"
	"log/slog"
	"sync"

	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/go-co-op/gocron"
	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/probe"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/panjf2000/ants/v2"
)

type Runner struct {
	cfg       config.Config
	logger    *slog.Logger
	client    *agentclient.Client
	checker   *probe.Checker
	discovery monitorDiscoverer
	agentID   string
	stop      context.CancelFunc
	taskPool  *ants.Pool
	sched     *gocron.Scheduler
	tasks     *collectionmapping.Map[string, scheduledTask]
	results   ResultQueue
	flushMu   sync.Mutex
}

type monitorDiscoverer interface {
	Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error)
	Close(ctx context.Context) error
}

type scheduledTask struct {
	signature string
}

func NewRunner(cfg config.Config, logger *slog.Logger, client *agentclient.Client, taskPool *ants.Pool) *Runner {
	runner := &Runner{
		cfg:      cfg,
		logger:   logger,
		client:   client,
		taskPool: taskPool,
		checker:  probe.New(),
		tasks:    collectionmapping.NewMap[string, scheduledTask](),
	}
	if cfg.Buffer.Enabled {
		runner.results = newResultQueue(cfg.Buffer.Driver, cfg.Buffer.Path, cfg.Buffer.Capacity)
	}
	return runner
}
