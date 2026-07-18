package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/arcgolabs/observabilityx"
	agentclient "github.com/lyonbrown4d/orivis/internal/agentclient"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/servicediscovery"
	"github.com/panjf2000/ants/v2"
	"github.com/samber/oops"
)

type RuntimeController struct {
	watcher  *config.Watcher
	logger   *slog.Logger
	obs      observabilityx.Observability
	taskPool *ants.Pool
	deps     RuntimeControllerDeps

	mu      sync.Mutex
	cancel  context.CancelFunc
	started bool
	runSeq  uint64
	runtime *runtimeInstance
}

var (
	errRuntimeControllerStartContextRequired = errors.New("runtime controller start context is required")
	errRuntimeControllerStopContextRequired  = errors.New("runtime controller stop context is required")
)

type runtimeInstance struct {
	client    *agentclient.Client
	runner    *Runner
	serverURL string
}

type RuntimeControllerDeps struct {
	NewClient      func(config.Config) (*agentclient.Client, error)
	NewDiscoverer  func(config.Config) (MonitorDiscoverer, error)
	NewResultQueue func(context.Context, config.Config) (ResultQueue, error)
}

func NewRuntimeControllerDeps(logger *slog.Logger, obs observabilityx.Observability) RuntimeControllerDeps {
	obs = observabilityx.Normalize(obs, logger)
	return RuntimeControllerDeps{
		NewClient: func(cfg config.Config) (*agentclient.Client, error) {
			return agentclient.New(cfg, logger, obs)
		},
		NewDiscoverer: func(cfg config.Config) (MonitorDiscoverer, error) {
			return NewMonitorDiscoverer(cfg, logger)
		},
		NewResultQueue: NewResultQueue,
	}
}

func NewRuntimeController(
	watcher *config.Watcher,
	logger *slog.Logger,
	obs observabilityx.Observability,
	taskPool *ants.Pool,
	deps RuntimeControllerDeps,
) (*RuntimeController, error) {
	if watcher == nil {
		return nil, errors.New("agent config watcher is required")
	}
	if taskPool == nil {
		return nil, errors.New("agent task pool is required")
	}
	obs = observabilityx.Normalize(obs, logger)
	deps = normalizeRuntimeControllerDeps(deps, logger, obs)
	return &RuntimeController{
		watcher:  watcher,
		logger:   logger,
		obs:      obs,
		taskPool: taskPool,
		deps:     deps,
	}, nil
}

func normalizeRuntimeControllerDeps(
	deps RuntimeControllerDeps,
	logger *slog.Logger,
	obs observabilityx.Observability,
) RuntimeControllerDeps {
	defaults := NewRuntimeControllerDeps(logger, obs)
	if deps.NewClient == nil {
		deps.NewClient = defaults.NewClient
	}
	if deps.NewDiscoverer == nil {
		deps.NewDiscoverer = defaults.NewDiscoverer
	}
	if deps.NewResultQueue == nil {
		deps.NewResultQueue = defaults.NewResultQueue
	}
	return deps
}

func (c *RuntimeController) Start(ctx context.Context) error {
	if ctx == nil {
		return oops.Wrapf(errRuntimeControllerStartContextRequired, "start runtime controller")
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	runSeq, shouldStart := c.beginRuntimeStart(cancel)
	if !shouldStart {
		cancel()
		return nil
	}

	if err := c.reload(runCtx, runSeq, c.watcher.Config()); err != nil {
		c.rollbackRuntimeStart(runSeq, cancel)
		return err
	}

	c.watchConfigChanges(runCtx, runSeq)

	go c.watch(runCtx)
	return nil
}

func (c *RuntimeController) Stop(ctx context.Context) error {
	if ctx == nil {
		return oops.Wrapf(errRuntimeControllerStopContextRequired, "stop runtime controller")
	}

	c.mu.Lock()
	runtime := c.runtime
	cancel := c.cancel
	c.cancel = nil
	c.started = false
	c.runSeq++
	c.runtime = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	if runtime == nil {
		if err := c.watcher.Close(); err != nil {
			c.logger.Warn("close agent config watcher failed", "error", err)
		}
		return nil
	}

	if err := c.watcher.Close(); err != nil {
		c.logger.Warn("close agent config watcher failed", "error", err)
	}
	return runtime.close(ctx)
}

func (c *RuntimeController) watch(ctx context.Context) {
	if err := c.watcher.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		c.logger.Warn("agent config watcher stopped", "error", err)
	}
}

func (c *RuntimeController) reload(ctx context.Context, runSeq uint64, cfg config.Config) error {
	if !c.isActive(runSeq) {
		return nil
	}
	if err := c.ctxErr(ctx); err != nil {
		return err
	}
	return c.startRuntime(ctx, runSeq, cfg)
}

func (c *RuntimeController) startRuntime(ctx context.Context, runSeq uint64, cfg config.Config) error {
	next, err := c.buildRuntime(ctx, cfg)
	if err != nil {
		return err
	}
	if err := next.runner.Start(ctx); err != nil {
		return c.closeRuntimeOnStartFailure(ctx, next, err)
	}
	return c.activateRuntime(ctx, runSeq, cfg.Agent.Name, next)
}

func (c *RuntimeController) closeRuntimeOnStartFailure(ctx context.Context, runtime *runtimeInstance, startErr error) error {
	if closeErr := runtime.close(ctx); closeErr != nil {
		return errors.Join(startErr, closeErr)
	}
	return startErr
}

func (c *RuntimeController) activateRuntime(ctx context.Context, runSeq uint64, agentName string, next *runtimeInstance) error {
	previous, active := c.swapRuntime(runSeq, next)
	if !active {
		return next.close(ctx)
	}

	if previous != nil {
		if err := previous.close(ctx); err != nil {
			c.logger.Warn("close previous agent runtime failed", "error", err)
		}
	}
	c.logger.Info("agent runtime started", "agent", agentName, "server_url", next.serverURL)
	return nil
}

func (c *RuntimeController) swapRuntime(runSeq uint64, next *runtimeInstance) (*runtimeInstance, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started || c.runSeq != runSeq {
		return nil, false
	}
	previous := c.runtime
	c.runtime = next
	return previous, true
}

func (c *RuntimeController) ctxErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("agent runtime context stopped: %w", err)
	}
	return nil
}

func (c *RuntimeController) buildRuntime(ctx context.Context, cfg config.Config) (*runtimeInstance, error) {
	endpoint, err := c.resolveServerEndpoint(ctx, cfg)
	if err != nil {
		return nil, err
	}
	cfg.Server.URL = endpoint.URL
	client, err := c.deps.NewClient(cfg)
	if err != nil {
		return nil, oops.Wrapf(err, "create agent client")
	}
	discovery, err := c.deps.NewDiscoverer(cfg)
	if err != nil {
		return nil, oops.Wrapf(err, "create monitor discoverer")
	}
	results, err := c.deps.NewResultQueue(ctx, cfg)
	if err != nil {
		return nil, oops.Wrapf(err, "create result queue")
	}
	return &runtimeInstance{
		client:    client,
		serverURL: endpoint.URL,
		runner: NewRunner(
			cfg,
			c.logger,
			c.obs,
			client,
			c.taskPool,
			discovery,
			results,
		),
	}, nil
}

func (c *RuntimeController) resolveServerEndpoint(ctx context.Context, cfg config.Config) (servicediscovery.ServerEndpoint, error) {
	if cfg.Server.URL != "" {
		return servicediscovery.ServerEndpoint{URL: cfg.Server.URL, Source: "static"}, nil
	}
	endpoint, err := servicediscovery.ResolveMDNSServer(ctx, servicediscovery.MDNSResolveConfig{
		Service:       cfg.Server.MDNS.Service,
		Domain:        cfg.Server.MDNS.Domain,
		Timeout:       cfg.Server.MDNS.Timeout,
		DefaultScheme: cfg.Server.MDNS.DefaultScheme,
	}, c.logger)
	if err != nil {
		return servicediscovery.ServerEndpoint{}, fmt.Errorf("resolve server URL: %w", err)
	}
	return endpoint, nil
}

func (r *runtimeInstance) close(ctx context.Context) error {
	var errs []error
	if r.runner != nil {
		if err := r.runner.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if r.client != nil {
		if err := r.client.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
