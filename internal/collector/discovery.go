package collector

import (
	"context"

	collectionlist "github.com/arcgolabs/collectionx/list"
	agentdiscovery "github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

func (r *Runner) configureDiscovery() error {
	discoverers := collectionlist.NewListWithCapacity[monitorDiscoverer](2)

	if r.cfg.Discovery.Static.Enabled && len(r.cfg.Discovery.Static.Monitors) > 0 {
		discoverers.Add(agentdiscovery.NewStaticDiscoverer(r.cfg.Discovery.Static.Monitors))
		r.logger.Info("static discovery enabled", "count", len(r.cfg.Discovery.Static.Monitors))
	}

	if r.cfg.Discovery.Docker.Enabled {
		discoverer, err := agentdiscovery.NewDockerDiscoverer(agentdiscovery.DockerOptions{
			Mode:               r.cfg.Discovery.Docker.Mode,
			DefaultEnvironment: defaultDiscoveryEnvironment(r.cfg.Agent.Environments),
		})
		if err != nil {
			return oops.Wrapf(err, "create Docker discoverer")
		}
		discoverers.Add(discoverer)
		r.logger.Info("Docker discovery enabled", "mode", r.cfg.Discovery.Docker.Mode)
	}

	switch discoverers.Len() {
	case 0:
		return nil
	case 1:
		discoverer, _ := discoverers.GetFirst()
		r.discovery = discoverer
	default:
		r.discovery = compositeDiscoverer{discoverers: discoverers}
	}
	return nil
}

func defaultDiscoveryEnvironment(environments []string) string {
	if len(environments) == 0 {
		return ""
	}
	return environments[0]
}

type compositeDiscoverer struct {
	discoverers *collectionlist.List[monitorDiscoverer]
}

func (d compositeDiscoverer) Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	out := collectionlist.NewList[protocol.AgentDiscoveredMonitor]()
	var discoverErr error
	d.discoverers.Range(func(_ int, discoverer monitorDiscoverer) bool {
		monitors, err := discoverer.Discover(ctx)
		if err != nil {
			discoverErr = oops.Wrapf(err, "discover monitors")
			return false
		}
		out.Add(monitors...)
		return true
	})
	if discoverErr != nil {
		return nil, discoverErr
	}
	return out.Values(), nil
}

func (d compositeDiscoverer) Close(ctx context.Context) error {
	var closeErr error
	d.discoverers.Range(func(_ int, discoverer monitorDiscoverer) bool {
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
