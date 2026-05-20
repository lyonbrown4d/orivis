package client

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/arcgolabs/clientx"
	clienthttp "github.com/arcgolabs/clientx/http"
	"github.com/arcgolabs/observabilityx"
	"github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"resty.dev/v3"
)

type Client struct {
	HTTP        clienthttp.Client
	logger      *slog.Logger
	retry       retryConfig
	gzipResults bool
}

func New(cfg config.Config, logger *slog.Logger, obs observabilityx.Observability) (*Client, error) {
	obs = observabilityx.Normalize(obs, logger)

	httpClient, err := clienthttp.New(
		clienthttp.Config{
			BaseURL:   cfg.Server.URL,
			Timeout:   cfg.Transport.RequestTimeout,
			UserAgent: "orivis-agent/" + buildinfo.Version,
		},
		agentHTTPTransportOption(cfg),
		clienthttp.WithHooks(
			clientx.NewLoggingHook(logger),
			clientx.NewObservabilityHook(obs, clientx.WithHookMetricPrefix("orivis_agent_client")),
		),
		clienthttp.WithPolicies(
			clientx.NewTimeoutPolicy(cfg.Transport.RequestTimeout),
		),
	)
	if err != nil {
		return nil, wrapError(err, "create agent HTTP client")
	}

	return &Client{
		HTTP:        httpClient,
		logger:      logger,
		retry:       newRetryConfig(cfg),
		gzipResults: cfg.Transport.GzipResults,
	}, nil
}

func (c *Client) Close(context.Context) error {
	if c == nil || c.HTTP == nil {
		return nil
	}
	if err := c.HTTP.Close(); err != nil {
		return wrapError(err, "close agent HTTP client")
	}
	return nil
}

func (c *Client) Register(ctx context.Context, req protocol.AgentRegisterRequest) (protocol.AgentRegisterResponse, error) {
	var out protocol.AgentRegisterResponse
	resp, err := c.execute(
		ctx,
		func() *resty.Request {
			return c.HTTP.R().SetBody(req).SetResult(&out)
		},
		http.MethodPost,
		"/api/agent/register",
	)
	if err != nil {
		return out, wrapError(err, "execute register agent request")
	}
	if resp.IsError() {
		return out, errorf("register agent: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) Heartbeat(ctx context.Context, req protocol.AgentHeartbeatRequest) (protocol.AgentHeartbeatResponse, error) {
	var out protocol.AgentHeartbeatResponse
	resp, err := c.execute(
		ctx,
		func() *resty.Request {
			return c.HTTP.R().SetBody(req).SetResult(&out)
		},
		http.MethodPost,
		"/api/agent/heartbeat",
	)
	if err != nil {
		return out, wrapError(err, "execute heartbeat request")
	}
	if resp.IsError() {
		return out, errorf("heartbeat agent: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) Tasks(ctx context.Context, req protocol.AgentTasksRequest) (protocol.AgentTasksResponse, error) {
	var out protocol.AgentTasksResponse
	resp, err := c.execute(ctx, func() *resty.Request {
		request := c.HTTP.R().
			SetQueryParam("agent_id", req.AgentID).
			SetResult(&out)
		if req.Token != "" {
			request.SetQueryParam("token", req.Token)
		}
		return request
	}, http.MethodGet, "/api/agent/tasks")
	if err != nil {
		return out, wrapError(err, "execute tasks request")
	}
	if resp.IsError() {
		return out, errorf("pull agent tasks: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) SyncMonitors(ctx context.Context, req protocol.AgentMonitorSyncRequest) (protocol.AgentMonitorSyncResponse, error) {
	var out protocol.AgentMonitorSyncResponse
	resp, err := c.execute(
		ctx,
		func() *resty.Request {
			return c.HTTP.R().SetBody(req).SetResult(&out)
		},
		http.MethodPost,
		"/api/agent/monitors",
	)
	if err != nil {
		return out, wrapError(err, "execute monitor sync request")
	}
	if resp.IsError() {
		return out, errorf("sync agent monitors: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) ReportResult(ctx context.Context, req protocol.AgentResultRequest) error {
	body, err := c.resultRequestBody(req)
	if err != nil {
		return err
	}
	resp, err := c.execute(ctx, func() *resty.Request {
		request := c.HTTP.R()
		if c.gzipResults {
			request.SetHeader("Content-Encoding", "gzip")
			request.SetHeader("Content-Type", "application/json")
		}
		return request.SetBody(body)
	}, http.MethodPost, "/api/agent/results")
	if err != nil {
		return wrapError(err, "execute report result request")
	}
	if resp.IsError() {
		return errorf("report agent result: server returned %s: %s", resp.Status(), resp.String())
	}
	return nil
}

func (c *Client) ReportResults(ctx context.Context, req protocol.AgentResultBatchRequest) (protocol.AgentResultBatchResponse, error) {
	var out protocol.AgentResultBatchResponse
	body, err := c.resultBatchRequestBody(req)
	if err != nil {
		return out, err
	}
	resp, err := c.execute(ctx, func() *resty.Request {
		request := c.HTTP.R().SetResult(&out)
		if c.gzipResults {
			request.SetHeader("Content-Encoding", "gzip")
			request.SetHeader("Content-Type", "application/json")
		}
		return request.SetBody(body)
	}, http.MethodPost, "/api/agent/results/batch")
	if err != nil {
		return out, wrapError(err, "execute report result batch request")
	}
	if resp.IsError() {
		return out, errorf("report agent result batch: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}
