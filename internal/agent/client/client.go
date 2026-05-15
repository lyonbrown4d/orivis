package client

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/clientx"
	clienthttp "github.com/arcgolabs/clientx/http"
	"github.com/arcgolabs/observabilityx"
	"github.com/lyonbrown4d/orivis/internal/agent/config"
	"github.com/lyonbrown4d/orivis/internal/shared/buildinfo"
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
		return nil, err
	}

	return &Client{HTTP: httpClient}, nil
}

func (c *Client) Close(context.Context) error {
	if c == nil || c.HTTP == nil {
		return nil
	}
	return c.HTTP.Close()
}
