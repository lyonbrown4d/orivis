package discovery

import (
	"context"
	"fmt"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/swarm"
	dockerclient "github.com/moby/moby/client"
)

const (
	DockerModeAuto      = "auto"
	DockerModeContainer = "container"
	DockerModeSwarm     = "swarm"
)

type DockerOptions struct {
	Mode               string
	DefaultEnvironment string
}

type DockerDiscoverer struct {
	client             *dockerclient.Client
	mode               string
	defaultEnvironment string
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
	}, nil
}

func (d *DockerDiscoverer) Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	if d == nil || d.client == nil {
		return nil, nil
	}

	mode, err := d.discoveryMode(ctx)
	if err != nil {
		return nil, err
	}
	switch mode {
	case DockerModeContainer:
		return d.discoverContainers(ctx)
	case DockerModeSwarm:
		return d.discoverServices(ctx)
	default:
		return nil, fmt.Errorf("unsupported Docker discovery mode %q", mode)
	}
}

func (d *DockerDiscoverer) discoveryMode(ctx context.Context) (string, error) {
	if d.mode != DockerModeAuto {
		return d.mode, nil
	}
	ping, err := d.client.Ping(ctx, dockerclient.PingOptions{})
	if err != nil {
		return "", fmt.Errorf("inspect Docker daemon: %w", err)
	}
	if swarmServiceDiscoveryAvailable(ping.SwarmStatus) {
		return DockerModeSwarm, nil
	}
	return DockerModeContainer, nil
}

func swarmServiceDiscoveryAvailable(status *dockerclient.SwarmStatus) bool {
	if status == nil {
		return false
	}
	return status.NodeState == swarm.LocalNodeStateActive && status.ControlAvailable
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

	monitors, err := collectionlist.ReduceErrList(
		collectionlist.NewList(result.Items...),
		collectionlist.NewList[protocol.AgentDiscoveredMonitor](),
		func(out *collectionlist.List[protocol.AgentDiscoveredMonitor], _ int, item container.Summary) (*collectionlist.List[protocol.AgentDiscoveredMonitor], error) {
			return collectDockerLabelMonitors(out, ContainerLabelSource(item), d.defaultEnvironment)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("parse Docker container labels: %w", err)
	}
	return monitors.Values(), nil
}

func (d *DockerDiscoverer) discoverServices(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	result, err := d.client.ServiceList(ctx, dockerclient.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Docker services: %w", err)
	}

	monitors, err := collectionlist.ReduceErrList(
		collectionlist.NewList(result.Items...),
		collectionlist.NewList[protocol.AgentDiscoveredMonitor](),
		func(out *collectionlist.List[protocol.AgentDiscoveredMonitor], _ int, item swarm.Service) (*collectionlist.List[protocol.AgentDiscoveredMonitor], error) {
			return collectDockerLabelMonitors(out, ServiceLabelSource(item), d.defaultEnvironment)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("parse Docker service labels: %w", err)
	}
	return monitors.Values(), nil
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
	case "", DockerModeAuto:
		return DockerModeAuto
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
