package cache

import (
	"context"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/bytex"
	"github.com/arcgolabs/kvx"
)

type kvxStore struct {
	client kvx.KV
	prefix string
}

func NewKVXStore(client kvx.KV, prefix string) Store {
	return &kvxStore{client: client, prefix: cachePrefix(prefix)}
}

func (s *kvxStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := s.client.Get(ctx, s.key(key))
	if err != nil {
		if kvx.IsNil(err) {
			return nil, false, nil
		}
		return nil, false, wrapError(err, "get kvx cache")
	}
	return bytex.WrapList(value).Snapshot(), true, nil
}

func (s *kvxStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, s.key(key), value, ttl); err != nil {
		return wrapError(err, "set kvx cache")
	}
	return nil
}

func (s *kvxStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Delete(ctx, s.key(key)); err != nil {
		return wrapError(err, "delete kvx cache")
	}
	return nil
}

func (s *kvxStore) Close(context.Context) error {
	if closer, ok := s.client.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			return wrapError(err, "close kvx cache")
		}
	}
	return nil
}

func (s *kvxStore) key(key string) string {
	return s.prefix + strings.TrimSpace(key)
}
