package discovery

import (
	"context"
	"fmt"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/swarm"
	dockerclient "github.com/moby/moby/client"
)

const (
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
	case "", DockerModeContainer:
		return DockerModeContainer
	case DockerModeSwarm:
		return DockerModeSwarm
	default:
		return ""
	}
}

// ContainerLabelSource returns label parsing metadata for a Docker container.
func ContainerLabelSource(item container.Summary) LabelSource {
	return LabelSource{
		SourceKey:          ContainerSourceKey(item),
		Labels:             item.Labels,
		DefaultName:        ContainerName(item),
		DefaultEnvironment: ContainerEnvironment(item),
		TargetHost:         ContainerTargetHost(item),
		Ports:              ContainerPorts(item),
	}
}

// ContainerSourceKey returns the stable discovery source key for a Docker container.
func ContainerSourceKey(item container.Summary) string {
	if project := strings.TrimSpace(item.Labels["com.docker.compose.project"]); project != "" {
		if service := strings.TrimSpace(item.Labels["com.docker.compose.service"]); service != "" {
			return "docker:compose:" + project + ":" + service
		}
	}

	if len(item.Names) > 0 {
		name := strings.Trim(strings.TrimSpace(item.Names[0]), "/")
		if name != "" {
			return "docker:container:" + name
		}
	}

	id := shortDockerID(item.ID)
	if id == "" {
		id = "unknown"
	}
	return "docker:container:" + id
}

// ContainerName returns the best user-facing monitor name for a Docker container.
func ContainerName(item container.Summary) string {
	return firstNonEmpty(
		item.Labels["com.docker.compose.service"],
		item.Labels["com.docker.swarm.service.name"],
		containerRuntimeName(item),
		shortDockerID(item.ID),
	)
}

// ContainerEnvironment returns the best environment fallback for a Docker container.
func ContainerEnvironment(item container.Summary) string {
	return firstNonEmpty(
		item.Labels["com.docker.compose.project"],
		item.Labels["com.docker.stack.namespace"],
	)
}

// ContainerTargetHost returns the DNS name an agent can use from the same Docker network.
func ContainerTargetHost(item container.Summary) string {
	return firstNonEmpty(
		item.Labels["com.docker.compose.service"],
		item.Labels["com.docker.swarm.service.name"],
		containerRuntimeName(item),
		shortDockerID(item.ID),
	)
}

// ContainerPorts returns exposed private container ports.
func ContainerPorts(item container.Summary) []int {
	seen := collectionset.NewSetWithCapacity[int](len(item.Ports))
	return collectionlist.FilterMapList(collectionlist.NewList(item.Ports...), func(_ int, port container.PortSummary) (int, bool) {
		if !strings.EqualFold(strings.TrimSpace(port.Type), "tcp") {
			return 0, false
		}
		value := int(port.PrivatePort)
		if value == 0 {
			return 0, false
		}
		if seen.Contains(value) {
			return 0, false
		}
		seen.Add(value)
		return value, true
	}).Values()
}

// ServiceSourceKey returns the stable discovery source key for a Docker Swarm service.
func ServiceSourceKey(item swarm.Service) string {
	if namespace := strings.TrimSpace(item.Spec.Labels["com.docker.stack.namespace"]); namespace != "" {
		if item.Spec.Name != "" {
			return "docker:swarm:" + namespace + ":" + item.Spec.Name
		}
	}
	if item.Spec.Name != "" {
		return "docker:swarm:" + item.Spec.Name
	}
	id := shortDockerID(item.ID)
	if id == "" {
		id = "unknown"
	}
	return "docker:swarm:" + id
}

// ServiceLabelSource returns label parsing metadata for a Docker Swarm service.
func ServiceLabelSource(item swarm.Service) LabelSource {
	return LabelSource{
		SourceKey:          ServiceSourceKey(item),
		Labels:             item.Spec.Labels,
		DefaultName:        ServiceName(item),
		DefaultEnvironment: ServiceEnvironment(item),
		TargetHost:         ServiceTargetHost(item),
		Ports:              ServicePorts(item),
	}
}

// ServiceName returns the best user-facing monitor name for a Docker Swarm service.
func ServiceName(item swarm.Service) string {
	return firstNonEmpty(item.Spec.Name, shortDockerID(item.ID))
}

// ServiceEnvironment returns the best environment fallback for a Docker Swarm service.
func ServiceEnvironment(item swarm.Service) string {
	return firstNonEmpty(item.Spec.Labels["com.docker.stack.namespace"])
}

// ServiceTargetHost returns the DNS name an agent can use inside a Docker Swarm network.
func ServiceTargetHost(item swarm.Service) string {
	return ServiceName(item)
}

// ServicePorts returns exposed target ports for a Docker Swarm service.
func ServicePorts(item swarm.Service) []int {
	seen := collectionset.NewSetWithCapacity[int](len(item.Endpoint.Ports))
	return collectionlist.FilterMapList(collectionlist.NewList(item.Endpoint.Ports...), func(_ int, port swarm.PortConfig) (int, bool) {
		if !strings.EqualFold(string(port.Protocol), "tcp") {
			return 0, false
		}
		value := int(port.TargetPort)
		if value == 0 {
			return 0, false
		}
		if seen.Contains(value) {
			return 0, false
		}
		seen.Add(value)
		return value, true
	}).Values()
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
