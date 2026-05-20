package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"
)

const (
	DockerModeContainer = "container"
	DockerModeSwarm     = "swarm"
)

const defaultContainerInspectCacheTTL = 30 * time.Second

type DockerOptions struct {
	Mode               string
	DefaultEnvironment string
	Logger             *slog.Logger
}

type DockerDiscoverer struct {
	client             *dockerclient.Client
	mode               string
	defaultEnvironment string
	logger             *slog.Logger
	inspectCache       map[string]cachedDockerContainerConfig
	inspectCacheTTL    time.Duration
	inspectCacheMu     sync.RWMutex
}

type cachedDockerContainerConfig struct {
	config    container.Config
	expiresAt time.Time
}

func NewDockerDiscoverer(opts DockerOptions) (*DockerDiscoverer, error) {
	mode := normalizeDockerMode(opts.Mode)
	if mode == "" {
		return nil, fmt.Errorf("unsupported Docker discovery mode %q", opts.Mode)
	}

	client, err := dockerclient.New(dockerclient.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}

	return &DockerDiscoverer{
		client:             client,
		mode:               mode,
		defaultEnvironment: strings.TrimSpace(opts.DefaultEnvironment),
		logger:             opts.Logger,
		inspectCache:       make(map[string]cachedDockerContainerConfig),
		inspectCacheTTL:    defaultContainerInspectCacheTTL,
	}, nil
}

func (d *DockerDiscoverer) Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	if d == nil || d.client == nil {
		return nil, nil
	}

	if d.logger != nil {
		d.logger.Info("docker discovery mode", "mode", d.mode)
	}
	switch d.mode {
	case DockerModeContainer:
		return d.discoverContainers(ctx)
	case DockerModeSwarm:
		return d.discoverServices(ctx)
	default:
		return nil, fmt.Errorf("unsupported Docker discovery mode %q", d.mode)
	}
}

func (d *DockerDiscoverer) Close(context.Context) error {
	if d == nil || d.client == nil {
		return nil
	}
	if err := d.client.Close(); err != nil {
		return fmt.Errorf("close Docker client: %w", err)
	}
	return nil
}

func (d *DockerDiscoverer) discoverContainers(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	result, err := d.client.ContainerList(ctx, dockerclient.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Docker containers: %w", err)
	}
	if d.logger != nil {
		d.logger.Info("discovering docker containers", "count", len(result.Items))
	}
	containers := d.enrichContainers(ctx, result.Items)
	parsed, err := discoverByItems(
		containers,
		"docker_container",
		d.logger,
		d.defaultEnvironment,
		ContainerLabelSource,
		"list Docker containers",
	)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (d *DockerDiscoverer) enrichContainers(ctx context.Context, items []container.Summary) []container.Summary {
	d.pruneContainerInspectCache(time.Now().UTC())
	stats := dockerContainerEnrichmentStats{
		scanned: len(items),
	}
	portsNeeded := collectionlist.NewListWithCapacity[container.Summary](len(items))
	for i := range items {
		item := items[i]
		enriched := d.enrichContainerPortsFromEngine(ctx, item, &stats)
		portsNeeded.Add(enriched)
	}
	if d.logger != nil {
		d.logger.Info(
			"docker container port metadata enrichment",
			"scanned", stats.scanned,
			"skipped_with_ports", stats.skippedWithPorts,
			"inspected", stats.inspected,
			"inspect_failed", stats.inspectFailed,
			"inspect_no_config", stats.inspectNoConfig,
			"enriched_with_ports", stats.enrichedWithPorts,
			"image_backfilled", stats.imageBackfilled,
			"cache_hits", stats.cacheHits,
			"cache_misses", stats.cacheMisses,
			"cache_expired", stats.cacheExpired,
			"no_id", stats.noID,
		)
	}
	return portsNeeded.Values()
}

type dockerContainerEnrichmentStats struct {
	scanned           int
	noID              int
	skippedWithPorts  int
	inspected         int
	inspectFailed     int
	inspectNoConfig   int
	enrichedWithPorts int
	imageBackfilled   int
	cacheHits         int
	cacheMisses       int
	cacheExpired      int
}

func (d *DockerDiscoverer) enrichContainerPortsFromEngine(ctx context.Context, item container.Summary, stats *dockerContainerEnrichmentStats) container.Summary {
	if len(ContainerPorts(item)) > 0 {
		stats.skippedWithPorts++
		return item
	}
	stats.inspected++
	config, ok := d.inspectContainerForEnrichment(ctx, item.ID, stats)
	if !ok {
		return item
	}

	beforeLen := len(item.Ports)
	enriched := enrichContainerPortsFromInspect(item, config.ExposedPorts)
	if strings.TrimSpace(enriched.Image) == "" {
		enriched.Image = config.Image
		if strings.TrimSpace(enriched.Image) != "" {
			stats.imageBackfilled++
		}
	}
	if len(enriched.Ports) > beforeLen {
		stats.enrichedWithPorts++
	}
	return enriched
}

func (d *DockerDiscoverer) inspectContainerForEnrichment(ctx context.Context, containerID string, stats *dockerContainerEnrichmentStats) (container.Config, bool) {
	if d == nil || d.client == nil {
		return container.Config{}, false
	}
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		stats.noID++
		return container.Config{}, false
	}

	now := time.Now().UTC()
	if config, ok := d.getCachedContainerConfig(containerID, now, stats); ok {
		return config, true
	}

	result, err := d.client.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
	if err != nil {
		stats.inspectFailed++
		if d.logger != nil {
			d.logger.Warn("inspect docker container failed", "container_id", shortDockerID(containerID), "error", err)
		}
		return container.Config{}, false
	}
	if result.Container.Config == nil {
		stats.inspectNoConfig++
		return container.Config{}, false
	}
	d.putCachedContainerConfig(containerID, now, *result.Container.Config)
	return *result.Container.Config, true
}

func (d *DockerDiscoverer) getCachedContainerConfig(containerID string, now time.Time, stats *dockerContainerEnrichmentStats) (container.Config, bool) {
	if d == nil || d.inspectCacheTTL <= 0 {
		return container.Config{}, false
	}

	d.inspectCacheMu.RLock()
	entry, ok := d.inspectCache[containerID]
	d.inspectCacheMu.RUnlock()
	if !ok {
		stats.cacheMisses++
		return container.Config{}, false
	}
	if now.After(entry.expiresAt) {
		stats.cacheExpired++
		d.inspectCacheMu.Lock()
		delete(d.inspectCache, containerID)
		d.inspectCacheMu.Unlock()
		return container.Config{}, false
	}
	stats.cacheHits++
	return entry.config, true
}

func (d *DockerDiscoverer) putCachedContainerConfig(containerID string, now time.Time, config container.Config) {
	if d == nil || d.inspectCacheTTL <= 0 {
		return
	}
	d.inspectCacheMu.Lock()
	d.inspectCache[containerID] = cachedDockerContainerConfig{
		config:    config,
		expiresAt: now.Add(d.inspectCacheTTL),
	}
	d.inspectCacheMu.Unlock()
}

func (d *DockerDiscoverer) pruneContainerInspectCache(now time.Time) {
	if d == nil || d.inspectCacheTTL <= 0 {
		return
	}
	d.inspectCacheMu.Lock()
	for containerID := range d.inspectCache {
		entry := d.inspectCache[containerID]
		if now.After(entry.expiresAt) {
			delete(d.inspectCache, containerID)
		}
	}
	d.inspectCacheMu.Unlock()
}
