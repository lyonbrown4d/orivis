package collector

import (
	"context"
	"log/slog"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionset "github.com/arcgolabs/collectionx/set"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	agentdiscovery "github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/lo"
	"github.com/samber/oops"
)

func NewMonitorDiscoverer(cfg config.Config, logger *slog.Logger) (MonitorDiscoverer, error) {
	discoverers := collectionlist.NewListWithCapacity[MonitorDiscoverer](3)

	if cfg.Discovery.Static.Enabled && len(cfg.Discovery.Static.Monitors) > 0 {
		discoverers.Add(agentdiscovery.NewStaticDiscoverer(cfg.Discovery.Static.Monitors))
		logger.Info("static discovery enabled", "count", len(cfg.Discovery.Static.Monitors))
	}

	if dockerDiscoveryEnabled(cfg) {
		discoverer, err := agentdiscovery.NewDockerDiscoverer(agentdiscovery.DockerOptions{
			Mode:               cfg.Discovery.Docker.Mode,
			DefaultEnvironment: defaultDiscoveryEnvironment(cfg.Agent.Environments),
			Logger:             logger,
		})
		if err != nil {
			return nil, oops.Wrapf(err, "create Docker discoverer")
		}
		discoverers.Add(discoverer)
		logger.Info(
			"Docker discovery enabled",
			"provider", cfg.Discovery.Provider,
			"mode", cfg.Discovery.Docker.Mode,
			"default_environment", defaultDiscoveryEnvironment(cfg.Agent.Environments),
		)
	}

	if kubernetesDiscoveryEnabled(cfg) {
		discoverer, err := agentdiscovery.NewKubernetesDiscoverer(agentdiscovery.KubernetesOptions{
			Mode:               cfg.Discovery.Kubernetes.Mode,
			Namespace:          cfg.Discovery.Kubernetes.Namespace,
			Namespaces:         cfg.Discovery.Kubernetes.Namespaces,
			Kubeconfig:         cfg.Discovery.Kubernetes.Kubeconfig,
			DefaultEnvironment: defaultDiscoveryEnvironment(cfg.Agent.Environments),
			Logger:             logger,
		})
		if err != nil {
			return nil, oops.Wrapf(err, "create Kubernetes discoverer")
		}
		discoverers.Add(discoverer)
		logger.Info(
			"Kubernetes discovery enabled",
			"provider", cfg.Discovery.Provider,
			"mode", cfg.Discovery.Kubernetes.Mode,
			"namespace", cfg.Discovery.Kubernetes.Namespace,
			"namespaces", cfg.Discovery.Kubernetes.Namespaces,
			"default_environment", defaultDiscoveryEnvironment(cfg.Agent.Environments),
		)
	}

	if discoverers.Len() == 0 {
		logger.Info("monitor discovery disabled")
		return disabledDiscoverer{}, nil
	}
	return compositeDiscoverer{discoverers: discoverers}, nil
}

func dockerDiscoveryEnabled(cfg config.Config) bool {
	return cfg.Discovery.Provider == "docker" || cfg.Discovery.Docker.Enabled
}

func kubernetesDiscoveryEnabled(cfg config.Config) bool {
	return cfg.Discovery.Provider == "kubernetes" || cfg.Discovery.Kubernetes.Enabled
}

func defaultDiscoveryEnvironment(environments []string) string {
	return lo.FirstOrEmpty(environments)
}

type compositeDiscoverer struct {
	discoverers *collectionlist.List[MonitorDiscoverer]
}

func (d compositeDiscoverer) Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	out := collectionlist.NewList[protocol.AgentDiscoveredMonitor]()
	out, discoverErr := collectionlist.ReduceErrList(
		d.discoverers,
		out,
		func(acc *collectionlist.List[protocol.AgentDiscoveredMonitor], _ int, discoverer MonitorDiscoverer) (*collectionlist.List[protocol.AgentDiscoveredMonitor], error) {
			monitors, err := discoverer.Discover(ctx)
			if err != nil {
				return nil, oops.Wrapf(err, "discover monitors")
			}
			acc.Add(monitors...)
			return acc, nil
		},
	)
	if discoverErr != nil {
		return nil, oops.Wrapf(discoverErr, "discover monitors")
	}
	return deduplicateMonitors(out.Values()), nil
}

func (d compositeDiscoverer) Close(ctx context.Context) error {
	var closeErr error
	d.discoverers.Range(func(_ int, discoverer MonitorDiscoverer) bool {
		if err := discoverer.Close(ctx); err != nil {
			closeErr = oops.Wrapf(err, "close discoverer")
			return false
		}
		return true
	})
	if closeErr != nil {
		return closeErr
	}
	return nil
}

type disabledDiscoverer struct{}

func (disabledDiscoverer) Discover(context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	return nil, nil
}

func (disabledDiscoverer) Close(context.Context) error {
	return nil
}

func deduplicateMonitors(monitors []protocol.AgentDiscoveredMonitor) []protocol.AgentDiscoveredMonitor {
	if len(monitors) == 0 {
		return nil
	}
	seen := collectionset.NewSetWithCapacity[string](len(monitors))
	out := collectionlist.NewListWithCapacity[protocol.AgentDiscoveredMonitor](len(monitors))
	for i := range monitors {
		key := discoveredMonitorKey(monitors[i])
		if seen.Contains(key) {
			continue
		}
		seen.Add(key)
		out.Add(monitors[i])
	}
	return out.Values()
}
