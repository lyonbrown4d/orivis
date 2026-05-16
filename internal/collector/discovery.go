package collector

import (
	"context"

	agentdiscovery "github.com/lyonbrown4d/orivis/internal/discovery"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func (r *Runner) configureDiscovery() error {
	discoverers := make([]monitorDiscoverer, 0, 2)

	if r.cfg.Discovery.Static.Enabled && len(r.cfg.Discovery.Static.Monitors) > 0 {
		discoverers = append(discoverers, agentdiscovery.NewStaticDiscoverer(r.cfg.Discovery.Static.Monitors))
		r.logger.Info("static discovery enabled", "count", len(r.cfg.Discovery.Static.Monitors))
	}

	if r.cfg.Discovery.Docker.Enabled {
		discoverer, err := agentdiscovery.NewDockerDiscoverer(agentdiscovery.DockerOptions{
			Mode: r.cfg.Discovery.Docker.Mode,
		})
		if err != nil {
			return err
		}
		discoverers = append(discoverers, discoverer)
		r.logger.Info("Docker discovery enabled", "mode", r.cfg.Discovery.Docker.Mode)
	}

	switch len(discoverers) {
	case 0:
		return nil
	case 1:
		r.discovery = discoverers[0]
	default:
		r.discovery = compositeDiscoverer{discoverers: discoverers}
	}
	return nil
}

type compositeDiscoverer struct {
	discoverers []monitorDiscoverer
}

func (d compositeDiscoverer) Discover(ctx context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	out := make([]protocol.AgentDiscoveredMonitor, 0)
	for _, discoverer := range d.discoverers {
		monitors, err := discoverer.Discover(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, monitors...)
	}
	return out, nil
}

func (d compositeDiscoverer) Close(ctx context.Context) error {
	for _, discoverer := range d.discoverers {
		if err := discoverer.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}
