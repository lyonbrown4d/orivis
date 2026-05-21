// Package cache provides pluggable server-side cache stores.
package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/arcgolabs/collectionx/bytex"
	"github.com/redis/go-redis/v9"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close(ctx context.Context) error
}

type memoryStore struct {
	mu     sync.RWMutex
	values map[string]memoryItem
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

type redisStore struct {
	client *redis.Client
	prefix string
}

func NewStore(cfg config.Config, logger *slog.Logger) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Cache.Driver)) {
	case "", "memory", "mem":
		return NewMemoryStore(), nil
	case "redis":
		return NewRedisStore(cfg, logger)
	default:
		return nil, newErrorf("unsupported cache driver %q", cfg.Cache.Driver)
	}
}

func NewMemoryStore() Store {
	return &memoryStore{values: make(map[string]memoryItem)}
}

func (s *memoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, wrapError(err, "get memory cache")
	}
	s.mu.RLock()
	item, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !item.expiresAt.IsZero() && time.Now().UTC().After(item.expiresAt) {
		if err := s.Delete(ctx, key); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	return bytex.WrapList(item.value).Snapshot(), true, nil
}

func (s *memoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return wrapError(err, "set memory cache")
	}
	item := memoryItem{value: bytex.WrapList(value).Snapshot()}
	if ttl > 0 {
		item.expiresAt = time.Now().UTC().Add(ttl)
	}
	s.mu.Lock()
	s.values[key] = item
	s.mu.Unlock()
	return nil
}

func (s *memoryStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return wrapError(err, "delete memory cache")
	}
	s.mu.Lock()
	delete(s.values, key)
	s.mu.Unlock()
	return nil
}

func (s *memoryStore) Close(context.Context) error {
	s.mu.Lock()
	clear(s.values)
	s.mu.Unlock()
	return nil
}

func NewRedisStore(cfg config.Config, logger *slog.Logger) (Store, error) {
	addr := strings.TrimSpace(cfg.Cache.Redis.Addr)
	if addr == "" {
		return nil, newError("redis cache addr is required")
	}
	options := &redis.Options{
		Addr:     addr,
		Password: cfg.Cache.Redis.Password,
		DB:       cfg.Cache.Redis.DB,
	}
	if cfg.Cache.Redis.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, wrapError(err, "connect redis cache")
	}
	if logger != nil {
		logger.Info("connected redis cache", "addr", addr)
	}
	return &redisStore{client: client, prefix: cachePrefix(cfg.Cache.Prefix)}, nil
}

func (s *redisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := s.client.Get(ctx, s.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, wrapError(err, "get redis cache")
	}
	return value, true, nil
}

func (s *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, s.key(key), value, ttl).Err(); err != nil {
		return wrapError(err, "set redis cache")
	}
	return nil
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, s.key(key)).Err(); err != nil {
		return wrapError(err, "delete redis cache")
	}
	return nil
}

func (s *redisStore) Close(context.Context) error {
	if err := s.client.Close(); err != nil {
		return wrapError(err, "close redis cache")
	}
	return nil
}

func (s *redisStore) key(key string) string {
	return s.prefix + key
}

func cachePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "orivis"
	}
	return prefix + ":"
}
