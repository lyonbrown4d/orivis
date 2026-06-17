package discovery

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
)

type imageComponentProvider struct {
	monitorType string
	exact       []string
	contains    []string
}

var imageComponentProviders = []imageComponentProvider{
	{
		monitorType: "http",
		exact: []string{
			"adminer",
			"apache",
			"cadvisor",
			"caddy",
			"flask",
			"grafana",
			"haproxy",
			"httpd",
			"nginx",
			"pgadmin",
			"phpmyadmin",
			"pushgateway",
			"traefik",
			"tomcat",
		},
		contains: []string{
			"alertmanager",
			"blackbox-exporter",
			"cadvisor",
			"consul",
			"dozzle",
			"express",
			"gitea",
			"grafana",
			"homepage",
			"httpbin",
			"jaeger",
			"jenkins",
			"keycloak",
			"kibana",
			"minio",
			"nextjs",
			"node",
			"node-exporter",
			"pgadmin",
			"phpmyadmin",
			"prometheus",
			"pushgateway",
			"sonarqube",
			"spring",
			"uptime-kuma",
			"vault",
			"whoami",
			"wordpress",
			"zipkin",
		},
	},
	{monitorType: "kafka", contains: []string{"kafka", "redpanda"}},
	{monitorType: "redis", contains: []string{"dragonfly", "keydb", "redis", "valkey"}},
	{monitorType: "rabbitmq", contains: []string{"rabbitmq"}},
	{monitorType: "mongodb", contains: []string{"mongo"}},
	{monitorType: "mysql", contains: []string{"mariadb", "mysql", "percona"}},
	{monitorType: "postgres", contains: []string{"postgis", "postgres", "postgresql", "timescaledb"}},
	{monitorType: "memcached", contains: []string{"memcached"}},
	{monitorType: "nats", contains: []string{"nats"}},
	{monitorType: "smtp", contains: []string{"mailhog", "mailpit", "postfix", "smtp"}},
	{monitorType: "tcp", contains: []string{"cockroach", "etcd", "zookeeper"}},
}

var monitorTargetSchemes = map[string]string{
	"http":      "http",
	"redis":     "redis",
	"memcached": "memcached",
	"nats":      "nats",
	"smtp":      "smtp",
}

var plainMonitorTargetTypes = map[string]struct{}{
	"amqp":       {},
	"kafka":      {},
	"mongo":      {},
	"mongodb":    {},
	"mysql":      {},
	"postgres":   {},
	"postgresql": {},
	"rabbitmq":   {},
	"tcp":        {},
}

var defaultMonitorPorts = map[string]int{
	"http":       80,
	"kafka":      9092,
	"redis":      6379,
	"mysql":      3306,
	"postgres":   5432,
	"postgresql": 5432,
	"mongo":      27017,
	"mongodb":    27017,
	"rabbitmq":   5672,
	"amqp":       5672,
	"memcached":  11211,
	"nats":       4222,
	"smtp":       25,
}

var preferredMonitorPortValues = map[string][]int{
	"http":       {80, 8080, 3000, 3001, 8000, 8081, 8088, 9000, 9001, 9090, 9091, 9093, 9100, 9115, 5601, 16686, 9411},
	"tcp":        {2181, 2379, 26257},
	"kafka":      {9092, 19092, 29092},
	"redis":      {6379},
	"mysql":      {3306},
	"postgres":   {5432},
	"postgresql": {5432},
	"mongo":      {27017},
	"mongodb":    {27017},
	"rabbitmq":   {5672},
	"amqp":       {5672},
	"memcached":  {11211},
	"nats":       {4222},
	"smtp":       {25, 465, 587, 1025},
}

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
	monitorType := strings.ToLower(strings.TrimSpace(out["type"]))
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
	provider, ok := lo.Find(imageComponentProviders, func(provider imageComponentProvider) bool {
		return provider.matches(normalizedImage)
	})
	if !ok {
		return ""
	}
	return provider.monitorType
}

func (p imageComponentProvider) matches(image string) bool {
	if lo.Contains(p.exact, image) {
		return true
	}
	for _, value := range p.contains {
		if strings.Contains(image, value) {
			return true
		}
	}
	return false
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
	if scheme, ok := monitorTargetSchemes[monitorType]; ok {
		return fmt.Sprintf("%s://%s:%d", scheme, host, port)
	}
	if _, ok := plainMonitorTargetTypes[monitorType]; ok {
		return fmt.Sprintf("%s:%d", host, port)
	}
	return ""
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
	if port, ok := defaultMonitorPorts[monitorType]; ok {
		return port
	}
	return 0
}

func preferredMonitorPorts(monitorType string) []int {
	if ports, ok := preferredMonitorPortValues[monitorType]; ok {
		return ports
	}
	return nil
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
