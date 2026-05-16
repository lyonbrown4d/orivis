package discovery

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const (
	LabelEnable      = "orivis.enable"
	LabelEnvironment = "orivis.environment"
	LabelMonitor     = "orivis.monitor."
)

type LabelSource struct {
	SourceKey string
	Labels    map[string]string
}

func ParseLabels(source LabelSource) ([]protocol.AgentDiscoveredMonitor, error) {
	sourceKey := strings.TrimSpace(source.SourceKey)
	if sourceKey == "" {
		return nil, fmt.Errorf("source key is required")
	}
	if !labelBool(source.Labels[LabelEnable], true) {
		return nil, nil
	}

	groups := collectionmapping.NewMap[string, map[string]string]()
	for key, value := range source.Labels {
		if !strings.HasPrefix(key, LabelMonitor) {
			continue
		}
		rest := strings.TrimPrefix(key, LabelMonitor)
		parts := strings.SplitN(rest, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		name := parts[0]
		field := parts[1]
		fields, ok := groups.Get(name)
		if !ok {
			fields = map[string]string{}
			groups.Set(name, fields)
		}
		fields[field] = strings.TrimSpace(value)
	}

	names := groups.Keys()
	sort.Strings(names)

	monitors := make([]protocol.AgentDiscoveredMonitor, 0, len(names))
	for _, name := range names {
		fields, _ := groups.Get(name)
		monitor, err := parseMonitor(sourceKey, source.Labels[LabelEnvironment], name, fields)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, monitor)
	}
	return monitors, nil
}

func parseMonitor(sourceKey, environment, key string, fields map[string]string) (protocol.AgentDiscoveredMonitor, error) {
	monitorType := strings.ToLower(strings.TrimSpace(fields["type"]))
	target := strings.TrimSpace(fields["target"])
	if monitorType == "" {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("monitor %q type is required", key)
	}
	if target == "" {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("monitor %q target is required", key)
	}

	enabled := labelBool(fields["enabled"], true)
	interval, err := labelSeconds(fields["interval"])
	if err != nil {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("monitor %q interval: %w", key, err)
	}
	timeout, err := labelSeconds(fields["timeout"])
	if err != nil {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("monitor %q timeout: %w", key, err)
	}
	retryCount, err := labelInt(fields["retry"])
	if err != nil {
		return protocol.AgentDiscoveredMonitor{}, fmt.Errorf("monitor %q retry: %w", key, err)
	}

	name := strings.TrimSpace(fields["name"])
	if name == "" {
		name = key
	}

	return protocol.AgentDiscoveredMonitor{
		SourceKey:         sourceKey + ":" + key,
		Name:              name,
		Type:              monitorType,
		Target:            target,
		EnvironmentCode:   strings.TrimSpace(environment),
		Enabled:           &enabled,
		IntervalSeconds:   interval,
		TimeoutSeconds:    timeout,
		RetryCount:        retryCount,
		AggregationPolicy: strings.TrimSpace(fields["aggregation"]),
	}, nil
}

func labelBool(value string, fallback bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func labelSeconds(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return seconds, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	return int(duration.Seconds()), nil
}

func labelInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}
