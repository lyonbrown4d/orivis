package probe

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const defaultMemcachedPort = "11211"

func (c *Checker) checkMemcached(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target, err := memcachedProbeTarget(task.Target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, err
	}
	conn, err := new(net.Dialer).DialContext(ctx, "tcp", target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": target}, wrapError(err, "execute memcached probe")
	}
	defer closeSilently(conn)
	if deadline, ok := ctx.Deadline(); ok {
		if deadlineErr := conn.SetDeadline(deadline); deadlineErr != nil {
			return model.StatusDown, map[string]any{"target": target}, wrapError(deadlineErr, "set memcached probe deadline")
		}
	}
	if _, writeErr := io.WriteString(conn, "version\r\n"); writeErr != nil {
		return model.StatusDown, map[string]any{"target": target}, wrapError(writeErr, "write memcached version command")
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return model.StatusDown, map[string]any{"target": target}, wrapError(err, "read memcached version response")
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "VERSION") {
		return model.StatusDown, map[string]any{"target": target, "response": line}, errorf("unexpected memcached response %q", line)
	}
	return model.StatusUp, map[string]any{"target": target, "version": strings.TrimSpace(strings.TrimPrefix(line, "VERSION"))}, nil
}

func memcachedProbeTarget(rawTarget string) (string, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return "", newError("memcached target is empty")
	}
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" {
		if parsed.Scheme != string(model.MonitorMemcached) {
			return "", errorf("unsupported memcached probe scheme %q", parsed.Scheme)
		}
		target = parsed.Host
	}
	target = ensureHostPort(target, defaultMemcachedPort)
	if _, _, err := net.SplitHostPort(target); err != nil {
		return "", wrapError(err, "parse memcached target")
	}
	return target, nil
}
