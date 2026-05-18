package probe

import (
	"context"
	"net/url"
	"strings"

	"github.com/nats-io/nats.go"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const defaultNATSPort = "4222"

func (c *Checker) checkNATS(_ context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, detail, err := natsProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, detail, err
	}

	timeout := taskTimeout(task)
	conn, err := nats.Connect(target, nats.Name("orivis-agent"), nats.NoReconnect(), nats.Timeout(timeout))
	if err != nil {
		return model.StatusDown, detail, wrapError(err, "execute NATS probe")
	}
	defer conn.Close()

	if err := conn.FlushTimeout(timeout); err != nil {
		return model.StatusDown, detail, wrapError(err, "flush NATS probe")
	}
	return model.StatusUp, detail, nil
}

func natsProbeTarget(raw string) (string, map[string]any, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", map[string]any{"target": raw}, errorf("NATS probe target is empty")
	}
	if !strings.Contains(target, "://") {
		target = "nats://" + ensureHostPort(target, defaultNATSPort)
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return "", map[string]any{"target": raw}, wrapError(err, "parse NATS probe target")
	}
	switch strings.ToLower(parsed.Scheme) {
	case string(model.MonitorNATS), "tls", "ws", "wss":
		if parsed.Scheme == string(model.MonitorNATS) || parsed.Scheme == "tls" {
			parsed.Host = ensureHostPort(parsed.Host, defaultNATSPort)
		}
	default:
		return "", map[string]any{"target": raw, "scheme": parsed.Scheme}, errorf("unsupported NATS target scheme %q", parsed.Scheme)
	}

	normalized := parsed.String()
	return normalized, map[string]any{"target": normalized, "scheme": parsed.Scheme}, nil
}
