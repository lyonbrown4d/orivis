package probe

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arcgolabs/clientx"
	clienthttp "github.com/arcgolabs/clientx/http"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const defaultTimeout = 5 * time.Second
const probeHTTPClientTimeout = 24 * time.Hour

type Checker struct {
	httpClient    clienthttp.Client
	httpClientErr error
	resolver      *net.Resolver
}

type Result struct {
	Status       model.Status
	Latency      time.Duration
	ErrorMessage string
	CheckedAt    time.Time
	RawDetail    []byte
}

func New() *Checker {
	httpClient, err := newProbeHTTPClient()
	return &Checker{
		httpClient:    httpClient,
		httpClientErr: err,
		resolver:      net.DefaultResolver,
	}
}

func (c *Checker) Close() error {
	if c == nil || c.httpClient == nil {
		return nil
	}
	if err := c.httpClient.Close(); err != nil {
		return wrapError(err, "close HTTP probe client")
	}
	return nil
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
		return c.productionProbeChecker(probeType)
	}
}

func (c *Checker) productionProbeChecker(probeType string) probeChecker {
	switch probeType {
	case string(model.MonitorMongoDB), "mongo":
		return c.checkMongoDB
	case string(model.MonitorRabbitMQ), string(model.MonitorAMQP):
		return c.checkAMQP
	case string(model.MonitorNATS):
		return c.checkNATS
	case string(model.MonitorKafka):
		return c.checkKafka
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
	if c.httpClientErr != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, wrapError(c.httpClientErr, "initialize HTTP probe client")
	}
	if c.httpClient == nil {
		return model.StatusDown, map[string]any{"target": task.Target}, errorf("HTTP probe client is not initialized")
	}
	resp, err := c.httpClient.Execute(
		ctx,
		c.httpClient.R().SetDoNotParseResponse(true),
		http.MethodGet,
		task.Target,
	)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, wrapError(err, "execute HTTP probe")
	}
	if resp != nil && resp.Body != nil {
		defer closeSilently(resp.Body)
	}

	statusCode := resp.StatusCode()
	detail := map[string]any{"status_code": statusCode}
	if statusCode >= 200 && statusCode < 400 {
		return model.StatusUp, detail, nil
	}
	return model.StatusDown, detail, errorf("http status %d", statusCode)
}

func newProbeHTTPClient() (clienthttp.Client, error) {
	client, err := clienthttp.New(
		clienthttp.Config{
			Timeout:   probeHTTPClientTimeout,
			UserAgent: "orivis-probe/" + buildinfo.Version,
		},
		clienthttp.WithPolicies(clientx.NewTimeoutPolicy(probeHTTPClientTimeout)),
	)
	if err != nil {
		return nil, wrapError(err, "create HTTP probe client")
	}
	return client, nil
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
