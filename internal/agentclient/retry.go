package client

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"net/http"
	"time"

	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
	"resty.dev/v3"
)

type retryConfig struct {
	attempts    int
	baseDelay   time.Duration
	maxDelay    time.Duration
	jitterRatio float64
}

func newRetryConfig(cfg config.Config) retryConfig {
	return retryConfig{
		attempts:    cfg.Transport.RetryAttempts,
		baseDelay:   cfg.Transport.RetryBaseDelay,
		maxDelay:    cfg.Transport.RetryMaxDelay,
		jitterRatio: cfg.Transport.RetryJitterRatio,
	}
}

func (c *Client) execute(ctx context.Context, build func() *resty.Request, method, endpoint string) (*resty.Response, error) {
	attempts := max(1, c.retry.attempts)
	var resp *resty.Response
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err = c.HTTP.Execute(ctx, build(), method, endpoint)
		if !retryableAgentResponse(resp, err) || attempt == attempts {
			return resp, wrapRetryError(err)
		}
		delay := c.retry.delay(attempt)
		c.logRetry(ctx, method, endpoint, attempt, attempts, delay, resp, err)
		if waitErr := sleepRetry(ctx, delay); waitErr != nil {
			return resp, waitErr
		}
	}
	return resp, wrapRetryError(err)
}

func (c *Client) logRetry(ctx context.Context, method, endpoint string, attempt, maxAttempts int, delay time.Duration, resp *resty.Response, err error) {
	if c == nil || c.logger == nil {
		return
	}
	args := []any{
		"method", method,
		"endpoint", endpoint,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"next_delay", delay,
	}
	if resp != nil {
		args = append(args, "status_code", resp.StatusCode(), "status", resp.Status())
	}
	if err != nil {
		args = append(args, "error", err)
	}
	c.logger.DebugContext(ctx, "agent HTTP request retrying", args...)
}

func retryableAgentResponse(resp *resty.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	switch resp.StatusCode() {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (cfg retryConfig) delay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := min(time.Duration(float64(cfg.baseDelay)*math.Pow(2, float64(attempt-1))), cfg.maxDelay)
	return jitterDelay(delay, cfg.jitterRatio)
}

func jitterDelay(delay time.Duration, ratio float64) time.Duration {
	if ratio <= 0 || delay <= 0 {
		return delay
	}
	if ratio > 1 {
		ratio = 1
	}
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return delay
	}
	factor := float64(value.Int64()) / 1_000_000
	delta := float64(delay) * ratio
	jittered := float64(delay) - delta + (2 * delta * factor)
	if jittered <= 0 {
		return 0
	}
	if jittered > float64(delay)*2 {
		return delay * 2
	}
	return time.Duration(jittered)
}

func sleepRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return wrapRetryError(ctx.Err())
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return wrapRetryError(ctx.Err())
	case <-timer.C:
		return nil
	}
}

func wrapRetryError(err error) error {
	if err == nil {
		return nil
	}
	return wrapError(err, "execute agent HTTP request")
}
