package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/arcgolabs/clientx"
	clienthttp "github.com/arcgolabs/clientx/http"
	"github.com/arcgolabs/observabilityx"
	"github.com/lyonbrown4d/orivis/internal/agentconfig"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

type Client struct {
	HTTP clienthttp.Client
}

func New(cfg config.Config, logger *slog.Logger, obs observabilityx.Observability) (*Client, error) {
	obs = observabilityx.Normalize(obs, logger)

	httpClient, err := clienthttp.New(
		clienthttp.Config{
			BaseURL:   cfg.Server.URL,
			Timeout:   10 * time.Second,
			UserAgent: "orivis-agent/" + buildinfo.Version,
		},
		clienthttp.WithHooks(
			clientx.NewLoggingHook(logger),
			clientx.NewObservabilityHook(obs, clientx.WithHookMetricPrefix("orivis_agent_client")),
		),
		clienthttp.WithPolicies(
			clientx.NewTimeoutPolicy(10*time.Second),
			clientx.NewRetryPolicy(clientx.RetryPolicyConfig{
				MaxAttempts: 3,
				BaseDelay:   time.Second,
				MaxDelay:    5 * time.Second,
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create agent HTTP client: %w", err)
	}

	return &Client{HTTP: httpClient}, nil
}

func (c *Client) Close(context.Context) error {
	if c == nil || c.HTTP == nil {
		return nil
	}
	if err := c.HTTP.Close(); err != nil {
		return fmt.Errorf("close agent HTTP client: %w", err)
	}
	return nil
}

func (c *Client) Register(ctx context.Context, req protocol.AgentRegisterRequest) (protocol.AgentRegisterResponse, error) {
	var out protocol.AgentRegisterResponse
	resp, err := c.HTTP.Execute(
		ctx,
		c.HTTP.R().SetBody(req).SetResult(&out),
		http.MethodPost,
		"/api/agent/register",
	)
	if err != nil {
		return out, fmt.Errorf("execute register agent request: %w", err)
	}
	if resp.IsError() {
		return out, fmt.Errorf("register agent: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) Heartbeat(ctx context.Context, req protocol.AgentHeartbeatRequest) (protocol.AgentHeartbeatResponse, error) {
	var out protocol.AgentHeartbeatResponse
	resp, err := c.HTTP.Execute(
		ctx,
		c.HTTP.R().SetBody(req).SetResult(&out),
		http.MethodPost,
		"/api/agent/heartbeat",
	)
	if err != nil {
		return out, fmt.Errorf("execute heartbeat request: %w", err)
	}
	if resp.IsError() {
		return out, fmt.Errorf("heartbeat agent: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) Tasks(ctx context.Context, req protocol.AgentTasksRequest) (protocol.AgentTasksResponse, error) {
	var out protocol.AgentTasksResponse
	request := c.HTTP.R().
		SetQueryParam("agent_id", req.AgentID).
		SetResult(&out)
	if req.Token != "" {
		request.SetQueryParam("token", req.Token)
	}

	resp, err := c.HTTP.Execute(ctx, request, http.MethodGet, "/api/agent/tasks")
	if err != nil {
		return out, fmt.Errorf("execute tasks request: %w", err)
	}
	if resp.IsError() {
		return out, fmt.Errorf("pull agent tasks: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) SyncMonitors(ctx context.Context, req protocol.AgentMonitorSyncRequest) (protocol.AgentMonitorSyncResponse, error) {
	var out protocol.AgentMonitorSyncResponse
	resp, err := c.HTTP.Execute(
		ctx,
		c.HTTP.R().SetBody(req).SetResult(&out),
		http.MethodPost,
		"/api/agent/monitors",
	)
	if err != nil {
		return out, fmt.Errorf("execute monitor sync request: %w", err)
	}
	if resp.IsError() {
		return out, fmt.Errorf("sync agent monitors: server returned %s: %s", resp.Status(), resp.String())
	}
	return out, nil
}

func (c *Client) ReportResult(ctx context.Context, req protocol.AgentResultRequest) error {
	resp, err := c.HTTP.Execute(
		ctx,
		c.HTTP.R().SetBody(req),
		http.MethodPost,
		"/api/agent/results",
	)
	if err != nil {
		return fmt.Errorf("execute report result request: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("report agent result: server returned %s: %s", resp.Status(), resp.String())
	}
	return nil
}
