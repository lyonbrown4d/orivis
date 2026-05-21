package discovery

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const (
	LabelEnable      = "orivis.enable"
	LabelEnvironment = "orivis.environment"
	LabelGroup       = "orivis.group"
	LabelMonitor     = "orivis.monitor."
)

type LabelSource struct {
	SourceKey          string
	Labels             map[string]string
	DefaultName        string
	DefaultEnvironment string
	DefaultGroup       string
	TargetHost         string
	ImageName          string
	Ports              []int
}

func ParseLabels(source LabelSource) ([]protocol.AgentDiscoveredMonitor, error) {
	sourceKey := strings.TrimSpace(source.SourceKey)
	if sourceKey == "" {
		return nil, errors.New("source key is required")
	}
	enableValue := strings.TrimSpace(source.Labels[LabelEnable])
	if !labelBool(enableValue, true) {
		return nil, nil
	}
	environment := firstNonEmpty(source.Labels[LabelEnvironment], source.DefaultEnvironment)
	group := firstNonEmpty(source.Labels[LabelGroup], source.DefaultGroup)
	groups := monitorLabelGroups(source.Labels, source.DefaultName)
	if groups.Len() == 0 {
		if !labelBool(enableValue, false) {
			return nil, nil
		}
		fields, ok := inferredMonitorFields(source, nil)
		if !ok {
			return nil, nil
		}
		groups.Set(defaultMonitorKey(source.DefaultName), fields)
	}
	return parseMonitorGroups(sourceKey, environment, group, source, groups)
}

func monitorLabelGroups(labels map[string]string, defaultName string) *collectionmapping.Map[string, map[string]string] {
	groups := collectionmapping.NewMap[string, map[string]string]()
	for key, value := range labels {
		name, field, ok := monitorLabelField(key, defaultName)
		if !ok {
			continue
		}
		fields := groups.GetOption(name).OrElse(map[string]string{})
		fields[field] = strings.TrimSpace(value)
		groups.Set(name, fields)
	}
	return groups
}

func monitorLabelField(key, defaultName string) (string, string, bool) {
	if !strings.HasPrefix(key, LabelMonitor) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, LabelMonitor)
	if monitorField(rest) {
		return defaultMonitorKey(defaultName), rest, true
	}
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseMonitorGroups(
	sourceKey string,
	environment string,
	defaultGroup string,
	source LabelSource,
	groups *collectionmapping.Map[string, map[string]string],
) ([]protocol.AgentDiscoveredMonitor, error) {
	names := groups.Keys()
	sort.Strings(names)

	monitors, err := collectionlist.ReduceErrList(
		collectionlist.NewList(names...),
		collectionlist.NewListWithCapacity[protocol.AgentDiscoveredMonitor](len(names)),
		func(out *collectionlist.List[protocol.AgentDiscoveredMonitor], _ int, name string) (*collectionlist.List[protocol.AgentDiscoveredMonitor], error) {
			fields := groups.GetOrDefault(name, nil)
			fields, _ = inferredMonitorFields(source, fields)
			monitor, err := parseMonitor(sourceKey, environment, defaultGroup, name, fields)
			if err != nil {
				return nil, err
			}
			out.Add(monitor)
			return out, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("parse monitor label groups: %w", err)
	}
	return monitors.Values(), nil
}

func parseMonitor(sourceKey, environment, defaultGroup, key string, fields map[string]string) (protocol.AgentDiscoveredMonitor, error) {
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
	groupName := firstNonEmpty(fields["group"], defaultGroup)
	enabled := labelBool(fields["enabled"], true)

	return protocol.AgentDiscoveredMonitor{
		SourceKey:         sourceKey + ":" + key,
		Name:              name,
		Type:              monitorType,
		Target:            target,
		GroupName:         strings.TrimSpace(groupName),
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
