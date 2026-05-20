package config

import (
	"runtime"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/discovery"
)

func normalizePollConfig(cfg *Config) {
	if cfg.Poll.Workers <= 0 {
		cfg.Poll.Workers = runtime.NumCPU()
		if cfg.Poll.Workers <= 0 {
			cfg.Poll.Workers = 1
		}
	}
}

func normalizeStaticMonitors(single discovery.StaticMonitor, monitors, hclMonitors []discovery.StaticMonitor) []discovery.StaticMonitor {
	out := collectionlist.NewListWithCapacity[discovery.StaticMonitor](len(monitors) + len(hclMonitors) + 1)
	if hasStaticMonitor(single) {
		out.Add(single)
	}
	out.Add(monitors...)
	out.Add(hclMonitors...)
	return out.Values()
}

func hasStaticMonitor(monitor discovery.StaticMonitor) bool {
	return strings.TrimSpace(monitor.SourceKey) != "" ||
		strings.TrimSpace(monitor.Name) != "" ||
		strings.TrimSpace(monitor.Type) != "" ||
		strings.TrimSpace(monitor.Target) != "" ||
		strings.TrimSpace(monitor.EnvironmentCode) != ""
}

func normalizeStringSlice(values []string) []string {
	parts := collectionlist.FlatMapList(
		collectionlist.NewList(values...),
		func(_ int, value string) []string {
			return strings.Split(value, ",")
		},
	)
	return collectionlist.FilterMapList(parts, func(_ int, part string) (string, bool) {
		part = strings.TrimSpace(part)
		return part, part != ""
	}).Values()
}
