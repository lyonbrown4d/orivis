package probe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
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

	status, detail, err := c.check(checkCtx, task)
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

func (c *Checker) check(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	probeType := strings.ToLower(strings.TrimSpace(task.Type))
	checker := c.probeChecker(probeType)
	if checker == nil {
		return model.StatusUnknown, map[string]any{"type": task.Type}, errorf("unsupported monitor type %q", task.Type)
	}
	return checker(ctx, task)
}

type probeChecker func(context.Context, protocol.AgentTask) (model.Status, map[string]any, error)

func (c *Checker) probeChecker(probeType string) probeChecker {
	if isDatabaseProbeType(probeType) {
		return c.checkDatabase
	}
	if checker := c.networkProbeChecker(probeType); checker != nil {
		return checker
	}
	return c.serviceProbeChecker(probeType)
}

func (c *Checker) networkProbeChecker(probeType string) probeChecker {
	switch probeType {
	case string(model.MonitorHTTP):
		return c.checkHTTP
	case string(model.MonitorTCP):
		return c.checkTCP
	case string(model.MonitorUDP):
		return c.checkUDP
	case string(model.MonitorPing):
		return c.checkPing
	default:
		return nil
	}
}

func (c *Checker) serviceProbeChecker(probeType string) probeChecker {
	switch probeType {
	case string(model.MonitorDNS):
		return c.checkDNS
	case string(model.MonitorTLS):
		return c.checkTLS
	case string(model.MonitorSMTP):
		return c.checkSMTP
	case string(model.MonitorRedis):
		return c.checkRedis
	case string(model.MonitorMemcached):
		return c.checkMemcached
	default:
		return nil
	}
}

func isDatabaseProbeType(probeType string) bool {
	switch probeType {
	case string(model.MonitorDatabase), string(model.MonitorSQLite), string(model.MonitorMySQL), string(model.MonitorPostgres), "db", "pg", "postgresql":
		return true
	default:
		return false
	}
}

func (c *Checker) checkHTTP(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.Target, http.NoBody)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, wrapError(err, "build HTTP probe request")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, wrapError(err, "execute HTTP probe")
	}
	defer closeSilently(resp.Body)

	detail := map[string]any{"status_code": resp.StatusCode}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return model.StatusUp, detail, nil
	}
	return model.StatusDown, detail, errorf("http status %d", resp.StatusCode)
}

func (c *Checker) checkTCP(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", task.Target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, wrapError(err, "execute TCP probe")
	}
	closeSilently(conn)
	return model.StatusUp, map[string]any{"target": task.Target}, nil
}

func (c *Checker) checkDNS(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	host := dnsTargetHost(task.Target)
	ips, err := c.resolver.LookupHost(ctx, host)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target, "host": host}, wrapError(err, "execute DNS probe")
	}
	return model.StatusUp, map[string]any{"target": task.Target, "host": host, "answers": ips}, nil
}

func (c *Checker) checkTLS(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	host, _, err := net.SplitHostPort(task.Target)
	if err != nil {
		host = task.Target
	}
	dialer := tls.Dialer{Config: &tls.Config{ServerName: host}}
	conn, err := dialer.DialContext(ctx, "tcp", task.Target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target, "server_name": host}, wrapError(err, "execute TLS probe")
	}
	defer closeSilently(conn)

	detail := map[string]any{"target": task.Target, "server_name": host}
	if tlsConn, ok := conn.(*tls.Conn); ok {
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			detail["not_after"] = state.PeerCertificates[0].NotAfter
		}
	}
	return model.StatusUp, detail, nil
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

func closeSilently(closer io.Closer) {
	if closer == nil {
		return
	}
	if err := closer.Close(); err != nil {
		return
	}
}
