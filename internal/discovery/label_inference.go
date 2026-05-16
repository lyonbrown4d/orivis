package discovery

import (
	"fmt"
	"maps"
	"strings"
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
	out := make(map[string]string, len(fields)+3)
	maps.Copy(out, fields)
	return out
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
	for _, want := range preferredMonitorPorts(monitorType) {
		for _, port := range ports {
			if port == want {
				return port
			}
		}
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
	switch field {
	case "type", "target", "name", "enabled", "interval", "timeout", "retry", "aggregation":
		return true
	default:
		return false
	}
}

func defaultMonitorKey(defaultName string) string {
	name := strings.TrimSpace(defaultName)
	if name == "" {
		return "default"
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
