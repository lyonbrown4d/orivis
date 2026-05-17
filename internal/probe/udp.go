package probe

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func (c *Checker) checkUDP(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, payload, expect, err := udpProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, err
	}
	conn, err := new(net.Dialer).DialContext(ctx, "udp", target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": target}, wrapError(err, "execute UDP probe")
	}
	defer closeSilently(conn)
	if deadline, ok := ctx.Deadline(); ok {
		if deadlineErr := conn.SetDeadline(deadline); deadlineErr != nil {
			return model.StatusDown, map[string]any{"target": target}, wrapError(deadlineErr, "set UDP probe deadline")
		}
	}
	if _, writeErr := conn.Write([]byte(payload)); writeErr != nil {
		return model.StatusDown, map[string]any{"target": target}, wrapError(writeErr, "write UDP probe payload")
	}
	if expect == "" {
		return model.StatusUp, map[string]any{"target": target, "payload_bytes": len(payload)}, nil
	}
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return model.StatusDown, map[string]any{"target": target}, wrapError(err, "read UDP probe response")
	}
	response := string(buffer[:n])
	if !strings.Contains(response, expect) {
		return model.StatusDown, map[string]any{"target": target, "response": response}, errorf("udp response does not contain %q", expect)
	}
	return model.StatusUp, map[string]any{"target": target, "response": response}, nil
}

func udpProbeTarget(rawTarget string) (string, string, string, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return "", "", "", newError("udp target is empty")
	}
	payload := "ping"
	expect := ""
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme == "udp" {
		target = parsed.Host
		query := parsed.Query()
		payload = firstNonEmpty(query.Get("payload"), payload)
		expect = query.Get("expect")
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return "", "", "", wrapError(err, "parse UDP target")
	}
	return target, payload, expect, nil
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
