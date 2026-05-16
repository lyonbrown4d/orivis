package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/protocol"
)

type StaticMonitor struct {
	SourceKey         string        `mapstructure:"source_key"`
	Name              string        `mapstructure:"name"`
	Type              string        `mapstructure:"type"`
	Target            string        `mapstructure:"target"`
	EnvironmentCode   string        `mapstructure:"environment"`
	Enabled           *bool         `mapstructure:"enabled"`
	Interval          time.Duration `mapstructure:"interval"`
	Timeout           time.Duration `mapstructure:"timeout"`
	RetryCount        int           `mapstructure:"retry_count"`
	AggregationPolicy string        `mapstructure:"aggregation"`
}

type StaticDiscoverer struct {
	monitors []StaticMonitor
}

func NewStaticDiscoverer(monitors []StaticMonitor) *StaticDiscoverer {
	return &StaticDiscoverer{monitors: monitors}
}

func (d *StaticDiscoverer) Discover(context.Context) ([]protocol.AgentDiscoveredMonitor, error) {
	if d == nil || len(d.monitors) == 0 {
		return nil, nil
	}

	out := make([]protocol.AgentDiscoveredMonitor, 0, len(d.monitors))
	for _, monitor := range d.monitors {
		discovered, err := staticMonitor(monitor)
		if err != nil {
			return nil, err
		}
		out = append(out, discovered)
	}
	return out, nil
}

func (d *StaticDiscoverer) Close(context.Context) error {
	return nil
}

func staticMonitor(monitor StaticMonitor) (protocol.AgentDiscoveredMonitor, error) {
	name := strings.TrimSpace(monitor.Name)
	monitorType := strings.ToLower(strings.TrimSpace(monitor.Type))
	target := strings.TrimSpace(monitor.Target)
	sourceKey := strings.TrimSpace(monitor.SourceKey)

	if name == "" {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("static monitor name is required")
	}
	if monitorType == "" {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("static monitor %q type is required", name)
	}
	if target == "" {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("static monitor %q target is required", name)
	}
	if sourceKey == "" {
		sourceKey = "static:" + name
	}

	return protocol.AgentDiscoveredMonitor{
		SourceKey:         sourceKey,
		Name:              name,
		Type:              monitorType,
		Target:            target,
		EnvironmentCode:   strings.TrimSpace(monitor.EnvironmentCode),
		Enabled:           monitor.Enabled,
		IntervalSeconds:   seconds(monitor.Interval),
		TimeoutSeconds:    seconds(monitor.Timeout),
		RetryCount:        monitor.RetryCount,
		AggregationPolicy: strings.TrimSpace(monitor.AggregationPolicy),
	}, nil
}

func seconds(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	return int(duration.Seconds())
}
