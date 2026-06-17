package discovery

import (
	"context"
	"log/slog"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"
)

func (d *DockerDiscoverer) discoverServices(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	result, err := d.client.ServiceList(ctx, dockerclient.ServiceListOptions{})
	if err != nil {
		return nil, wrapError(err, "list Docker services")
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
			return collectLabelMonitors(out, toSource(item), defaultEnvironment)
		},
	)
	if err != nil {
		return nil, wrapErrorf(err, "%s labels", parseErrPrefix)
	}
	parsed := monitors.Values()
	if logger != nil {
		logger.Info(
			source+" monitors discovered",
			"count", len(parsed),
			"source", source,
		)
		for i := range parsed {
			monitor := &parsed[i]
			logger.Debug(
				"monitor parsed",
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

func collectLabelMonitors(
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
