package discovery

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
)

func inferredMonitorFields(source LabelSource, fields map[string]string) (map[string]string, bool) {
	out := cloneFields(fields)
	monitorType := strings.ToLower(strings.TrimSpace(out["type"]))
	if monitorType == "" {
		monitorType = inferMonitorType(source.Ports)
		if monitorType != "" {
			out["type"] = monitorType
		}
	}
	if strings.TrimSpace(out["target"]) == "" {
		if target := inferMonitorTarget(monitorType, source.TargetHost, source.Ports); target != "" {
			out["target"] = target
		}
	}
	if strings.TrimSpace(out["name"]) == "" {
		if name := strings.TrimSpace(source.DefaultName); name != "" {
			out["name"] = name
		}
	}
	return out, strings.TrimSpace(out["type"]) != "" || strings.TrimSpace(out["target"]) != ""
}

func cloneFields(fields map[string]string) map[string]string {
	return lo.Assign(map[string]string{}, fields)
}

func inferMonitorType(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	return "tcp"
}

func inferMonitorTarget(monitorType, host string, ports []int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	port := selectMonitorPort(monitorType, ports)
	if port == 0 {
		return ""
	}
	switch monitorType {
	case "http":
		return fmt.Sprintf("http://%s:%d", host, port)
	case "redis":
		return fmt.Sprintf("redis://%s:%d", host, port)
	case "tcp":
		return fmt.Sprintf("%s:%d", host, port)
	default:
		return ""
	}
}

func selectMonitorPort(monitorType string, ports []int) int {
	if len(ports) == 0 {
		return defaultMonitorPort(monitorType)
	}
	port, ok := lo.Find(preferredMonitorPorts(monitorType), func(want int) bool {
		return lo.Contains(ports, want)
	})
	if ok {
		return port
	}
	return ports[0]
}

func defaultMonitorPort(monitorType string) int {
	switch monitorType {
	case "http":
		return 80
	case "redis":
		return 6379
	default:
		return 0
	}
}

func preferredMonitorPorts(monitorType string) []int {
	switch monitorType {
	case "http":
		return []int{80, 8080, 3000, 8000}
	case "redis":
		return []int{6379}
	default:
		return nil
	}
}

func monitorField(field string) bool {
	return lo.Contains([]string{"type", "target", "name", "group", "enabled", "interval", "timeout", "retry", "aggregation"}, field)
}

func defaultMonitorKey(defaultName string) string {
	name := strings.TrimSpace(defaultName)
	if name == "" {
		return "default"
	}
	return name
}

func firstNonEmpty(values ...string) string {
	value, ok := lo.Find(values, func(value string) bool {
		return strings.TrimSpace(value) != ""
	})
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
