package probe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/shared/model"
	"github.com/lyonbrown4d/orivis/internal/shared/protocol"
)

const defaultTimeout = 5 * time.Second

type Checker struct {
	httpClient *http.Client
	resolver   *net.Resolver
}

type Result struct {
	Status       model.Status
	Latency      time.Duration
	ErrorMessage string
	CheckedAt    time.Time
	RawDetail    []byte
}

func New() *Checker {
	return &Checker{
		httpClient: &http.Client{},
		resolver:   net.DefaultResolver,
	}
}

func (c *Checker) Check(ctx context.Context, task protocol.AgentTask) Result {
	start := time.Now().UTC()
	checkCtx, cancel := context.WithTimeout(ctx, taskTimeout(task))
	defer cancel()

	status, err, detail := c.check(checkCtx, task)
	result := Result{
		Status:       status,
		Latency:      time.Since(start),
		CheckedAt:    start,
		ErrorMessage: errorString(err),
		RawDetail:    detailBytes(detail),
	}
	if err != nil && result.Status == "" {
		result.Status = model.StatusDown
	}
	if result.Status == "" {
		result.Status = model.StatusUp
	}
	return result
}

func (c *Checker) check(ctx context.Context, task protocol.AgentTask) (model.Status, error, map[string]any) {
	switch strings.ToLower(strings.TrimSpace(task.Type)) {
	case string(model.MonitorHTTP):
		return c.checkHTTP(ctx, task)
	case string(model.MonitorTCP):
		return c.checkTCP(ctx, task)
	case string(model.MonitorDNS):
		return c.checkDNS(ctx, task)
	case string(model.MonitorTLS):
		return c.checkTLS(ctx, task)
	case string(model.MonitorPing):
		return model.StatusUnknown, fmt.Errorf("ping probe is not implemented yet"), map[string]any{"type": task.Type}
	default:
		return model.StatusUnknown, fmt.Errorf("unsupported monitor type %q", task.Type), map[string]any{"type": task.Type}
	}
}

func (c *Checker) checkHTTP(ctx context.Context, task protocol.AgentTask) (model.Status, error, map[string]any) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.Target, nil)
	if err != nil {
		return model.StatusDown, err, map[string]any{"target": task.Target}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.StatusDown, err, map[string]any{"target": task.Target}
	}
	defer resp.Body.Close()

	detail := map[string]any{"status_code": resp.StatusCode}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return model.StatusUp, nil, detail
	}
	return model.StatusDown, fmt.Errorf("http status %d", resp.StatusCode), detail
}

func (c *Checker) checkTCP(ctx context.Context, task protocol.AgentTask) (model.Status, error, map[string]any) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", task.Target)
	if err != nil {
		return model.StatusDown, err, map[string]any{"target": task.Target}
	}
	_ = conn.Close()
	return model.StatusUp, nil, map[string]any{"target": task.Target}
}

func (c *Checker) checkDNS(ctx context.Context, task protocol.AgentTask) (model.Status, error, map[string]any) {
	host := dnsTargetHost(task.Target)
	ips, err := c.resolver.LookupHost(ctx, host)
	if err != nil {
		return model.StatusDown, err, map[string]any{"target": task.Target, "host": host}
	}
	return model.StatusUp, nil, map[string]any{"target": task.Target, "host": host, "answers": ips}
}

func (c *Checker) checkTLS(ctx context.Context, task protocol.AgentTask) (model.Status, error, map[string]any) {
	host, _, err := net.SplitHostPort(task.Target)
	if err != nil {
		host = task.Target
	}
	dialer := tls.Dialer{Config: &tls.Config{ServerName: host}}
	conn, err := dialer.DialContext(ctx, "tcp", task.Target)
	if err != nil {
		return model.StatusDown, err, map[string]any{"target": task.Target, "server_name": host}
	}
	defer conn.Close()

	detail := map[string]any{"target": task.Target, "server_name": host}
	if tlsConn, ok := conn.(*tls.Conn); ok {
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			detail["not_after"] = state.PeerCertificates[0].NotAfter
		}
	}
	return model.StatusUp, nil, detail
}

func taskTimeout(task protocol.AgentTask) time.Duration {
	if task.TimeoutSeconds <= 0 {
		return defaultTimeout
	}
	return time.Duration(task.TimeoutSeconds) * time.Second
}

func dnsTargetHost(target string) string {
	if parsed, err := url.Parse(target); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	if host, _, err := net.SplitHostPort(target); err == nil {
		return host
	}
	return strings.TrimSpace(target)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func detailBytes(detail map[string]any) []byte {
	if len(detail) == 0 {
		return nil
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil
	}
	return raw
}
