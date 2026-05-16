// Package retention provides scheduled cleanup jobs for server-side data.
package retention

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type Cleaner struct {
	storage   *store.Store
	logger    *slog.Logger
	enabled   bool
	ttl       time.Duration
	interval  time.Duration
	scheduler *gocron.Scheduler
}

func New(cfg config.Config, storage *store.Store, logger *slog.Logger) (*Cleaner, error) {
	if !cfg.Retention.Enabled {
		return &Cleaner{storage: storage, logger: logger}, nil
	}
	ttl, err := parsePositiveDuration(cfg.Retention.ResultTTL, "retention result TTL")
	if err != nil {
		return nil, err
	}
	interval, err := parsePositiveDuration(cfg.Retention.CleanupInterval, "retention cleanup interval")
	if err != nil {
		return nil, err
	}
	return &Cleaner{
		storage:  storage,
		logger:   logger,
		enabled:  true,
		ttl:      ttl,
		interval: interval,
	}, nil
}

func (c *Cleaner) Start(ctx context.Context) error {
	if c == nil || !c.enabled {
		return nil
	}
	scheduler := gocron.NewScheduler(time.UTC)
	if _, err := scheduler.Every(c.interval).SingletonMode().Do(func() {
		c.cleanup(ctx)
	}); err != nil {
		return wrapError(err, "schedule retention cleanup")
	}
	c.scheduler = scheduler
	scheduler.StartAsync()
	go c.cleanup(ctx)
	return nil
}

func (c *Cleaner) Stop(context.Context) error {
	if c == nil || c.scheduler == nil {
		return nil
	}
	c.scheduler.Stop()
	return nil
}

func (c *Cleaner) cleanup(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	if c.storage == nil || c.storage.ResultStore() == nil {
		c.logWarn("retention cleanup skipped", "reason", "result store is not available")
		return
	}
	before := time.Now().UTC().Add(-c.ttl)
	deleted, err := c.storage.ResultStore().DeleteBefore(ctx, before)
	if err != nil {
		c.logWarn("retention cleanup failed", "error", err)
		return
	}
	if deleted > 0 {
		c.logInfo("retention cleanup completed", "deleted", deleted, "before", before)
	}
}

func (c *Cleaner) logInfo(message string, args ...any) {
	if c.logger != nil {
		c.logger.Info(message, args...)
	}
}

func (c *Cleaner) logWarn(message string, args ...any) {
	if c.logger != nil {
		c.logger.Warn(message, args...)
	}
}

func parsePositiveDuration(value, name string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, newError(name + " is required")
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, wrapError(err, "parse "+name)
	}
	if duration <= 0 {
		return 0, newError(name + " must be greater than zero")
	}
	return duration, nil
}
