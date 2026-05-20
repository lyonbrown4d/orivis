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
	cfg                       config.Config
	logger                    *slog.Logger
	client                    *agentclient.Client
	checker                   *probe.Checker
	discovery                 MonitorDiscoverer
	agentID                   string
	stop                      context.CancelFunc
	taskPool                  *ants.Pool
	sched                     *gocron.Scheduler
	tasks                     *collectionmapping.Map[string, scheduledTask]
	results                   ResultQueue
	flushMu                   sync.Mutex
	discoverySignatureMu      sync.Mutex
	lastDiscoverySignature    string
	lastDiscoveryMonitorCount int
}

type MonitorDiscoverer interface {
	Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error)
	Close(ctx context.Context) error
}

type scheduledTask struct {
	signature string
}

func NewRunner(cfg config.Config, logger *slog.Logger, client *agentclient.Client, taskPool *ants.Pool, discoverer MonitorDiscoverer, results ResultQueue) *Runner {
	runner := &Runner{
		cfg:       cfg,
		logger:    logger,
		client:    client,
		taskPool:  taskPool,
		checker:   probe.New(),
		tasks:     collectionmapping.NewMap[string, scheduledTask](),
		discovery: discoverer,
		results:   results,
	}
	return runner
}
