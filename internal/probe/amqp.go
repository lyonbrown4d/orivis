package probe

import (
	"context"
	"net"
	"net/url"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const (
	defaultAMQPPort  = "5672"
	defaultAMQPSPort = "5671"
)

func (c *Checker) checkAMQP(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, detail, err := amqpProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, detail, err
	}

	timeout := taskTimeout(task)
	dialer := net.Dialer{Timeout: timeout}
	conn, err := amqp.DialConfig(target, amqp.Config{
		Heartbeat: timeout,
		Dial: func(network string, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, address)
		},
	})
	if err != nil {
		return model.StatusDown, detail, wrapError(err, "execute AMQP probe")
	}
	closeSilently(conn)
	return model.StatusUp, detail, nil
}

func amqpProbeTarget(raw string) (string, map[string]any, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", map[string]any{"target": raw}, errorf("AMQP probe target is empty")
	}
	if !strings.Contains(target, "://") {
		target = "amqp://" + ensureHostPort(target, defaultAMQPPort)
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return "", map[string]any{"target": raw}, wrapError(err, "parse AMQP probe target")
	}

	switch strings.ToLower(parsed.Scheme) {
	case string(model.MonitorAMQP):
		parsed.Host = ensureHostPort(parsed.Host, defaultAMQPPort)
	case "amqps":
		parsed.Host = ensureHostPort(parsed.Host, defaultAMQPSPort)
	case string(model.MonitorRabbitMQ):
		parsed.Scheme = string(model.MonitorAMQP)
		parsed.Host = ensureHostPort(parsed.Host, defaultAMQPPort)
	default:
		return "", map[string]any{"target": raw, "scheme": parsed.Scheme}, errorf("unsupported AMQP target scheme %q", parsed.Scheme)
	}

	normalized := parsed.String()
	return normalized, map[string]any{"target": normalized, "scheme": parsed.Scheme}, nil
}
