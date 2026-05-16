package discovery

import (
	"context"
	"fmt"
	"strings"

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
	Mode string
}

type DockerDiscoverer struct {
	client *dockerclient.Client
	mode   string
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

	return &DockerDiscoverer{client: client, mode: mode}, nil
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

	out := make([]protocol.AgentDiscoveredMonitor, 0)
	for index := range result.Items {
		item := result.Items[index]
		monitors, err := ParseLabels(LabelSource{
			SourceKey: ContainerSourceKey(item),
			Labels:    item.Labels,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, monitors...)
	}
	return out, nil
}

func (d *DockerDiscoverer) discoverServices(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	result, err := d.client.ServiceList(ctx, dockerclient.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Docker services: %w", err)
	}

	out := make([]protocol.AgentDiscoveredMonitor, 0)
	for index := range result.Items {
		item := result.Items[index]
		monitors, err := ParseLabels(LabelSource{
			SourceKey: ServiceSourceKey(item),
			Labels:    item.Spec.Labels,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, monitors...)
	}
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

func shortDockerID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
