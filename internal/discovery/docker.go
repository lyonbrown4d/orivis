package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"
)

const (
	DockerModeContainer = "container"
	DockerModeSwarm     = "swarm"
)

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
	parsed, err := discoverByItems(
		result.Items,
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

func (d *DockerDiscoverer) discoverServices(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	result, err := d.client.ServiceList(ctx, dockerclient.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Docker services: %w", err)
	}
	if d.logger != nil {
		d.logger.Info("discovering docker services", "count", len(result.Items))
	}
	parsed, err := discoverByItems(
		result.Items,
		"docker_swarm_service",
		d.logger,
		d.defaultEnvironment,
		ServiceLabelSource,
		"list Docker services",
	)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func discoverByItems[T any](
	items []T,
	source string,
	logger *slog.Logger,
	defaultEnvironment string,
	toSource func(T) LabelSource,
	parseErrPrefix string,
) ([]protocol.AgentDiscoveredMonitor, error) {
	monitors, err := collectionlist.ReduceErrList(
		collectionlist.NewList(items...),
		collectionlist.NewList[protocol.AgentDiscoveredMonitor](),
		func(out *collectionlist.List[protocol.AgentDiscoveredMonitor], _ int, item T) (*collectionlist.List[protocol.AgentDiscoveredMonitor], error) {
			return collectDockerLabelMonitors(out, toSource(item), defaultEnvironment)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("%s labels: %w", parseErrPrefix, err)
	}
	parsed := monitors.Values()
	if logger != nil {
		logger.Info(
			fmt.Sprintf("docker %s monitors discovered", source),
			"count", len(parsed),
			"source", source,
		)
		for i := range parsed {
			monitor := &parsed[i]
			logger.Info(
				"docker monitor parsed",
				"source_key", monitor.SourceKey,
				"monitor_name", monitor.Name,
				"monitor_type", monitor.Type,
				"monitor_target", monitor.Target,
				"source", source,
				"environment", monitor.EnvironmentCode,
				"group", monitor.GroupName,
			)
		}
	}
	return parsed, nil
}

func collectDockerLabelMonitors(
	out *collectionlist.List[protocol.AgentDiscoveredMonitor],
	source LabelSource,
	defaultEnvironment string,
) (*collectionlist.List[protocol.AgentDiscoveredMonitor], error) {
	source.DefaultEnvironment = firstNonEmpty(defaultEnvironment, source.DefaultEnvironment)
	parsed, err := ParseLabels(source)
	if err != nil {
		return nil, err
	}
	out.Add(parsed...)
	return out, nil
}

func normalizeDockerMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case DockerModeContainer:
		return DockerModeContainer
	case DockerModeSwarm:
		return DockerModeSwarm
	default:
		return ""
	}
}

func containerRuntimeName(item container.Summary) string {
	if len(item.Names) == 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(item.Names[0]), "/")
}

func shortDockerID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
