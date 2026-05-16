package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	redis "github.com/redis/go-redis/v9"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

const defaultRedisPort = "6379"

func (c *Checker) checkRedis(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	options, detail, err := redisProbeOptions(task.Target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": task.Target}, err
	}

	client := redis.NewClient(options)
	defer closeSilently(client)

	if _, err := client.Ping(ctx).Result(); err != nil {
		return model.StatusDown, detail, fmt.Errorf("execute Redis probe: %w", err)
	}
	return model.StatusUp, detail, nil
}

func redisProbeOptions(rawTarget string) (*redis.Options, map[string]any, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return nil, nil, errors.New("redis target is empty")
	}
	if strings.HasPrefix(strings.ToLower(target), "redis://") || strings.HasPrefix(strings.ToLower(target), "rediss://") {
		options, err := redis.ParseURL(target)
		if err != nil {
			return nil, nil, fmt.Errorf("parse Redis URL: %w", err)
		}
		return options, redisDetail(options), nil
	}
	address := redisAddress(target)
	options := &redis.Options{Addr: address}
	return options, redisDetail(options), nil
}

func redisAddress(target string) string {
	if _, _, err := net.SplitHostPort(target); err == nil {
		return target
	}
	return net.JoinHostPort(target, defaultRedisPort)
}

func redisDetail(options *redis.Options) map[string]any {
	return map[string]any{
		"target": options.Addr,
		"tls":    options.TLSConfig != nil,
	}
}
