package discovery

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
)

func inferredMonitorFields(source LabelSource, fields map[string]string) (map[string]string, bool) {
	out := cloneFields(fields)
	ensureInferredType(source, out)
	ensureInferredTarget(source, out)
	ensureInferredName(source, out)
	return out, isMonitorFieldsUsable(out)
}

func ensureInferredType(source LabelSource, out map[string]string) {
	if strings.TrimSpace(out["type"]) != "" {
		return
	}
	monitorType := inferMonitorTypeFromImage(source.ImageName)
	if monitorType == "" {
		monitorType = inferMonitorType(source.Ports)
	}
	if monitorType != "" {
		out["type"] = monitorType
	}
}

func ensureInferredTarget(source LabelSource, out map[string]string) {
	if strings.TrimSpace(out["target"]) != "" {
		return
	}
	monitorType := strings.TrimSpace(out["type"])
	if target := inferMonitorTarget(monitorType, source.TargetHost, source.Ports); target != "" {
		out["target"] = target
	}
}

func ensureInferredName(source LabelSource, out map[string]string) {
	if strings.TrimSpace(out["name"]) != "" {
		return
	}
	if name := strings.TrimSpace(source.DefaultName); name != "" {
		out["name"] = name
	}
}

func isMonitorFieldsUsable(out map[string]string) bool {
	return strings.TrimSpace(out["type"]) != "" || strings.TrimSpace(out["target"]) != ""
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

func inferMonitorTypeFromImage(imageName string) string {
	normalizedImage := strings.ToLower(strings.TrimSpace(imageName))
	if normalizedImage == "" {
		return ""
	}
	normalizedImage = dockerImageName(normalizedImage)
	switch normalizedImage {
	case "nginx", "caddy", "traefik", "haproxy", "apache", "httpd", "flask", "tomcat":
		return "http"
	}

	if strings.Contains(normalizedImage, "kafka") {
		return "kafka"
	}
	if strings.Contains(normalizedImage, "redis") {
		return "redis"
	}
	if strings.Contains(normalizedImage, "rabbitmq") {
		return "rabbitmq"
	}
	if strings.Contains(normalizedImage, "mongo") {
		return "mongo"
	}
	if strings.Contains(normalizedImage, "mysql") {
		return "mysql"
	}
	if strings.Contains(normalizedImage, "postgres") {
		return "postgres"
	}
	return ""
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
	case "kafka", "mysql", "postgres", "mongo", "rabbitmq", "amqp", "postgresql":
		return fmt.Sprintf("%s:%d", host, port)
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
	case "kafka":
		return 9092
	case "redis":
		return 6379
	case "mysql":
		return 3306
	case "postgres", "postgresql":
		return 5432
	case "mongo", "mongodb":
		return 27017
	case "rabbitmq", "amqp":
		return 5672
	default:
		return 0
	}
}

func preferredMonitorPorts(monitorType string) []int {
	switch monitorType {
	case "http":
		return []int{80, 8080, 3000, 8000}
	case "kafka":
		return []int{9092, 19092}
	case "redis":
		return []int{6379}
	case "mysql":
		return []int{3306}
	case "postgres", "postgresql":
		return []int{5432}
	case "mongo", "mongodb":
		return []int{27017}
	case "rabbitmq", "amqp":
		return []int{5672}
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
