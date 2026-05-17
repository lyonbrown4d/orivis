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
	"github.com/samber/oops"
)

type RuntimeController struct {
	watcher *config.Watcher
	logger  *slog.Logger
	obs     observabilityx.Observability

	mu      sync.Mutex
	cancel  context.CancelFunc
	runtime *runtimeInstance
}

type runtimeInstance struct {
	client *agentclient.Client
	runner *Runner
}

func NewRuntimeController(watcher *config.Watcher, logger *slog.Logger, obs observabilityx.Observability) (*RuntimeController, error) {
	if watcher == nil {
		return nil, errors.New("agent config watcher is required")
	}
	obs = observabilityx.Normalize(obs, logger)
	return &RuntimeController{
		watcher: watcher,
		logger:  logger,
		obs:     obs,
	}, nil
}

func (c *RuntimeController) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	c.cancel = cancel

	if err := c.reload(runCtx, c.watcher.Config()); err != nil {
		cancel()
		return err
	}

	c.watcher.OnChange(func(cfg config.Config, err error) {
		if err != nil {
			c.logger.Warn("agent config reload failed", "error", err)
			return
		}
		go c.handleConfigChange(runCtx, cfg)
	})

	go c.watch(runCtx)
	return nil
}

func (c *RuntimeController) Stop(ctx context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	if err := c.watcher.Close(); err != nil {
		c.logger.Warn("close agent config watcher failed", "error", err)
	}
	return c.stopRuntime(ctx)
}

func (c *RuntimeController) handleConfigChange(ctx context.Context, cfg config.Config) {
	if err := c.reload(ctx, cfg); err != nil {
		c.logger.Warn("restart agent runtime failed; keeping previous runtime", "error", err)
	}
}

func (c *RuntimeController) watch(ctx context.Context) {
	if err := c.watcher.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		c.logger.Warn("agent config watcher stopped", "error", err)
	}
}

func (c *RuntimeController) reload(ctx context.Context, cfg config.Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("agent runtime context stopped: %w", err)
	}

	next, err := c.buildRuntime(cfg)
	if err != nil {
		return err
	}
	if err := next.runner.Start(ctx); err != nil {
		if closeErr := next.close(ctx); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}

	previous := c.runtime
	c.runtime = next

	if previous != nil {
		if err := previous.close(ctx); err != nil {
			c.logger.Warn("close previous agent runtime failed", "error", err)
		}
	}

	c.logger.Info("agent runtime started", "agent", cfg.Agent.Name, "server_url", cfg.Server.URL)
	return nil
}

func (c *RuntimeController) buildRuntime(cfg config.Config) (*runtimeInstance, error) {
	client, err := agentclient.New(cfg, c.logger, c.obs)
	if err != nil {
		return nil, oops.Wrapf(err, "create agent client")
	}
	return &runtimeInstance{
		client: client,
		runner: NewRunner(
			cfg,
			c.logger,
			client,
		),
	}, nil
}

func (c *RuntimeController) stopRuntime(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.runtime == nil {
		return nil
	}
	runtime := c.runtime
	c.runtime = nil
	return runtime.close(ctx)
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
