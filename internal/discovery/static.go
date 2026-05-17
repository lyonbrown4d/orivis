package discovery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

type StaticMonitor struct {
	SourceKey         string        `mapstructure:"source_key"`
	Name              string        `mapstructure:"name"`
	Type              string        `mapstructure:"type"`
	Target            string        `mapstructure:"target"`
	GroupName         string        `mapstructure:"group"`
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

	monitors, err := collectionlist.ReduceErrList(
		collectionlist.NewList(d.monitors...),
		collectionlist.NewListWithCapacity[protocol.AgentDiscoveredMonitor](len(d.monitors)),
		func(out *collectionlist.List[protocol.AgentDiscoveredMonitor], _ int, monitor StaticMonitor) (*collectionlist.List[protocol.AgentDiscoveredMonitor], error) {
			discovered, err := staticMonitor(monitor)
			if err != nil {
				return nil, fmt.Errorf("decode static monitor: %w", err)
			}
			out.Add(discovered)
			return out, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("discover static monitors: %w", err)
	}
	return monitors.Values(), nil
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
		return protocol.AgentDiscoveredMonitor{}, errors.New("static monitor name is required")
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
		GroupName:         strings.TrimSpace(monitor.GroupName),
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
