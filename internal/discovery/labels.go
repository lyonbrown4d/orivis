package discovery

import (
	"errors"
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
		return nil, errors.New("source key is required")
	}
	if !labelBool(source.Labels[LabelEnable], true) {
		return nil, nil
	}
	return parseMonitorGroups(sourceKey, source.Labels[LabelEnvironment], monitorLabelGroups(source.Labels))
}

func monitorLabelGroups(labels map[string]string) *collectionmapping.Map[string, map[string]string] {
	groups := collectionmapping.NewMap[string, map[string]string]()
	for key, value := range labels {
		name, field, ok := monitorLabelField(key)
		if !ok {
			continue
		}
		fields, ok := groups.Get(name)
		if !ok {
			fields = map[string]string{}
			groups.Set(name, fields)
		}
		fields[field] = strings.TrimSpace(value)
	}
	return groups
}

func monitorLabelField(key string) (string, string, bool) {
	if !strings.HasPrefix(key, LabelMonitor) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, LabelMonitor)
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseMonitorGroups(
	sourceKey string,
	environment string,
	groups *collectionmapping.Map[string, map[string]string],
) ([]protocol.AgentDiscoveredMonitor, error) {
	names := groups.Keys()
	sort.Strings(names)

	monitors := make([]protocol.AgentDiscoveredMonitor, 0, len(names))
	for _, name := range names {
		fields, _ := groups.Get(name)
		monitor, err := parseMonitor(sourceKey, environment, name, fields)
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

	timing, err := parseMonitorTiming(key, fields)
	if err != nil {
		return protocol.AgentDiscoveredMonitor{}, err
	}
	name := monitorName(key, fields["name"])
	enabled := labelBool(fields["enabled"], true)

	return protocol.AgentDiscoveredMonitor{
		SourceKey:         sourceKey + ":" + key,
		Name:              name,
		Type:              monitorType,
		Target:            target,
		EnvironmentCode:   strings.TrimSpace(environment),
		Enabled:           &enabled,
		IntervalSeconds:   timing.interval,
		TimeoutSeconds:    timing.timeout,
		RetryCount:        timing.retryCount,
		AggregationPolicy: strings.TrimSpace(fields["aggregation"]),
	}, nil
}

type monitorTiming struct {
	interval   int
	timeout    int
	retryCount int
}

func parseMonitorTiming(key string, fields map[string]string) (monitorTiming, error) {
	interval, err := labelSeconds(fields["interval"])
	if err != nil {
		return monitorTiming{}, fmt.Errorf("monitor %q interval: %w", key, err)
	}
	timeout, err := labelSeconds(fields["timeout"])
	if err != nil {
		return monitorTiming{}, fmt.Errorf("monitor %q timeout: %w", key, err)
	}
	retryCount, err := labelInt(fields["retry"])
	if err != nil {
		return monitorTiming{}, fmt.Errorf("monitor %q retry: %w", key, err)
	}
	return monitorTiming{interval: interval, timeout: timeout, retryCount: retryCount}, nil
}

func monitorName(key, value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return key
	}
	return name
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
		return 0, fmt.Errorf("parse label duration: %w", err)
	}
	return int(duration.Seconds()), nil
}

func labelInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse label int: %w", err)
	}
	return parsed, nil
}
